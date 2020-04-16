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

type stringValidator struct {
	values  []string
	invalid bool
}

func (v *stringValidator) addValue(value string) {
	if value != "" {
		v.values = append(v.values, value)
	}
}

func (v *stringValidator) requireValue(value string, label string) {
	if value == "" {
		log.Printf("[VALIDATE] missing %s", label)
		v.invalid = true
		return
	}

	v.addValue(value)
}

func (v *stringValidator) Values() []string {
	return v.values
}

func (v *stringValidator) Invalid() bool {
	return v.invalid
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

func (p *poolContext) validateConfig() {
	// ensure the existence and validity of required variables/solr fields/translation ids

	invalid := false

	var solrFields stringValidator
	var messageIDs stringValidator
	var miscValues stringValidator

	miscValues.requireValue(p.config.Identity.Mode, "pool mode")

	miscValues.requireValue(p.config.Solr.Host, "solr host")
	miscValues.requireValue(p.config.Solr.Core, "solr core")
	miscValues.requireValue(p.config.Solr.Handler, "solr handler")
	miscValues.requireValue(p.config.Solr.Params.Qt, "solr param qt")
	miscValues.requireValue(p.config.Solr.Params.DefType, "solr param deftype")

	if len(p.config.Solr.Params.Fq) == 0 {
		log.Printf("[VALIDATE] solr param fq is empty")
		invalid = true
	}

	miscValues.requireValue(p.config.Solr.Grouping.SortOrder, "solr grouping sort order")

	solrFields.requireValue(p.config.Solr.Grouping.Field, "solr grouping field")
	solrFields.requireValue(p.config.Solr.ExactMatchTitleField, "solr exact match title field")
	solrFields.requireValue(p.config.Availability.Anon.Field, "anon availability field")
	solrFields.requireValue(p.config.Availability.Auth.Field, "auth availability field")

	messageIDs.requireValue(p.config.Identity.NameXID, "identity name xid")
	messageIDs.requireValue(p.config.Identity.DescXID, "identity description xid")
	messageIDs.requireValue(p.config.Solr.Grouping.SortXID, "solr grouping sort xid")

	if p.config.Identity.Mode == "image" {
		if p.config.Related.Image == nil {
			log.Printf("[VALIDATE] missing related image section")
			invalid = true
		} else {
			solrFields.requireValue(p.config.Related.Image.IDField, "iiif id field")
			solrFields.requireValue(p.config.Related.Image.IdentifierField, "iiif identifier field")
			solrFields.requireValue(p.config.Related.Image.IIIFManifestField, "iiif manifest field")
			solrFields.requireValue(p.config.Related.Image.IIIFImageField, "iiif image field")
		}
	}

	for i, val := range p.config.Identity.SortOptions {
		solrFields.requireValue(val.Field, fmt.Sprintf("sort option %d field", i))
		messageIDs.requireValue(val.XID, fmt.Sprintf("sort option %d xid", i))
	}

	for i, val := range p.config.Providers {
		messageIDs.requireValue(val.XID, fmt.Sprintf("provider %d xid", i))
	}

	for i, val := range p.config.Facets {
		messageIDs.requireValue(val.XID, fmt.Sprintf("facet %d xid", i))
		for j, depval := range val.DependentFacetXIDs {
			messageIDs.requireValue(depval, fmt.Sprintf("facet %d dependent xid %d", i, j))
		}
	}
	for i, field := range p.config.Fields {
		messageIDs.addValue(field.XID)

		miscValues.requireValue(field.Properties.Name, fmt.Sprintf("field %d properties name", i))

		switch field.Format {
		case "access_url":
			if field.AccessURL == nil {
				log.Printf("[VALIDATE] missing field %d %s section", i, field.Format)
				invalid = true
				continue
			}

			solrFields.requireValue(field.AccessURL.URLField, fmt.Sprintf("field %d %s url field", i, field.Format))
			solrFields.requireValue(field.AccessURL.LabelField, fmt.Sprintf("field %d %s label field", i, field.Format))
			solrFields.requireValue(field.AccessURL.ProviderField, fmt.Sprintf("field %d %s provider field", i, field.Format))
			messageIDs.requireValue(field.AccessURL.DefaultItemXID, fmt.Sprintf("field %d %s default item xid", i, field.Format))

		case "authentication_prompt":

		case "availability":

		case "cover_image_url":
			if field.CoverImageURL == nil {
				log.Printf("[VALIDATE] missing field %d %s section", i, field.Format)
				invalid = true
				continue
			}

			solrFields.requireValue(field.CoverImageURL.ThumbnailField, fmt.Sprintf("field %d %s thumbnail url field", i, field.Format))
			solrFields.requireValue(field.CoverImageURL.IDField, fmt.Sprintf("field %d %s id field", i, field.Format))
			solrFields.requireValue(field.CoverImageURL.TitleField, fmt.Sprintf("field %d %s title field", i, field.Format))
			solrFields.requireValue(field.CoverImageURL.PoolField, fmt.Sprintf("field %d %s pool field", i, field.Format))

			solrFields.addValue(field.CoverImageURL.ISBNField)
			solrFields.addValue(field.CoverImageURL.OCLCField)
			solrFields.addValue(field.CoverImageURL.LCCNField)
			solrFields.addValue(field.CoverImageURL.UPCField)

		case "iiif_base_url":
			if field.IIIFBaseURL == nil {
				log.Printf("[VALIDATE] missing field %d %s section", i, field.Format)
				invalid = true
				continue
			}

			solrFields.requireValue(field.IIIFBaseURL.IdentifierField, fmt.Sprintf("field %d %s identifier field", i, field.Format))

		case "sirsi_url":
			if field.SirsiURL == nil {
				log.Printf("[VALIDATE] missing field %d %s section", i, field.Format)
				invalid = true
				continue
			}

			solrFields.requireValue(field.SirsiURL.IDField, fmt.Sprintf("field %d %s id field", i, field.Format))
			miscValues.requireValue(field.SirsiURL.IDPrefix, fmt.Sprintf("field %d %s id prefix", i, field.Format))

		default:
			if field.Format != "" {
				log.Printf("[VALIDATE] field %d: unhandled format: [%s]", i, field.Format)
				invalid = true
				continue
			}

			solrFields.requireValue(field.Field, fmt.Sprintf("field %d field", i))
		}
	}

	// validate solr fields can actually be found in a solr document

	doc := solrDocument{}

	for _, tag := range solrFields.Values() {
		if val := doc.getFieldByTag(tag); val == nil {
			log.Printf("[VALIDATE] tag not found in Solr document struct tags: [%s]", tag)
			invalid = true
		}
	}

	// validate xids can actually be translated

	langs := []string{}
	tags := p.translations.bundle.LanguageTags()

	for _, tag := range tags {
		lang := tag.String()
		langs = append(langs, lang)
		localizer := i18n.NewLocalizer(p.translations.bundle, lang)
		for _, id := range messageIDs.Values() {
			if _, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: id}); err != nil {
				log.Printf("[VALIDATE] [%s] missing translation for message ID: [%s]  (%s)", lang, id, err.Error())
				invalid = true
			}
		}
	}

	// check if anything went wrong anywhere

	if invalid || solrFields.Invalid() || messageIDs.Invalid() || miscValues.Invalid() {
		log.Printf("[VALIDATE] exiting due to missing/incorrect field value(s) above")
		os.Exit(1)
	}

	log.Printf("[POOL] supported languages       = [%s]", strings.Join(langs, ", "))
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

	p.validateConfig()

	return &p
}
