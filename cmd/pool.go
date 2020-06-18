package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/uvalib/virgo4-api/v4api"
	"golang.org/x/text/language"
)

// git commit used for this build; supplied at compile time
var gitCommit string

type poolVersion struct {
	BuildVersion string `json:"build,omitempty"`
	GoVersion    string `json:"go_version,omitempty"`
	GitCommit    string `json:"git_commit,omitempty"`
}

type poolSolrContext struct {
	client *http.Client
	url    string
}

type poolSolr struct {
	service              poolSolrContext
	healthcheck          poolSolrContext
	scoreThresholdMedium float32
	scoreThresholdHigh   float32
}

type poolPdf struct {
	client *http.Client
}

type poolTranslations struct {
	bundle *i18n.Bundle
}

type poolMaps struct {
	sortFields      map[string]poolConfigSort
	attributes      map[string]v4api.PoolAttribute
	availableFacets map[string]poolConfigFacet
	relatorTerms    map[string]string
	relatorCodes    map[string]string
}

type poolContext struct {
	randomSource *rand.Rand
	config       *poolConfig
	translations poolTranslations
	identity     v4api.PoolIdentity
	providers    v4api.PoolProviders
	version      poolVersion
	solr         poolSolr
	pdf          poolPdf
	maps         poolMaps
}

type stringValidator struct {
	values  []string
	invalid bool
	prefix  string
	postfix string
}

func (v *stringValidator) addValue(value string) {
	if value != "" {
		v.values = append(v.values, value)
	}
}

func (v *stringValidator) setPrefix(prefix string) {
	v.prefix = prefix
}

func (v *stringValidator) setPostfix(postfix string) {
	v.postfix = postfix
}

