package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

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
	scoreThresholdMedium float32
	scoreThresholdHigh   float32
}

type poolTranslations struct {
	bundle *i18n.Bundle
}

type poolMaps struct {
	sortFields      map[string]string
	attributes      map[string]VirgoPoolAttribute
	availableFacets map[string]poolConfigFacet
}

type poolContext struct {
	randomSource *rand.Rand
	config       *poolConfig
	translations poolTranslations
	identity     VirgoPoolIdentity
	providers    VirgoPoolProviders
	version      poolVersion
	solr         poolSolr
	maps         poolMaps
}

func (p *poolContext) initIdentity() {
	p.identity = VirgoPoolIdentity{
		Name:        p.config.Identity.NameXID,
		Description: p.config.Identity.DescXID,
		Mode:        p.config.Identity.Mode,
		Attributes:  p.config.Identity.Attributes,
	}

	// create sort field map
	p.maps.sortFields = make(map[string]string)
	for _, val := range p.config.Identity.SortOptions {
		p.identity.SortOptions = append(p.identity.SortOptions, VirgoSortOption{ID: val.XID})
		p.maps.sortFields[val.XID] = val.Field
	}

	// create attribute map
	p.maps.attributes = make(map[string]VirgoPoolAttribute)
	for _, attribute := range p.identity.Attributes {
		p.maps.attributes[attribute.Name] = attribute
	}

	log.Printf("[POOL] identity.Name             = [%s]", p.identity.Name)
	log.Printf("[POOL] identity.Description      = [%s]", p.identity.Description)
	log.Printf("[POOL] identity.Mode             = [%s]", p.identity.Mode)
}

func (p *poolContext) initProviders() {
	for _, val := range p.config.Providers {
		provider := VirgoProvider{
			Provider:    val.Name,
			Label:       val.XID,
			LogoURL:     val.Logo,
			HomepageURL: val.URL,
		}

		p.providers.Providers = append(p.providers.Providers, provider)
	}
}

func (p *poolContext) initVersion() {
	buildVersion := "unknown"
	files, _ := filepath.Glob("buildtag.*")
	if len(files) == 1 {
		buildVersion = strings.Replace(files[0], "buildtag.", "", 1)
	}

	p.version = poolVersion{
		BuildVersion: buildVersion,
		GoVersion:    fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
		GitCommit:    gitCommit,
	}

	log.Printf("[POOL] version.BuildVersion      = [%s]", p.version.BuildVersion)
	log.Printf("[POOL] version.GoVersion         = [%s]", p.version.GoVersion)
	log.Printf("[POOL] version.GitCommit         = [%s]", p.version.GitCommit)
}

func (p *poolContext) initSolr() {
	// client setup

	connTimeout := timeoutWithMinimum(p.config.Solr.ConnTimeout, 5)
	readTimeout := timeoutWithMinimum(p.config.Solr.ReadTimeout, 5)

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

	// create facet map
	p.maps.availableFacets = make(map[string]poolConfigFacet)

	p.config.Availability.ExposedValues = []string{}
	p.config.Availability.ExposedValues = append(p.config.Availability.ExposedValues, p.config.Availability.Values.OnShelf...)
	p.config.Availability.ExposedValues = append(p.config.Availability.ExposedValues, p.config.Availability.Values.Online...)
	p.config.Availability.ExposedValues = append(p.config.Availability.ExposedValues, p.config.Availability.Values.Other...)

	for i, _ := range p.config.Facets {
		f := &p.config.Facets[i]

		// configure availability facet while we're here
		if f.IsAvailability == true {
			f.Solr.Field = p.config.Availability.Anon.Facet
			f.Solr.FieldAuth = p.config.Availability.Auth.Facet
			f.ExposedValues = p.config.Availability.ExposedValues
		}

		p.maps.availableFacets[f.XID] = *f
	}

	p.solr = poolSolr{
		url:                  fmt.Sprintf("%s/%s/%s", p.config.Solr.Host, p.config.Solr.Core, p.config.Solr.Handler),
		client:               solrClient,
		scoreThresholdMedium: p.config.Solr.ScoreThresholdMedium,
		scoreThresholdHigh:   p.config.Solr.ScoreThresholdHigh,
	}

	log.Printf("[POOL] solr.url                  = [%s]", p.solr.url)
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

	p.translations = poolTranslations{
		bundle: bundle,
	}
}

func (p *poolContext) validateTranslations() {
	// collect all translation IDs (XIDs) provided in pool config

	messageIDs := []string{}

	messageIDs = append(messageIDs, p.config.Identity.NameXID)
	messageIDs = append(messageIDs, p.config.Identity.DescXID)

	for _, val := range p.config.Identity.SortOptions {
		messageIDs = append(messageIDs, val.XID)
	}

	for _, val := range p.config.Providers {
		messageIDs = append(messageIDs, val.XID)
	}

	for _, val := range p.config.Fields {
		messageIDs = append(messageIDs, val.XID)
	}

	for _, val := range p.config.Facets {
		messageIDs = append(messageIDs, val.XID)
		messageIDs = append(messageIDs, val.DependentFacetXIDs...)
	}

	messageIDs = nonemptyValues(messageIDs)

	// ensure each XID has a translation for all loaded languages

	langs := []string{}

	tags := p.translations.bundle.LanguageTags()
	missingTranslations := false

	for _, tag := range tags {
		lang := tag.String()

		log.Printf("[LANG] [%s] validating translations...", lang)

		langs = append(langs, lang)

		localizer := i18n.NewLocalizer(p.translations.bundle, lang)

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

	log.Printf("[POOL] supported languages       = [%s]", strings.Join(langs, ", "))
}

func (p *poolContext) validateFields() {
	// ensure certain pool-specific field mappings exist by extracting values from a fake solr document

	fields := []string{}

	fields = append(fields, p.config.Solr.Grouping.Field)
	fields = append(fields, p.config.Availability.Anon.Field)
	fields = append(fields, p.config.Availability.Auth.Field)
	fields = append(fields, p.config.Related.Image.IDField)
	fields = append(fields, p.config.Related.Image.IdentifierField)
	fields = append(fields, p.config.Related.Image.IIIFManifestField)
	fields = append(fields, p.config.Related.Image.IIIFImageField)

	for _, val := range p.config.Identity.SortOptions {
		fields = append(fields, val.Field)
	}

	for _, val := range p.config.Fields {
		fields = append(fields, val.Field)
		fields = append(fields, val.URLField)
		fields = append(fields, val.LabelField)
		fields = append(fields, val.ProviderField)
	}

	fields = nonemptyValues(fields)

	doc := solrDocument{}

	log.Printf("[FIELDS] validating solr fields...")

	missingFields := false

	for _, tag := range fields {
		if val := doc.getFieldByTag(tag); val == nil {
			log.Printf("[FIELDS] field not found in struct tags: [%s]", tag)
			missingFields = true
		}
	}

	if missingFields == true {
		log.Printf("[FIELDS] exiting due to missing field(s) above")
		os.Exit(1)
	}
}

func initializePool(cfg *poolConfig) *poolContext {
	p := poolContext{}

	p.config = cfg
	p.randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

	p.initTranslations()
	p.initIdentity()
	p.initProviders()
	p.initVersion()
	p.initSolr()

	p.validateTranslations()
	p.validateFields()

	return &p
}
