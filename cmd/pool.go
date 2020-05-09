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

type poolSolr struct {
	serviceClient        *http.Client
	healthcheckClient    *http.Client
	url                  string
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
	risCodes        map[string]string
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
		Attributes:  p.config.Local.Identity.Attributes,
	}

	// create sort field map
	p.maps.sortFields = make(map[string]poolConfigSort)
	for i := range p.config.Mappings.Sorts {
		s := &p.config.Mappings.Sorts[i]
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

func (p *poolContext) initSolr() {
	// service client setup

	svcConnTimeout := timeoutWithMinimum(p.config.Local.Solr.Clients.Service.ConnTimeout, 1)
	svcReadTimeout := timeoutWithMinimum(p.config.Local.Solr.Clients.Service.ReadTimeout, 1)

	svcClient := &http.Client{
		Timeout: time.Duration(svcReadTimeout) * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(svcConnTimeout) * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
			MaxIdleConns:        100, // we are hitting one solr host, so
			MaxIdleConnsPerHost: 100, // these two values can be the same
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// health check client setup

	hcConnTimeout := timeoutWithMinimum(p.config.Local.Solr.Clients.HealthCheck.ConnTimeout, 1)
	hcReadTimeout := timeoutWithMinimum(p.config.Local.Solr.Clients.HealthCheck.ReadTimeout, 1)

	hcClient := &http.Client{
		Timeout: time.Duration(hcReadTimeout) * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(hcConnTimeout) * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
			MaxIdleConns:        100, // we are hitting one solr host, so
			MaxIdleConnsPerHost: 100, // these two values can be the same
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// create facet map
	p.maps.availableFacets = make(map[string]poolConfigFacet)

	p.config.Global.Availability.ExposedValues = []string{}
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.OnShelf...)
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.Online...)
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.Other...)

	for i := range p.config.Mappings.Facets {
		f := &p.config.Mappings.Facets[i]

		// configure availability facet while we're here
		if f.IsAvailability == true {
			f.Solr.Field = p.config.Global.Availability.Anon.Facet
			f.Solr.FieldAuth = p.config.Global.Availability.Auth.Facet
			f.ExposedValues = p.config.Global.Availability.ExposedValues
		}

		p.maps.availableFacets[f.XID] = *f
	}

	p.solr = poolSolr{
		url:                  fmt.Sprintf("%s/%s/%s", p.config.Local.Solr.Host, p.config.Local.Solr.Core, p.config.Local.Solr.Handler),
		serviceClient:        svcClient,
		healthcheckClient:    hcClient,
		scoreThresholdMedium: p.config.Local.Solr.ScoreThresholdMedium,
		scoreThresholdHigh:   p.config.Local.Solr.ScoreThresholdHigh,
	}

	// create RIS code map
	p.maps.risCodes = make(map[string]string)
	for _, code := range p.config.Global.RISCodes {
		p.maps.risCodes[code.Field] = code.Code
	}

	log.Printf("[POOL] solr.url                  = [%s]", p.solr.url)
	log.Printf("[POOL] solr.scoreThresholdMedium = [%0.1f]", p.solr.scoreThresholdMedium)
	log.Printf("[POOL] solr.scoreThresholdHigh   = [%0.1f]", p.solr.scoreThresholdHigh)
}

