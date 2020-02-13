package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const defaultScoreThresholdMedium = 100.0
const defaultScoreThresholdHigh = 200.0

// git commit used for this build; supplied at compile time
var gitCommit string

type poolVersion struct {
	BuildVersion string `json:"build,omitempty"`
	GoVersion    string `json:"go_version,omitempty"`
	GitCommit    string `json:"git_commit,omitempty"`
}

type poolSolr struct {
	client               *http.Client
	url                  string
	availableFacets      map[string]poolFacetDefinition
	virgoAvailableFacets []string
	scoreThresholdMedium float32
	scoreThresholdHigh   float32
}

type poolTranslations struct {
	messageIDs []string
	bundle     *i18n.Bundle
}

type poolContext struct {
	randomSource *rand.Rand
	config       *poolConfig
	translations poolTranslations
	identity     VirgoPoolIdentity
	version      poolVersion
	solr         poolSolr
	attributes   map[string]VirgoPoolAttribute
}

// NOTE: this struct is a superset of solrRequestFacet, and is separate so that extra fields are not sent to Solr
type poolFacetDefinition struct {
	Name          string   `json:"name"`           // the internal name of this facet
	Field         string   `json:"field"`          // the default Solr field to use
	FieldAuth     string   `json:"field_auth"`     // if defined, the Solr field to use if the client is authenticated
	ExposedValues []string `json:"exposed_values"` // if defined, only values in this list will be exposed to the client
	Type          string   `json:"type"`           // the Solr type of this facet
	Sort          string   `json:"sort"`           // the Solr sorting to use for this facet
	Offset        int      `json:"offset"`         // the Solr offset to use for this facet
	Limit         int      `json:"limit"`          // the Solr limit to apply to this facet
}

func buildVersion() string {
	files, _ := filepath.Glob("buildtag.*")
	if len(files) == 1 {
		return strings.Replace(files[0], "buildtag.", "", 1)
	}

	return "unknown"
}

func timeoutWithMinimum(str string, min int) int {
	val, err := strconv.Atoi(str)

	// fallback for invalid or nonsensical timeout values
	if err != nil || val < min {
		val = min
	}

	return val
}

func getScoreThresholds(confMed, confHigh string) (medium, high float32) {
	var err error
	var m, h float64

	medium = defaultScoreThresholdMedium
	high = defaultScoreThresholdHigh

	if m, err = strconv.ParseFloat(confMed, 32); err != nil {
		return
	}

	if h, err = strconv.ParseFloat(confHigh, 32); err != nil {
		return
	}

	if m < 0 || h < 0 || m >= h {
		return
	}

	medium = float32(m)
	high = float32(h)

	return
}

func (p *poolContext) initIdentity() {
	p.identity = VirgoPoolIdentity{
		Name:        p.config.poolName,
		Description: p.config.poolDescription,
	}

	// read in all defined attributes, and convert to a map
	if err := json.Unmarshal([]byte(p.config.poolAttributes), &p.identity.Attributes); err != nil {
		log.Printf("error parsing pool attributes json: %s", err.Error())
		os.Exit(1)
	}

	p.attributes = make(map[string]VirgoPoolAttribute)

	for _, attribute := range p.identity.Attributes {
		p.attributes[attribute.Name] = attribute
	}

	log.Printf("[POOL] identity.Name             = [%s]", p.identity.Name)
	log.Printf("[POOL] identity.Description      = [%s]", p.identity.Description)
	log.Printf("[POOL] identity.Attributes       = [%v]", p.identity.Attributes)
	log.Printf("[POOL] attributes                = [%v]", p.attributes)
}

func (p *poolContext) initVersion() {
	p.version = poolVersion{
		BuildVersion: buildVersion(),
		GoVersion:    fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
		GitCommit:    gitCommit,
	}

	log.Printf("[POOL] version.BuildVersion      = [%s]", p.version.BuildVersion)
	log.Printf("[POOL] version.GoVersion         = [%s]", p.version.GoVersion)
	log.Printf("[POOL] version.GitCommit         = [%s]", p.version.GitCommit)
}