func (v *stringValidator) requireValue(value string, label string) {
	if value == "" {
		log.Printf("[VALIDATE] %smissing %s%s", v.prefix, label, v.postfix)
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
	p.identity = v4api.PoolIdentity{
		Name:        p.config.Local.Identity.NameXID,
		Description: p.config.Local.Identity.DescXID,
		Mode:        p.config.Local.Identity.Mode,
	}

	// populate supported attributes

	for _, attribute := range p.config.Global.Attributes {
		supported := false
		if sliceContainsString(p.config.Local.Identity.Attributes, attribute) {
			supported = true
		}

		p.identity.Attributes = append(p.identity.Attributes, v4api.PoolAttribute{Name: attribute, Supported: supported})
	}

	// create sort field map
	p.maps.sortFields = make(map[string]poolConfigSort)
	for i := range p.config.Mappings.Definitions.Sorts {
		s := &p.config.Mappings.Definitions.Sorts[i]

		// FIXME: needs improvement; should not assume relevance sort XID
		if s.XID == "SortRelevance" {
			s.RecordXID = p.config.Local.Solr.RelevanceIntraGroupSort.XID
			s.RecordOrder = p.config.Local.Solr.RelevanceIntraGroupSort.Order
		}

		p.maps.sortFields[s.XID] = *s
		p.identity.SortOptions = append(p.identity.SortOptions, v4api.SortOption{ID: s.XID})
	}

	// create attribute map
	p.maps.attributes = make(map[string]v4api.PoolAttribute)
	for _, attribute := range p.identity.Attributes {
		p.maps.attributes[attribute.Name] = attribute
	}

	log.Printf("[POOL] identity.Name             = [%s]", p.identity.Name)
	log.Printf("[POOL] identity.Description      = [%s]", p.identity.Description)
	log.Printf("[POOL] identity.Mode             = [%s]", p.identity.Mode)
}

func (p *poolContext) initProviders() {
	for _, val := range p.config.Global.Providers {
		provider := v4api.Provider{
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

func httpClientWithTimeouts(conn, read string) *http.Client {
	connTimeout := integerWithMinimum(conn, 1)
	readTimeout := integerWithMinimum(read, 1)

	client := &http.Client{
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

	return client
}

func (p *poolContext) initSolr() {
	// service client setup

	serviceCtx := poolSolrContext{
		url:    fmt.Sprintf("%s/%s/%s", p.config.Local.Solr.Host, p.config.Local.Solr.Core, p.config.Local.Solr.Clients.Service.Endpoint),
		client: httpClientWithTimeouts(p.config.Local.Solr.Clients.Service.ConnTimeout, p.config.Local.Solr.Clients.Service.ReadTimeout),
	}

	healthCtx := poolSolrContext{
		url:    fmt.Sprintf("%s/%s/%s", p.config.Local.Solr.Host, p.config.Local.Solr.Core, p.config.Local.Solr.Clients.HealthCheck.Endpoint),
		client: httpClientWithTimeouts(p.config.Local.Solr.Clients.HealthCheck.ConnTimeout, p.config.Local.Solr.Clients.HealthCheck.ReadTimeout),
	}

	// create facet map
	p.maps.availableFacets = make(map[string]poolConfigFacet)

	p.config.Global.Availability.ExposedValues = []string{}
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.OnShelf...)
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.Online...)
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.Other...)

	for i := range p.config.Mappings.Definitions.Facets {
		f := &p.config.Mappings.Definitions.Facets[i]

		f.Index = i

		// configure availability facet while we're here
		if f.IsAvailability == true {
			f.Solr.Field = p.config.Global.Availability.Anon.Facet
			f.Solr.FieldAuth = p.config.Global.Availability.Auth.Facet
			f.ExposedValues = p.config.Global.Availability.ExposedValues
		}

		p.maps.availableFacets[f.XID] = *f
	}

	p.solr = poolSolr{
		service:              serviceCtx,
		healthcheck:          healthCtx,
		scoreThresholdMedium: p.config.Local.Solr.ScoreThresholdMedium,
		scoreThresholdHigh:   p.config.Local.Solr.ScoreThresholdHigh,
	}

	log.Printf("[POOL] solr.service.url          = [%s]", p.solr.service.url)
	log.Printf("[POOL] solr.healthcheck.url      = [%s]", p.solr.healthcheck.url)
	log.Printf("[POOL] solr.scoreThresholdMedium = [%0.1f]", p.solr.scoreThresholdMedium)
	log.Printf("[POOL] solr.scoreThresholdHigh   = [%0.1f]", p.solr.scoreThresholdHigh)
}

func (p *poolContext) initPdf() {
	// client setup

	p.pdf = poolPdf{
		client: httpClientWithTimeouts(p.config.Global.Service.Pdf.ConnTimeout, p.config.Global.Service.Pdf.ReadTimeout),
	}
}

func (p *poolContext) initRIS() {
	invalid := false

	for i := range p.config.Global.RISTypes {
		ristype := &p.config.Global.RISTypes[i]

		var err error

		if ristype.Type == "" {
			log.Printf("[INIT] empty type in RIS type entry %d", i)
			invalid = true
		}

		if ristype.Pattern == "" {
			log.Printf("[INIT] empty pattern in RIS type entry %d (type: %s)", i, ristype.Type)
			invalid = true
			continue
		}

		if ristype.re, err = regexp.Compile(ristype.Pattern); err != nil {
			log.Printf("[INIT] pattern compilation error in RIS type entry %d (type: %s): %s", i, ristype.Type, err.Error())
			invalid = true
			continue
		}
	}

	for i := range p.config.Global.Service.Publishers {
		publisher := &p.config.Global.Service.Publishers[i]

		var err error

		if publisher.Pattern == "" {
			log.Printf("[INIT] empty pattern in publisher entry %d (id: %s)", i, publisher.ID)
			invalid = true
			continue
		}

		if publisher.re, err = regexp.Compile(publisher.Pattern); err != nil {
			log.Printf("[INIT] pattern compilation error in publisher entry %d (id: %s): %s", i, publisher.ID, err.Error())
			invalid = true
			continue
		}
	}

	if invalid == true {
		log.Printf("[INIT] exiting due to error(s) above")
		os.Exit(1)
	}
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

	for _, attribute := range p.config.Local.Identity.Attributes {
		if sliceContainsString(p.config.Global.Attributes, attribute) == false {
			log.Printf("[VALIDATE] attribute [%s] not found in global attribute list", attribute)
			invalid = true
		}
	}

	miscValues.requireValue(p.config.Global.Service.DefaultSort.XID, "default sort xid")
	miscValues.requireValue(p.config.Global.Service.DefaultSort.Order, "default sort order")

	if p.config.Global.Service.DefaultSort.XID != "" && p.maps.sortFields[p.config.Global.Service.DefaultSort.XID].XID == "" {
		log.Printf("[VALIDATE] default sort xid not found in sort options list")
		invalid = true
	}

	if isValidSortOrder(p.config.Global.Service.DefaultSort.Order) == false {
		log.Printf("[VALIDATE] default sort order not valid")
		invalid = true
	}

	miscValues.requireValue(p.config.Local.Identity.Mode, "pool mode")

	miscValues.requireValue(p.config.Local.Solr.Host, "solr host")
	miscValues.requireValue(p.config.Local.Solr.Core, "solr core")
	miscValues.requireValue(p.config.Local.Solr.Clients.Service.Endpoint, "solr search endpoint")
	miscValues.requireValue(p.config.Local.Solr.Clients.HealthCheck.Endpoint, "solr health check endpoint")
	miscValues.requireValue(p.config.Local.Solr.Params.Qt, "solr param qt")
	miscValues.requireValue(p.config.Local.Solr.Params.DefType, "solr param deftype")

	if len(p.config.Local.Solr.Params.Fq) == 0 {
		log.Printf("[VALIDATE] solr param fq is empty")
		invalid = true
	}

	solrFields.requireValue(p.config.Local.Solr.GroupField, "solr grouping field")
	solrFields.requireValue(p.config.Local.Solr.ExactMatchTitleField, "solr exact match title field")
	solrFields.requireValue(p.config.Global.Availability.Anon.Field, "anon availability field")
	solrFields.requireValue(p.config.Global.Availability.Auth.Field, "auth availability field")

	messageIDs.requireValue(p.config.Local.Identity.NameXID, "identity name xid")
	messageIDs.requireValue(p.config.Local.Identity.DescXID, "identity description xid")

	if len(p.config.Global.Service.Relators.AuthorFields) == 0 {
		log.Printf("[VALIDATE] missing relator author field(s)")
		invalid = true
	}

	for _, field := range p.config.Global.Service.Relators.AuthorFields {
		solrFields.requireValue(field, "relator author field")
	}

	if p.config.Local.Identity.Mode == "image" {
		if p.config.Local.Related == nil {
			log.Printf("[VALIDATE] missing related section")
			invalid = true
		} else if p.config.Local.Related.Image == nil {
			log.Printf("[VALIDATE] missing related image section")
			invalid = true
		} else {
			solrFields.requireValue(p.config.Local.Related.Image.IDField, "iiif id field")
			solrFields.requireValue(p.config.Local.Related.Image.IIIFManifestField, "iiif manifest field")
			solrFields.requireValue(p.config.Local.Related.Image.IIIFImageField, "iiif image field")
		}
	}

	for i, val := range p.config.Mappings.Definitions.Sorts {
		messageIDs.requireValue(val.XID, fmt.Sprintf("sort option %d xid", i))
		solrFields.requireValue(val.Field, fmt.Sprintf("sort option %d group field", i))
		messageIDs.addValue(val.RecordXID)

		if val.RecordXID != "" && p.maps.sortFields[val.RecordXID].XID == "" {
			log.Printf("[VALIDATE] sort option %d record sort xid not found in sort options list", i)
			invalid = true
		}

		if val.Order != "" && isValidSortOrder(val.Order) == false {
			log.Printf("[VALIDATE] sort option %d sort order invalid", i)
			invalid = true
		}

		if val.RecordOrder != "" && isValidSortOrder(val.RecordOrder) == false {
			log.Printf("[VALIDATE] sort option %d record sort order invalid", i)
			invalid = true
		}
	}

	for i, val := range p.config.Global.Providers {
		messageIDs.requireValue(val.XID, fmt.Sprintf("provider %d xid", i))
	}

	for i, val := range p.config.Mappings.Definitions.Facets {
		messageIDs.requireValue(val.XID, fmt.Sprintf("facet %d xid", i))
		for j, depval := range val.DependentFacetXIDs {
			messageIDs.requireValue(depval, fmt.Sprintf("facet %d dependent xid %d", i, j))
		}
	}

	for i, val := range p.config.Global.Service.Publishers {
		solrFields.requireValue(val.Field, fmt.Sprintf("publisher %d solr field", i))
	}

	for i, field := range p.config.Mappings.Definitions.Fields {
		prefix := fmt.Sprintf("field index %d: ", i)
		postfix := fmt.Sprintf(` -- {Name:"%s" XID:"%s" Field:"%s"}`, field.Name, field.XID, field.Field)

		solrFields.setPrefix(prefix)
		messageIDs.setPrefix(prefix)
		miscValues.setPrefix(prefix)

		solrFields.setPostfix(postfix)
		messageIDs.setPostfix(postfix)
		miscValues.setPostfix(postfix)

		// start validating

		messageIDs.addValue(field.XID)

		miscValues.requireValue(field.Name, "name")

		if field.DigitalContentOnly == true {
			solrFields.requireValue(p.config.Global.Service.DigitalContent.FeatureField, "digital content feature field")
		}

		if field.Custom == true {
			if field.Field != "" {
				solrFields.requireValue(field.Field, "solr field")
			}

			switch field.Name {
			case "access_url":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.AccessURL == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.AccessURL.URLField, fmt.Sprintf("%s section url field", field.Name))
				solrFields.requireValue(field.CustomInfo.AccessURL.LabelField, fmt.Sprintf("%s section label field", field.Name))
				solrFields.requireValue(field.CustomInfo.AccessURL.ProviderField, fmt.Sprintf("%s section provider field", field.Name))
				messageIDs.requireValue(field.CustomInfo.AccessURL.DefaultItemXID, fmt.Sprintf("%s section default item xid", field.Name))

			case "authenticate":

			case "availability":

			case "cover_image":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.CoverImageURL == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				miscValues.requireValue(field.CustomInfo.CoverImageURL.MusicPool, "%s section music pool")

				solrFields.requireValue(field.CustomInfo.CoverImageURL.ThumbnailField, fmt.Sprintf("%s section thumbnail url field", field.Name))
				solrFields.requireValue(field.CustomInfo.CoverImageURL.IDField, fmt.Sprintf("%s section id field", field.Name))
				solrFields.requireValue(field.CustomInfo.CoverImageURL.TitleField, fmt.Sprintf("%s section title field", field.Name))
				solrFields.requireValue(field.CustomInfo.CoverImageURL.PoolField, fmt.Sprintf("%s section pool field", field.Name))

				solrFields.addValue(field.CustomInfo.CoverImageURL.ISBNField)
				solrFields.addValue(field.CustomInfo.CoverImageURL.OCLCField)
				solrFields.addValue(field.CustomInfo.CoverImageURL.LCCNField)
				solrFields.addValue(field.CustomInfo.CoverImageURL.UPCField)

				miscValues.requireValue(p.config.Global.Service.URLTemplates.CoverImages.Host, "cover images template host")
				miscValues.requireValue(p.config.Global.Service.URLTemplates.CoverImages.Path, "cover images template path")
				miscValues.requireValue(p.config.Global.Service.URLTemplates.CoverImages.Pattern, "cover images template pattern")

			case "digital_content_url":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.DigitalContentURL == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.DigitalContentURL.IDField, fmt.Sprintf("%s section id field", field.Name))

				miscValues.requireValue(p.config.Global.Service.URLTemplates.DigitalContent.Host, "digital content template host")
				miscValues.requireValue(p.config.Global.Service.URLTemplates.DigitalContent.Path, "digital content template path")
				miscValues.requireValue(p.config.Global.Service.URLTemplates.DigitalContent.Pattern, "digital content template pattern")

			case "pdf_download_url":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.PdfDownloadURL == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.PdfDownloadURL.URLField, fmt.Sprintf("%s section url field", field.Name))
				solrFields.requireValue(field.CustomInfo.PdfDownloadURL.PIDField, fmt.Sprintf("%s section pid field", field.Name))

			case "published_location":

			case "publisher_name":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.PublisherName == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.PublisherName.AlternateField, fmt.Sprintf("%s section alternate field", field.Name))

			case "ris_authors":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.RISAuthors == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.RISAuthors.AuthorField, fmt.Sprintf("%s section author field", field.Name))

			case "ris_type":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.RISType == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.RISType.FormatField, fmt.Sprintf("%s section format field", field.Name))

			case "sirsi_url":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.SirsiURL == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.SirsiURL.IDField, fmt.Sprintf("%s section id field", field.Name))
				miscValues.requireValue(field.CustomInfo.SirsiURL.IDPrefix, fmt.Sprintf("%s section id prefix", field.Name))

				miscValues.requireValue(p.config.Global.Service.URLTemplates.Sirsi.Host, "sirsi template host")
				miscValues.requireValue(p.config.Global.Service.URLTemplates.Sirsi.Path, "sirsi template path")
				miscValues.requireValue(p.config.Global.Service.URLTemplates.Sirsi.Pattern, "sirsi template pattern")

			case "thumbnail_url":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.ThumbnailURL == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.ThumbnailURL.URLField, fmt.Sprintf("%s section url field", field.Name))

			case "title_subtitle_edition":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.TitleSubtitleEdition == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.SubtitleField, fmt.Sprintf("%s section subtitle field", field.Name))
				solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.EditionField, fmt.Sprintf("%s section edition field", field.Name))

			default:
				log.Printf("[VALIDATE] field %d: unhandled custom field: [%s]", i, field.Name)
				invalid = true
				continue
			}
		} else {
			solrFields.requireValue(field.Field, "solr field")
		}
	}

	// validate solr fields can actually be found in a solr document

	doc := solrDocument{}

	for _, tag := range solrFields.Values() {
		if val := doc.getFieldByTag(tag); val == nil {
			log.Printf("[VALIDATE] field not found in Solr document struct tags: [%s]", tag)
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
				log.Printf("[VALIDATE] [%s] missing translation for message ID: [%s] (%s)", lang, id, err.Error())
				invalid = true
			}
		}
	}

	// check if anything went wrong anywhere

	if invalid || solrFields.Invalid() || messageIDs.Invalid() || miscValues.Invalid() {
		log.Printf("[VALIDATE] exiting due to error(s) above")
		os.Exit(1)
	}

	log.Printf("[POOL] supported languages       = [%s]", strings.Join(langs, ", "))
}