func (p *poolContext) initPdf() {
	// client setup

	connTimeout := timeoutWithMinimum(p.config.Global.Service.Pdf.ConnTimeout, 1)
	readTimeout := timeoutWithMinimum(p.config.Global.Service.Pdf.ReadTimeout, 1)

	pdfClient := &http.Client{
		Timeout: time.Duration(readTimeout) * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(connTimeout) * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
			MaxIdleConns:        100, // we are most likely hitting one pdf host, so
			MaxIdleConnsPerHost: 100, // these two values can be the same
			IdleConnTimeout:     90 * time.Second,
		},
	}

	p.pdf = poolPdf{
		client: pdfClient,
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
	miscValues.requireValue(p.config.Local.Solr.Handler, "solr handler")
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

	if p.config.Local.Identity.Mode == "image" {
		if p.config.Local.Related == nil {
			log.Printf("[VALIDATE] missing related section")
			invalid = true
		} else if p.config.Local.Related.Image == nil {
			log.Printf("[VALIDATE] missing related image section")
			invalid = true
		} else {
			solrFields.requireValue(p.config.Local.Related.Image.IDField, "iiif id field")
			solrFields.requireValue(p.config.Local.Related.Image.IdentifierField, "iiif identifier field")
			solrFields.requireValue(p.config.Local.Related.Image.IIIFManifestField, "iiif manifest field")
			solrFields.requireValue(p.config.Local.Related.Image.IIIFImageField, "iiif image field")

			miscValues.requireValue(p.config.Global.Service.URLTemplates.IIIF.Template, "iiif template url")
			miscValues.requireValue(p.config.Global.Service.URLTemplates.IIIF.Pattern, "iiif template pattern")
		}
	}

	for i, val := range p.config.Mappings.Sorts {
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

	for i, val := range p.config.Mappings.Facets {
		messageIDs.requireValue(val.XID, fmt.Sprintf("facet %d xid", i))
		for j, depval := range val.DependentFacetXIDs {
			messageIDs.requireValue(depval, fmt.Sprintf("facet %d dependent xid %d", i, j))
		}
	}

	for i, field := range p.config.Mappings.Fields {
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

				miscValues.requireValue(p.config.Global.Service.URLTemplates.CoverImages.Template, "cover images template url")
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

				miscValues.requireValue(p.config.Global.Service.URLTemplates.DigitalContent.Template, "digital content template url")
				miscValues.requireValue(p.config.Global.Service.URLTemplates.DigitalContent.Pattern, "digital content template pattern")

			case "iiif_base_url":
				if field.CustomInfo == nil {
					log.Printf("[VALIDATE] missing field index %d %s custom_info section", i, field.Name)
					invalid = true
					continue
				}

				if field.CustomInfo.IIIFBaseURL == nil {
					log.Printf("[VALIDATE] missing field index %d %s section", i, field.Name)
					invalid = true
					continue
				}

				solrFields.requireValue(field.CustomInfo.IIIFBaseURL.IdentifierField, fmt.Sprintf("%s section identifier field", field.Name))

				miscValues.requireValue(p.config.Global.Service.URLTemplates.IIIF.Template, "iiif template url")
				miscValues.requireValue(p.config.Global.Service.URLTemplates.IIIF.Pattern, "iiif template pattern")

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

				miscValues.requireValue(p.config.Global.Service.URLTemplates.Sirsi.Template, "sirsi template url")
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
	facetList := append(p.config.Global.Mappings.Facets, p.config.Local.Mappings.Facets...)
	facetMap := make(map[string]*poolConfigFacet)
	for i := range facetList {
		facetDef := &facetList[i]
		facetMap[facetDef.XID] = facetDef
	}

	// build list of unique facets by XID
	facetXIDs := append(p.config.Global.Mappings.FacetXIDs, p.config.Local.Mappings.FacetXIDs...)
	facetXIDSelected := make(map[string]bool)
	for _, facetXID := range facetXIDs {
		if facetXIDSelected[facetXID] == true {
			continue
		}

		facetDef := facetMap[facetXID]

		if facetDef == nil {
			log.Printf("[INIT] unrecognized facet xid: [%s]", facetXID)
			invalid = true
			continue
		}

		p.config.Mappings.Facets = append(p.config.Mappings.Facets, *facetDef)
		facetXIDSelected[facetXID] = true
	}

	// create mapping from field names to field definitions, allowing local overrides
	fieldList := append(p.config.Global.Mappings.Fields, p.config.Local.Mappings.Fields...)
	fieldMap := make(map[string]*poolConfigField)
	for i := range fieldList {
		fieldDef := &fieldList[i]
		fieldMap[fieldDef.Name] = fieldDef
	}

	// build list of unique fields by name
	fieldNames := append(p.config.Global.Mappings.FieldNames, p.config.Local.Mappings.FieldNames...)
	fieldNameSelected := make(map[string]bool)
	for _, fieldName := range fieldNames {
		if fieldNameSelected[fieldName] == true {
			continue
		}

		fieldDef := fieldMap[fieldName]

		if fieldDef == nil {
			log.Printf("[INIT] unrecognized field name: [%s]", fieldName)
			invalid = true
			continue
		}

		p.config.Mappings.Fields = append(p.config.Mappings.Fields, *fieldDef)
		fieldNameSelected[fieldName] = true
	}

	// create mapping from sort XIDs to sort definitions, allowing local overrides
	sortList := append(p.config.Global.Mappings.Sorts, p.config.Local.Mappings.Sorts...)
	sortMap := make(map[string]*poolConfigSort)
	for i := range sortList {
		sortDef := &sortList[i]
		sortMap[sortDef.XID] = sortDef
	}

	// build list of unique sorts by XID
	sortXIDs := append(p.config.Global.Mappings.SortXIDs, p.config.Local.Mappings.SortXIDs...)
	sortXIDSelected := make(map[string]bool)
	for _, sortXID := range sortXIDs {
		if sortXIDSelected[sortXID] == true {
			continue
		}

		sortDef := sortMap[sortXID]

		if sortDef == nil {
			log.Printf("[INIT] unrecognized sort xid: [%s]", sortXID)
			invalid = true
			continue
		}

		p.config.Mappings.Sorts = append(p.config.Mappings.Sorts, *sortDef)
		sortXIDSelected[sortXID] = true
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

	p.validateConfig()

	return &p
}