func (p *poolContext) initSolr() {
	// client setup

	connTimeout := timeoutWithMinimum(p.config.solrConnTimeout, 5)
	readTimeout := timeoutWithMinimum(p.config.solrReadTimeout, 5)

	solrClient := &http.Client{
		Timeout: time.Duration(readTimeout) * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(connTimeout) * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
			MaxIdleConns:        100, // we are hitting one solr host, so
			MaxIdleConnsPerHost: 100, // these two values can be the same
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// facet setup

	type facetInfo struct {
		Facets []poolFacetDefinition `json:"facets"`
	}

	var facetManifest facetInfo

	// read in all defined facets, and convert to a map
	if err := json.Unmarshal([]byte(p.config.solrFacetManifest), &facetManifest); err != nil {
		log.Printf("error parsing facets manifest json: %s", err.Error())
		os.Exit(1)
	}

	facetManifestMap := make(map[string]poolFacetDefinition)

	for _, facet := range facetManifest.Facets {
		facetManifestMap[facet.Name] = facet
	}

	// now select the facets from the manifest that are defined for this pool
	availableFacets := make(map[string]poolFacetDefinition)
	var virgoAvailableFacets []string

	for _, f := range strings.Split(p.config.poolFacets, ",") {
		facet, ok := facetManifestMap[f]

		if ok == false {
			continue
		}

		// add this facet
		virgoAvailableFacets = append(virgoAvailableFacets, f)
		availableFacets[f] = facet
	}

	// set score thresholds

	medium, high := getScoreThresholds(p.config.scoreThresholdMedium, p.config.scoreThresholdHigh)

	p.solr = poolSolr{
		url:                  fmt.Sprintf("%s/%s/%s", p.config.solrHost, p.config.solrCore, p.config.solrHandler),
		client:               solrClient,
		availableFacets:      availableFacets,
		virgoAvailableFacets: virgoAvailableFacets,
		scoreThresholdMedium: medium,
		scoreThresholdHigh:   high,
	}

	log.Printf("[POOL] solr.url                  = [%s]", p.solr.url)
	log.Printf("[POOL] solr.virgoAvailableFacets = [%s]", strings.Join(p.solr.virgoAvailableFacets, "; "))
	log.Printf("[POOL] solr.scoreThresholdMedium = [%0.1f]", p.solr.scoreThresholdMedium)
	log.Printf("[POOL] solr.scoreThresholdHigh   = [%0.1f]", p.solr.scoreThresholdHigh)
}

func (p *poolContext) initTranslations() {
	defaultLang := language.English

	bundle := i18n.NewBundle(defaultLang)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	toml, _ := filepath.Glob("i18n/*.toml")
	for _, f := range toml {
		bundle.MustLoadMessageFile(f)
	}

	// thorough check: for each messageID in english, ensure it exists in all other languages.
	// ...but there does not appear to be a way to get all loaded messageIDs for a given language, so:

	// hardcoded check: for each hardcoded messageID (i.e. all known), ensure it exists in all languages.
	// NOTE: this list must be kept up to date.

	messageIDs := []string{
		"PoolArchivalName",
		"PoolArchivalDescription",
		"PoolCatalogBroadName",
		"PoolCatalogBroadDescription",
		"PoolCatalogName",
		"PoolCatalogDescription",
		"PoolMusicRecordingsName",
		"PoolMusicRecordingsDescription",
		"PoolMusicalScoresName",
		"PoolMusicalScoresDescription",
		"PoolRareBooksName",
		"PoolRareBooksDescription",
		"PoolSerialsName",
		"PoolSerialsDescription",
		"PoolSoundRecordingsName",
		"PoolSoundRecordingsDescription",
		"PoolThesisName",
		"PoolThesisDescription",
		"PoolVideoName",
		"PoolVideoDescription",
		"FacetAuthor",
		"FacetAvailability",
		"FacetCallNumberBroad",
		"FacetCallNumberNarrow",
		"FacetComposer",
		"FacetCompositionEra",
		"FacetFormat",
		"FacetGenre",
		"FacetInstrument",
		"FacetLanguage",
		"FacetLibrary",
		"FacetRegion",
		"FacetSeries",
		"FacetSubject",
		"FacetVideoFormat",
		"FieldIdentifier",
		"FieldTitle",
		"FieldSubtitle",
		"FieldAuthor",
		"FieldDirector",
		"FieldSubject",
		"FieldLanguage",
		"FieldFormat",
		"FieldLibrary",
		"FieldLocation",
		"FieldCallNumber",
		"FieldCallNumberBroad",
		"FieldCallNumberNarrow",
		"FieldAvailability",
		"FieldSeries",
		"FieldGenre",
		"FieldPublicationDate",
		"FieldPublished",
		"FieldAccessURL",
		"FieldDetailsURL",
	}

	langs := []string{}

	tags := bundle.LanguageTags()
	missingTranslations := false

	for _, tag := range tags {
		lang := tag.String()

		log.Printf("[LANG] [%s] verifying translations...", lang)

		langs = append(langs, lang)

		localizer := i18n.NewLocalizer(bundle, lang)

		for _, id := range messageIDs {
			if _, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: id}); err != nil {
				log.Printf("[LANG] [%s] missing translation for message ID: [%s]  (%s)", lang, id, err.Error())
				missingTranslations = true
			}
		}
	}

	if missingTranslations == true {
		log.Printf("[LANG] exiting due to missing translation(s) above")
		os.Exit(1)
	}

	p.translations = poolTranslations{
		messageIDs: messageIDs,
		bundle:     bundle,
	}

	log.Printf("[POOL] supported languages       = [%s]", strings.Join(langs, ", "))
}

func (p *poolContext) sanityChecks() {
	// ensure certain pool-specific field mappings exist by extracting values from a fake solr document

	doc := solrDocument{
		Author:            []string{"test"},
		Director:          []string{"test"},
		WorkTitle2KeySort: "test",
		WorkTitle3KeySort: "test",
	}

	if group := doc.getStringValueByTag(p.config.solrGroupField); group == "" {
		log.Printf("[SANITY] grouping field not found in struct tags: [%s]", p.config.solrGroupField)
		os.Exit(1)
	}

	if author := doc.getStringSliceValueByTag(p.config.solrAuthorField); len(author) == 0 {
		log.Printf("[SANITY] author field not found in struct tags: [%s]", p.config.solrAuthorField)
		os.Exit(1)
	}
}

func initializePool(cfg *poolConfig) *poolContext {
	p := poolContext{}

	p.config = cfg
	p.randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

	p.initTranslations()
	p.initIdentity()
	p.initVersion()
	p.initSolr()

	p.sanityChecks()

	return &p
}
