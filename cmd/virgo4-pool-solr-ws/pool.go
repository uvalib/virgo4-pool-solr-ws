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

	"golang.org/x/text/language"
	"golang.org/x/text/message"
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

type poolIdentity struct {
	Name string `json:"name,omitempty"`        // pool type
	Desc string `json:"description,omitempty"` // localized description
	URL  string `json:"public_url,omitempty"`  // public (service) url
}

type poolSolr struct {
	client               *http.Client
	url                  string
	availableFacets      map[string]solrRequestFacet
	virgoAvailableFacets []string
	reverseFacetMap      map[string]string
	scoreThresholdMedium float32
	scoreThresholdHigh   float32
}

type poolContext struct {
	randomSource *rand.Rand
	config       *poolConfig
	identity     poolIdentity
	version      poolVersion
	solr         poolSolr
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
	p.identity = poolIdentity{
		Name: p.config.poolType,
		Desc: p.config.poolDescription,
		URL:  p.config.poolServiceURL,
	}

	log.Printf("[POOL] identity.Name             = [%s]", p.identity.Name)
	log.Printf("[POOL] identity.Desc             = [%s]", p.identity.Desc)
	log.Printf("[POOL] identity.URL              = [%s]", p.identity.URL)
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

	solrTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(connTimeout) * time.Second,
		}).Dial,
		TLSHandshakeTimeout: time.Duration(connTimeout) * time.Second,
	}

	solrClient := &http.Client{
		Timeout:   time.Duration(readTimeout) * time.Second,
		Transport: solrTransport,
	}

	// facet setup

	type facetInfo struct {
		Facets []solrRequestFacet `json:"facets"`
	}

	var facetManifest facetInfo

	// read in all defined facets, and convert to a map
	if err := json.Unmarshal([]byte(p.config.solrFacetManifest), &facetManifest); err != nil {
		log.Printf("error parsing facets manifest json: %s", err.Error())
		os.Exit(1)
	}

	facetManifestMap := make(map[string]solrRequestFacet)

	for _, facet := range facetManifest.Facets {
		facetManifestMap[facet.Name] = solrRequestFacet{Type: facet.Type, Field: facet.Field, Sort: facet.Sort, Limit: facet.Limit}
	}

	// now select the facets from the manifest that are defined for this pool
	availableFacets := make(map[string]solrRequestFacet)
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
	type translationItem struct {
		Language    string `json:"language"`
		Translation string `json:"translation"`
	}

	type translationSet struct {
		ID           string            `json:"id"`
		Translations []translationItem `json:"translations"`
	}

	type translationInfo struct {
		SupportedLanguages []string         `json:"supported_languages"`
		TranslationMap     []translationSet `json:"translation_map"`
	}

	var translations translationInfo

	if err := json.Unmarshal([]byte(p.config.poolTranslations), &translations); err != nil {
		log.Printf("error parsing translations json: %s", err.Error())
		os.Exit(1)
	}

	// ensure each id has a translation for each supported language, logging all missing entries
	missing := false

	for _, language := range translations.SupportedLanguages {
		for _, set := range translations.TranslationMap {
			found := false
			for _, item := range set.Translations {
				if item.Language == language {
					found = true
					break
				}
			}

			if found == false {
				log.Printf("missing translation for language: [%s] and identifier: [%s]", language, set.ID)
				missing = true
			}
		}
	}

	if missing == true {
		log.Printf("exiting due to missing translations above")
		os.Exit(1)
	}

	// now initialize translations

	reverseFacetMap := make(map[string]string)

	for _, set := range translations.TranslationMap {
		for _, item := range set.Translations {
			lang := language.MustParse(item.Language)
			message.SetString(lang, set.ID, item.Translation)

			// also fill out reverse facet translation map
			if strings.HasPrefix(set.ID, "FACET_") {
				reverseFacetMap[item.Translation] = set.ID
			}
		}
	}

	p.solr.reverseFacetMap = reverseFacetMap
}

func (p *poolContext) init(cfg *poolConfig) {
	p.config = cfg
	p.randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

	p.initIdentity()
	p.initVersion()
	p.initSolr()
	p.initTranslations()
}