func (p *poolContext) initMappings() {
	invalid := false

	// create mapping from facet XIDs to facet definitions, allowing local overrides
	facetList := append(p.config.Global.Mappings.Definitions.Facets, p.config.Local.Mappings.Definitions.Facets...)
	facetMap := make(map[string]*poolConfigFacet)
	for i := range facetList {
		facetDef := &facetList[i]
		facetMap[facetDef.XID] = facetDef
	}

	// build list of unique facets by XID
	facetXIDs := append(p.config.Global.Mappings.Configured.FacetXIDs, p.config.Local.Mappings.Configured.FacetXIDs...)
	facetsSeen := make(map[string]bool)
	for _, facetXID := range facetXIDs {
		if facetsSeen[facetXID] == true {
			continue
		}

		facetDef := facetMap[facetXID]

		if facetDef == nil {
			log.Printf("[INIT] unrecognized facet xid: [%s]", facetXID)
			invalid = true
			continue
		}

		// this is used to preserve facet order when building facets response
		facetDef.Index = len(facetsSeen)

		p.config.Mappings.Definitions.Facets = append(p.config.Mappings.Definitions.Facets, *facetDef)

		facetsSeen[facetXID] = true
	}

	// create mapping from field names to field definitions, allowing local overrides
	fieldList := append(p.config.Global.Mappings.Definitions.Fields, p.config.Local.Mappings.Definitions.Fields...)
	fieldMap := make(map[string]*poolConfigField)
	for i := range fieldList {
		fieldDef := &fieldList[i]
		fieldMap[fieldDef.Name] = fieldDef
	}

	// build list of unique basic fields by name
	basicFieldNames := append(p.config.Global.Mappings.Configured.FieldNames.Basic, p.config.Local.Mappings.Configured.FieldNames.Basic...)
	basicFieldNamesSeen := make(map[string]bool)
	for _, basicFieldName := range basicFieldNames {
		if basicFieldNamesSeen[basicFieldName] == true {
			continue
		}

		basicFieldDef := fieldMap[basicFieldName]

		if basicFieldDef == nil {
			log.Printf("[INIT] unrecognized basic field name: [%s]", basicFieldName)
			invalid = true
			continue
		}

		basicFieldDef.Properties.Visibility = ""

		p.config.Mappings.Definitions.Fields = append(p.config.Mappings.Definitions.Fields, *basicFieldDef)

		basicFieldNamesSeen[basicFieldName] = true
	}

	// build list of unique detailed fields by name
	detailedFieldNames := append(p.config.Global.Mappings.Configured.FieldNames.Detailed, p.config.Local.Mappings.Configured.FieldNames.Detailed...)
	detailedFieldNamesSeen := make(map[string]bool)
	for _, detailedFieldName := range detailedFieldNames {
		if basicFieldNamesSeen[detailedFieldName] == true {
			log.Printf("[INIT] field exists in both basic and detailed lists: [%s]", detailedFieldName)
			invalid = true
			continue
		}

		if detailedFieldNamesSeen[detailedFieldName] == true {
			continue
		}

		detailedFieldDef := fieldMap[detailedFieldName]

		if detailedFieldDef == nil {
			log.Printf("[INIT] unrecognized detailed field name: [%s]", detailedFieldName)
			invalid = true
			continue
		}

		detailedFieldDef.Properties.Visibility = "detailed"

		p.config.Mappings.Definitions.Fields = append(p.config.Mappings.Definitions.Fields, *detailedFieldDef)

		detailedFieldNamesSeen[detailedFieldName] = true
	}

	// create mapping from sort XIDs to sort definitions, allowing local overrides
	sortList := append(p.config.Global.Mappings.Definitions.Sorts, p.config.Local.Mappings.Definitions.Sorts...)
	sortMap := make(map[string]*poolConfigSort)
	for i := range sortList {
		sortDef := &sortList[i]
		sortMap[sortDef.XID] = sortDef
	}

	// build list of unique sorts by XID
	sortXIDs := append(p.config.Global.Mappings.Configured.SortXIDs, p.config.Local.Mappings.Configured.SortXIDs...)
	sortsSeen := make(map[string]bool)
	for _, sortXID := range sortXIDs {
		if sortsSeen[sortXID] == true {
			continue
		}

		sortDef := sortMap[sortXID]

		if sortDef == nil {
			log.Printf("[INIT] unrecognized sort xid: [%s]", sortXID)
			invalid = true
			continue
		}

		p.config.Mappings.Definitions.Sorts = append(p.config.Mappings.Definitions.Sorts, *sortDef)

		sortsSeen[sortXID] = true
	}

	// relator maps
	p.maps.relatorTerms = make(map[string]string)
	p.maps.relatorCodes = make(map[string]string)

	terms := []string{}

	for i := range p.config.Global.Service.Relators.Map {
		r := &p.config.Global.Service.Relators.Map[i]

		if r.Code == "" || r.Term == "" {
			log.Printf("[INIT] incomplete relator definition: code = [%s]  term = [%s]", r.Code, r.Term)
			invalid = true
			continue
		}

		p.maps.relatorTerms[r.Code] = r.Term
		p.maps.relatorCodes[strings.ToLower(r.Term)] = r.Code

		terms = append(terms, strings.ToLower(r.Term))
	}

	if invalid == true {
		log.Printf("[INIT] exiting due to error(s) above")
		os.Exit(1)
	}
}

func initializePool(cfg *poolConfig) *poolContext {
	p := poolContext{}

	p.config = cfg
	p.randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

	p.initMappings()
	p.initTranslations()
	p.initIdentity()
	p.initProviders()
	p.initVersion()
	p.initSolr()
	p.initPdf()
	p.initRIS()

	p.validateConfig()

	return &p
}
