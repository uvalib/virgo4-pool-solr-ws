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
	healthCheck          poolSolrContext
	scoreThresholdMedium float32
	scoreThresholdHigh   float32
}

type poolTranslations struct {
	bundle *i18n.Bundle
}

// pool-level maps
type poolMaps struct {
	attributes           map[string]v4api.PoolAttribute
	definedSorts         map[string]*poolConfigSort
	definedFields        map[string]*poolConfigField
	definedFilters       map[string]*poolConfigFilter
	preSearchFilters     map[string]*poolConfigFilter
	resourceTypeContexts map[string]*poolConfigResourceTypeContext // per-resource-type facets and fields
	relatorTerms         map[string]string
	relatorCodes         map[string]string
	solrExternalValues   map[string]map[string]string
	solrInternalValues   map[string]map[string]string
}

type poolContext struct {
	randomSource         *rand.Rand
	config               *poolConfig
	translations         poolTranslations
	identity             v4api.PoolIdentity
	providers            v4api.PoolProviders
	version              poolVersion
	solr                 poolSolr
	maps                 poolMaps
	sorts                []*poolConfigSort
	resourceTypeContexts []*poolConfigResourceTypeContext
	titleizer            *titleizeContext
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
		Source:      p.config.Local.Identity.Source,
	}

	// populate supported attributes
	for _, attribute := range p.config.Global.Attributes {
		supported := false
		if sliceContainsString(p.config.Local.Identity.Attributes, attribute, true) {
			supported = true
		}

		p.identity.Attributes = append(p.identity.Attributes, v4api.PoolAttribute{Name: attribute, Supported: supported})
	}

	// create attribute map
	p.maps.attributes = make(map[string]v4api.PoolAttribute)
	for _, attribute := range p.identity.Attributes {
		p.maps.attributes[attribute.Name] = attribute
	}

	// populate supported sorts
	for _, s := range p.sorts {
		p.identity.SortOptions = append(p.identity.SortOptions, v4api.SortOption{ID: s.XID, Asc: s.AscXID, Desc: s.DescXID})
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

	p.solr = poolSolr{
		service:              serviceCtx,
		healthCheck:          healthCtx,
		scoreThresholdMedium: p.config.Local.Solr.ScoreThresholdMedium,
		scoreThresholdHigh:   p.config.Local.Solr.ScoreThresholdHigh,
	}

	log.Printf("[POOL] solr.service.url          = [%s]", p.solr.service.url)
	log.Printf("[POOL] solr.healthCheck.url      = [%s]", p.solr.healthCheck.url)
	log.Printf("[POOL] solr.scoreThresholdMedium = [%0.1f]", p.solr.scoreThresholdMedium)
	log.Printf("[POOL] solr.scoreThresholdHigh   = [%0.1f]", p.solr.scoreThresholdHigh)
}

func (p *poolContext) initCitationFormats() {
	invalid := false

	for i := range p.config.Global.CitationFormats {
		citationFormat := &p.config.Global.CitationFormats[i]

		var err error

		if citationFormat.Format == "" {
			log.Printf("[INIT] empty format in citation format entry %d", i)
			invalid = true
		}

		if citationFormat.Pattern == "" {
			log.Printf("[INIT] empty pattern in citation format entry %d (format: %s)", i, citationFormat.Format)
			invalid = true
			continue
		}

		if citationFormat.re, err = regexp.Compile(citationFormat.Pattern); err != nil {
			log.Printf("[INIT] pattern compilation error in citation format entry %d (format: %s): %s", i, citationFormat.Format, err.Error())
			invalid = true
			continue
		}
	}

	for i := range p.config.Global.Publishers {
		publisher := &p.config.Global.Publishers[i]

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

	var err error

	invalid := false

	var solrFields stringValidator
	var messageIDs stringValidator
	var miscValues stringValidator

	for _, attribute := range p.config.Local.Identity.Attributes {
		if sliceContainsString(p.config.Global.Attributes, attribute, true) == false {
			log.Printf("[VALIDATE] attribute [%s] not found in global attribute list", attribute)
			invalid = true
		}
	}

	miscValues.requireValue(p.config.Global.Service.DefaultSort.XID, "default sort xid")
	miscValues.requireValue(p.config.Global.Service.DefaultSort.Order, "default sort order")

	if p.config.Global.Service.DefaultSort.XID != "" && p.maps.definedSorts[p.config.Global.Service.DefaultSort.XID].XID == "" {
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

	if len(p.config.Local.Solr.Params.PoolFq) == 0 {
		log.Printf("[VALIDATE] solr param pool fq is empty")
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
			solrFields.requireValue(p.config.Local.Related.Image.IIIFManifestField, "iiif manifest field")
			solrFields.requireValue(p.config.Local.Related.Image.IIIFImageField, "iiif image field")
		}
	}

	for i, val := range p.sorts {
		messageIDs.requireValue(val.XID, fmt.Sprintf("sort option %d xid", i))
		solrFields.requireValue(val.Field, fmt.Sprintf("sort option %d group field", i))
		messageIDs.addValue(val.AscXID)
		messageIDs.addValue(val.DescXID)
		messageIDs.addValue(val.RecordXID)

		if val.RecordXID != "" && p.maps.definedSorts[val.RecordXID].XID == "" {
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

	for field, values := range p.maps.solrExternalValues {
		for _, xid := range values {
			messageIDs.requireValue(xid, fmt.Sprintf("solr field [%s] value mapped xid", field))
		}
	}

	for xid, val := range p.maps.definedFilters {
		messageIDs.requireValue(val.XID, fmt.Sprintf("filter [%s] xid", xid))
		if val.Solr.Type == "terms" {
			solrFields.requireValue(val.Solr.Field, fmt.Sprintf("filter [%s] solr field", xid))
		}
	}

	for i, val := range p.config.Global.Publishers {
		solrFields.requireValue(val.Field, fmt.Sprintf("publisher %d solr field", i))
	}

	for i := range p.config.Global.Copyrights {
		val := &p.config.Global.Copyrights[i]

		solrFields.requireValue(val.Field, fmt.Sprintf("copyright %d solr field", i))
		miscValues.requireValue(val.Pattern, fmt.Sprintf("copyright %d pattern", i))

		if val.re, err = regexp.Compile(val.Pattern); err != nil {
			log.Printf("[INIT] pattern compilation error in copyright entry %d: %s", i, err.Error())
			invalid = true
			continue
		}
	}

	solrFields.requireValue(p.config.Global.RecordAttributes.DigitalContent.Field, "record attribute: digital content feature field")
	solrFields.requireValue(p.config.Global.RecordAttributes.Sirsi.Field, "record attribute: sirsi data source field")
	solrFields.requireValue(p.config.Global.RecordAttributes.WSLS.Field, "record attribute: wsls data source field")

	solrFields.requireValue(p.config.Global.ResourceTypes.Field, "resource types: solr field")
	messageIDs.requireValue(p.config.Global.ResourceTypes.FilterXID, "resource types: filter xid")

	for i := range p.resourceTypeContexts {
		r := p.resourceTypeContexts[i]

		for _, val := range r.AuthorFields.Preferred {
			solrFields.requireValue(val, fmt.Sprintf("resource type %d [%s] preferred author field", i, r.Value))
		}
		for _, val := range r.AuthorFields.Fallback {
			solrFields.requireValue(val, fmt.Sprintf("resource type %d [%s] fallback author field", i, r.Value))
		}

		for j, val := range r.filters {
			messageIDs.requireValue(val.XID, fmt.Sprintf("resource type %d [%s] filter %d xid", i, r.Value, j))

			for k, depval := range val.DependentFilterXIDs {
				messageIDs.requireValue(depval, fmt.Sprintf("resource type %d [%s] filter %d dependent xid %d", i, r.Value, j, k))
			}

			for k, q := range val.ComponentQueries {
				messageIDs.requireValue(q.XID, fmt.Sprintf("resource type %d [%s] filter %d component query xid %d", i, r.Value, j, k))
				miscValues.requireValue(q.Query, fmt.Sprintf("resource type %d [%s] filter %d component query query %d", i, r.Value, j, k))
			}
		}

		allFields := append(r.fields.basic, r.fields.detailed...)

		for j, field := range allFields {
			prefix := fmt.Sprintf("resource type %d [%s] field index %d: ", i, r.Value, j)
			postfix := fmt.Sprintf(` -- {Name:"%s" XID:"%s" Field:"%s"}`, field.Name, field.XID, field.Field)

			solrFields.setPrefix(prefix)
			messageIDs.setPrefix(prefix)
			miscValues.setPrefix(prefix)

			solrFields.setPostfix(postfix)
			messageIDs.setPostfix(postfix)
			miscValues.setPostfix(postfix)

			// start validating

			messageIDs.addValue(field.XID)
			messageIDs.addValue(field.WSLSXID)

			miscValues.requireValue(field.Name, "name")

			if field.Custom == true {
				if field.Field != "" {
					solrFields.requireValue(field.Field, "solr field")
				}

				switch field.Name {
				case "abstract":
					if field.CustomInfo == nil || field.CustomInfo.Abstract == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.Abstract.AlternateField, fmt.Sprintf("%s section alternate field", field.Name))

				case "access_url":
					if field.CustomInfo == nil || field.CustomInfo.AccessURL == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.AccessURL.URLField, fmt.Sprintf("%s section url field", field.Name))
					solrFields.requireValue(field.CustomInfo.AccessURL.LabelField, fmt.Sprintf("%s section label field", field.Name))
					solrFields.requireValue(field.CustomInfo.AccessURL.ProviderField, fmt.Sprintf("%s section provider field", field.Name))
					messageIDs.requireValue(field.CustomInfo.AccessURL.DefaultItemXID, fmt.Sprintf("%s section default item xid", field.Name))

				case "authenticate":

				case "author":

				case "author_list":

				case "availability":

				case "citation_advisor":

				case "citation_author":

				case "citation_compiler":

				case "citation_editor":

				case "citation_format":

				case "citation_is_online_only":
					if field.CustomInfo == nil || field.CustomInfo.CitationOnlineOnly == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					for k, f := range field.CustomInfo.CitationOnlineOnly.ComparisonFields {
						solrFields.requireValue(f.Field, fmt.Sprintf("%s section online field %d solr field", field.Name, k))
					}

				case "citation_is_virgo_url":
					if field.CustomInfo == nil || field.CustomInfo.CitationVirgoURL == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					for k, f := range field.CustomInfo.CitationVirgoURL.ComparisonFields {
						solrFields.requireValue(f.Field, fmt.Sprintf("%s section online field %d solr field", field.Name, k))
					}

				case "citation_subtitle":

				case "citation_title":

				case "citation_translator":

				case "composer_performer":

				case "copyright_and_permissions":

				case "cover_image_url":
					if field.CustomInfo == nil || field.CustomInfo.CoverImageURL == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					miscValues.requireValue(field.CustomInfo.CoverImageURL.MusicPool, "%s section music pool")

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

				case "creator":

				case "digital_content_url":
					if field.CustomInfo == nil || field.CustomInfo.DigitalContentURL == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.DigitalContentURL.IDField, fmt.Sprintf("%s section id field", field.Name))

					miscValues.requireValue(p.config.Global.Service.URLTemplates.DigitalContent.Host, "digital content template host")
					miscValues.requireValue(p.config.Global.Service.URLTemplates.DigitalContent.Path, "digital content template path")
					miscValues.requireValue(p.config.Global.Service.URLTemplates.DigitalContent.Pattern, "digital content template pattern")

				case "language":
					if field.CustomInfo == nil || field.CustomInfo.Language == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.Language.AlternateField, fmt.Sprintf("%s section alternate field", field.Name))

				case "online_related":
					if field.CustomInfo == nil || field.CustomInfo.AccessURL == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.AccessURL.URLField, fmt.Sprintf("%s section url field", field.Name))
					solrFields.requireValue(field.CustomInfo.AccessURL.LabelField, fmt.Sprintf("%s section label field", field.Name))
					messageIDs.requireValue(field.CustomInfo.AccessURL.DefaultItemXID, fmt.Sprintf("%s section default item xid", field.Name))

				case "published_location":

				case "publisher_name":
					if field.CustomInfo == nil || field.CustomInfo.PublisherName == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.PublisherName.AlternateField, fmt.Sprintf("%s section alternate field", field.Name))

				case "related_resources":
					if field.CustomInfo == nil || field.CustomInfo.AccessURL == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.AccessURL.URLField, fmt.Sprintf("%s section url field", field.Name))
					solrFields.requireValue(field.CustomInfo.AccessURL.LabelField, fmt.Sprintf("%s section label field", field.Name))
					messageIDs.requireValue(field.CustomInfo.AccessURL.DefaultItemXID, fmt.Sprintf("%s section default item xid", field.Name))

				case "sirsi_url":
					if field.CustomInfo == nil || field.CustomInfo.SirsiURL == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.SirsiURL.IDField, fmt.Sprintf("%s section id field", field.Name))
					miscValues.requireValue(field.CustomInfo.SirsiURL.IDPrefix, fmt.Sprintf("%s section id prefix", field.Name))

					miscValues.requireValue(p.config.Global.Service.URLTemplates.Sirsi.Host, "sirsi template host")
					miscValues.requireValue(p.config.Global.Service.URLTemplates.Sirsi.Path, "sirsi template path")
					miscValues.requireValue(p.config.Global.Service.URLTemplates.Sirsi.Pattern, "sirsi template pattern")

				case "summary_holdings":

				case "title_subtitle_edition":
					if field.CustomInfo == nil || field.CustomInfo.TitleSubtitleEdition == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.TitleField, fmt.Sprintf("%s section title field", field.Name))
					solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.SubtitleField, fmt.Sprintf("%s section subtitle field", field.Name))
					solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.EditionField, fmt.Sprintf("%s section edition field", field.Name))

				case "vernacularized_author":

				case "vernacularized_composer_performer":

				case "vernacularized_creator":

				case "vernacularized_title":
					if field.CustomInfo == nil || field.CustomInfo.TitleSubtitleEdition == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.TitleField, fmt.Sprintf("%s section title field", field.Name))

				case "vernacularized_title_subtitle_edition":
					if field.CustomInfo == nil || field.CustomInfo.TitleSubtitleEdition == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.TitleField, fmt.Sprintf("%s section title field", field.Name))
					solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.SubtitleField, fmt.Sprintf("%s section subtitle field", field.Name))
					solrFields.requireValue(field.CustomInfo.TitleSubtitleEdition.EditionField, fmt.Sprintf("%s section edition field", field.Name))

				case "wsls_collection_description":
					if field.CustomInfo == nil || field.CustomInfo.WSLSCollectionDescription == nil {
						log.Printf("[VALIDATE] missing field index %d custom_info/%s section", j, field.Name)
						invalid = true
						continue
					}

					messageIDs.requireValue(field.CustomInfo.WSLSCollectionDescription.ValueXID, fmt.Sprintf("%s section edition field", field.Name))

				default:
					log.Printf("[VALIDATE] field %d: unhandled custom field: [%s]", j, field.Name)
					invalid = true
					continue
				}
			} else {
				if field.Value == "" {
					solrFields.requireValue(field.Field, "solr field")
				}
			}
		}
	}

	// validate xids can actually be translated

	langs := []string{}

	for _, tag := range p.translations.bundle.LanguageTags() {
		lang := tag.String()
		langs = append(langs, lang)
		localizer := i18n.NewLocalizer(p.translations.bundle, lang)
		for _, id := range messageIDs.Values() {
			_, xtag, xerr := localizer.LocalizeWithTag(&i18n.LocalizeConfig{MessageID: id})
			if xerr != nil {
				log.Printf("[VALIDATE] [%s] [%s] translation error: %s", lang, id, xerr.Error())
				invalid = true
				continue
			}
			if xtag != tag {
				log.Printf("[VALIDATE] [%s] [%s] translated message has unexpected language (%s); missing translation?", lang, id, xtag)
				invalid = true
				continue
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

func (p *poolContext) populateFieldList(r *poolConfigResourceTypeContext, required []string, optional []string) ([]poolConfigField, bool) {
	var fields []poolConfigField

	invalid := false

	requiredFields := len(required)
	fieldNames := append(required, optional...)

	fieldNamesSeen := make(map[string]bool)

	for i, fieldName := range fieldNames {
		if fieldName == "" {
			log.Printf("[INIT] empty field name")
			invalid = true
			continue
		}

		if fieldNamesSeen[fieldName] == true {
			continue
		}

		fieldDef := p.maps.definedFields[fieldName]

		if fieldDef == nil {
			log.Printf("[INIT] unrecognized field name: [%s]", fieldName)
			invalid = true
			continue
		}

		if i < requiredFields {
			// we're working with required fields; check which one and set any overrides
			switch fieldName {
			case r.FieldNames.Title.Name:
				fieldDef.Properties.Type = r.FieldNames.Title.Type
				fieldDef.Properties.CitationPart = r.FieldNames.Title.CitationPart

			case r.FieldNames.TitleVernacular.Name:
				fieldDef.Properties.Type = r.FieldNames.TitleVernacular.Type
				fieldDef.Properties.CitationPart = r.FieldNames.TitleVernacular.CitationPart

			case r.FieldNames.Author.Name:
				fieldDef.Properties.Type = r.FieldNames.Author.Type
				fieldDef.Properties.CitationPart = r.FieldNames.Author.CitationPart

			case r.FieldNames.AuthorVernacular.Name:
				fieldDef.Properties.Type = r.FieldNames.AuthorVernacular.Type
				fieldDef.Properties.CitationPart = r.FieldNames.AuthorVernacular.CitationPart

			default:
				log.Printf("[INIT] unrecognized required field name: [%s]", fieldName)
				invalid = true
				continue
			}
		}

		fields = append(fields, *fieldDef)

		fieldNamesSeen[fieldName] = true
	}

	return fields, invalid
}

func (p *poolContext) initSorts() {
	invalid := false

	// configure globally defined sorts, and map their XIDs to sort definitions.
	// NOTE: all pools define (and use) the same list since this is a solr-level config.
	p.maps.definedSorts = make(map[string]*poolConfigSort)
	for i := range p.config.Global.Mappings.Definitions.Sorts {
		def := &p.config.Global.Mappings.Definitions.Sorts[i]

		if p.maps.definedSorts[def.XID] != nil {
			log.Printf("[SORTS] duplicate sort xid: [%s]", def.XID)
			invalid = true
			continue
		}

		if def.IsRelevance == true {
			def.RecordXID = p.config.Local.Solr.RelevanceIntraGroupSort.XID
			def.RecordOrder = p.config.Local.Solr.RelevanceIntraGroupSort.Order
		}

		p.maps.definedSorts[def.XID] = def
	}

	// create sort list based on defined sorts
	seen := make(map[string]bool)
	for _, xid := range p.config.Global.Mappings.Configured.SortXIDs {
		if seen[xid] == true {
			continue
		}

		def := p.maps.definedSorts[xid]
		if def == nil {
			log.Printf("[SORTS] unrecognized sort xid: [%s]", xid)
			invalid = true
			continue
		}

		p.sorts = append(p.sorts, def)

		seen[xid] = true
	}

	if invalid == true {
		log.Printf("[SORTS] exiting due to error(s) above")
		os.Exit(1)
	}
}

func (p *poolContext) initFields() {
	invalid := false

	// configure globally defined fields, and map their XIDs to field definitions.
	// NOTE: all pools define the same list; a given pool may only use a subset of these.
	p.maps.definedFields = make(map[string]*poolConfigField)
	for i := range p.config.Global.Mappings.Definitions.Fields {
		def := &p.config.Global.Mappings.Definitions.Fields[i]

		if p.maps.definedFields[def.Name] != nil {
			log.Printf("[FIELDS] duplicate field name: [%s]", def.Name)
			invalid = true
			continue
		}

		p.maps.definedFields[def.Name] = def
	}

	if invalid == true {
		log.Printf("[FIELDS] exiting due to error(s) above")
		os.Exit(1)
	}
}

func (p *poolContext) initFilters() {
	invalid := false

	// availability filter setup
	p.config.Global.Availability.ExposedValues = []string{}
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.OnShelf...)
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.Online...)
	p.config.Global.Availability.ExposedValues = append(p.config.Global.Availability.ExposedValues, p.config.Global.Availability.Values.Other...)

	// configure globally defined filters, and map their XIDs to filter definitions.
	// NOTE: all pools define the same list; a given pool may only use a subset of these.
	p.maps.definedFilters = make(map[string]*poolConfigFilter)
	for i := range p.config.Global.Mappings.Definitions.Filters {
		def := &p.config.Global.Mappings.Definitions.Filters[i]

		if p.maps.definedFilters[def.XID] != nil {
			log.Printf("[FILTERS] duplicate filter xid: [%s]", def.XID)
			invalid = true
			continue
		}

		// configure availability filter
		if def.IsAvailability == true {
			def.Solr.Field = p.config.Global.Availability.Anon.Facet
			def.Solr.FieldAuth = p.config.Global.Availability.Auth.Facet
			def.ExposedValues = p.config.Global.Availability.ExposedValues
		}

		// for component query facets, create mappings from any
		// possible translated value back to the query definition
		if len(def.ComponentQueries) > 0 {
			def.queryMap = make(map[string]*poolConfigFacetQuery)

			for j := range def.ComponentQueries {
				q := &def.ComponentQueries[j]

				for _, tag := range p.translations.bundle.LanguageTags() {
					lang := tag.String()
					localizer := i18n.NewLocalizer(p.translations.bundle, lang)
					msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: q.XID})

					if err != nil {
						log.Printf("[FILTERS] [%s] missing translation for message ID: [%s] (%s)", lang, q.XID, err.Error())
						invalid = true
						continue
					}

					def.queryMap[msg] = q
				}
			}
		}

		p.maps.definedFilters[def.XID] = def
	}

	// create pre-search filter map based on configured pre-search filters
	p.maps.preSearchFilters = make(map[string]*poolConfigFilter)
	for _, xid := range p.config.Global.Mappings.Configured.FilterXIDs {
		if p.maps.preSearchFilters[xid] != nil {
			continue
		}

		orig := p.maps.definedFilters[xid]
		if orig == nil {
			log.Printf("[MAPCFG] unrecognized filter xid: [%s]", xid)
			invalid = true
			continue
		}

		// create a copy of the definition to avoid index confusion
		def := *orig

		// this is used to preserve filter order when building filters response
		def.Index = len(p.maps.preSearchFilters)

		p.maps.preSearchFilters[def.XID] = &def
	}

	if invalid == true {
		log.Printf("[FILTERS] exiting due to error(s) above")
		os.Exit(1)
	}
}

func (p *poolContext) initResourceTypes() {
	invalid := false

	// configure globally defined resource types, and map their XIDs to resource type definitions.
	// NOTE: all pools define the same list; a given pool may only use a subset of these.
	p.maps.resourceTypeContexts = make(map[string]*poolConfigResourceTypeContext)
	for i := range p.config.Global.ResourceTypes.Contexts {
		def := &p.config.Global.ResourceTypes.Contexts[i]

		if p.maps.resourceTypeContexts[def.Value] != nil {
			log.Printf("[RESTYPES] duplicate resource type value: [%s]", def.Value)
			invalid = true
			continue
		}

		p.maps.resourceTypeContexts[def.Value] = def
		p.maps.resourceTypeContexts[def.XID] = def

		// since this is not a configured value, we can build the definitive list now
		p.resourceTypeContexts = append(p.resourceTypeContexts, def)
	}

	// NOTE: the following resource type setup loops are broken out for readability

	// for each resource type, set up its facets and facet map
	for i := range p.resourceTypeContexts {
		r := p.resourceTypeContexts[i]

		// create ordered facet list and convenience map
		r.filterMap = make(map[string]*poolConfigFilter)

		// append resource-type filters after global pre-search filters
		filterXIDs := append(p.config.Global.Mappings.Configured.FilterXIDs, r.FilterXIDs...)

		seen := make(map[string]bool)
		for _, xid := range filterXIDs {
			if seen[xid] == true {
				continue
			}

			orig := p.maps.definedFilters[xid]
			if orig == nil {
				log.Printf("[MAPCFG] resource type value [%s] contains unrecognized filter xid: [%s]", r.Value, xid)
				invalid = true
				continue
			}

			// create a copy of the definition to avoid index confusion
			def := *orig

			// this is used to preserve facet order when building facets response
			def.Index = len(seen)

			r.filters = append(r.filters, def)
			r.filterMap[xid] = &def

			seen[xid] = true
		}
	}

	// for each resource type, set up its fields and field map

	for i := range p.resourceTypeContexts {
		r := p.resourceTypeContexts[i]

		// create basic/detailed field lists

		// first, add special title/author fields
		// these will be populated in perhaps the most ugly fashion below

		// these are required
		requiredFieldNames := []string{
			r.FieldNames.Title.Name,
			r.FieldNames.Author.Name,
		}

		// these are optional
		if r.FieldNames.TitleVernacular.Name != "" {
			requiredFieldNames = append(requiredFieldNames, r.FieldNames.TitleVernacular.Name)
		}

		if r.FieldNames.AuthorVernacular.Name != "" {
			requiredFieldNames = append(requiredFieldNames, r.FieldNames.AuthorVernacular.Name)
		}

		var fieldListInvalid bool

		// build list of unique basic fields by name
		basicFieldNames := append(r.FieldNames.Basic, p.config.Global.Mappings.Configured.FieldNames.Basic...)
		r.fields.basic, fieldListInvalid = p.populateFieldList(r, requiredFieldNames, basicFieldNames)
		invalid = invalid || fieldListInvalid

		// build list of unique detailed fields by name
		detailedFieldNames := append(r.FieldNames.Detailed, p.config.Global.Mappings.Configured.FieldNames.Detailed...)
		r.fields.detailed, fieldListInvalid = p.populateFieldList(r, requiredFieldNames, detailedFieldNames)
		invalid = invalid || fieldListInvalid
	}

	// for each resource type, set up its solr value maps

	p.maps.solrExternalValues = make(map[string]map[string]string)
	p.maps.solrInternalValues = make(map[string]map[string]string)

	forwardMap := make(map[string]string)
	reverseMap := make(map[string]string)

	for i := range p.resourceTypeContexts {
		r := p.resourceTypeContexts[i]

		// if there is no translation for this field, do not map it.
		// this effectively hides it from the client.

		if r.XID == "" {
			continue
		}

		// solr internal/external field value forward/reverse maps

		for _, tag := range p.translations.bundle.LanguageTags() {
			lang := tag.String()
			localizer := i18n.NewLocalizer(p.translations.bundle, lang)
			msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: r.XID})

			if err != nil {
				log.Printf("[MAPCFG] [%s] missing translation for message ID: [%s] (%s)", lang, r.XID, err.Error())
				invalid = true
				continue
			}

			reverseMap[msg] = r.Value
		}

		forwardMap[r.Value] = r.XID
		reverseMap[r.XID] = r.Value
	}

	p.maps.solrExternalValues[p.config.Global.ResourceTypes.Field] = forwardMap
	p.maps.solrInternalValues[p.config.Global.ResourceTypes.Field] = reverseMap

	if invalid == true {
		log.Printf("[RESTYPES] exiting due to error(s) above")
		os.Exit(1)
	}
}

func (p *poolContext) initRelators() {
	invalid := false

	// relator maps
	p.maps.relatorTerms = make(map[string]string)
	p.maps.relatorCodes = make(map[string]string)

	for i := range p.config.Global.Relators.Map {
		r := &p.config.Global.Relators.Map[i]

		if r.Code == "" || r.Term == "" {
			log.Printf("[RELATORS] incomplete relator definition: code = [%s]  term = [%s]", r.Code, r.Term)
			invalid = true
			continue
		}

		p.maps.relatorTerms[r.Code] = r.Term
		p.maps.relatorCodes[strings.ToLower(r.Term)] = r.Code
	}

	if invalid == true {
		log.Printf("[RELATORS] exiting due to error(s) above")
		os.Exit(1)
	}
}

func (p *poolContext) initTitleizer() {
	cfg := titleizeConfig{
		debug:           false,
		wordDelimiters:  p.config.Global.Titleization.CharacterSets.WordDelimiters,
		partDelimiters:  p.config.Global.Titleization.CharacterSets.PartDelimiters,
		mixedCaseWords:  p.config.Global.Titleization.WordLists.MixedCaseWords,
		upperCaseWords:  p.config.Global.Titleization.WordLists.UpperCaseWords,
		lowerCaseWords:  p.config.Global.Titleization.WordLists.LowerCaseWords,
		multiPartWords:  p.config.Global.Titleization.WordLists.MultiPartWords,
		ordinalPatterns: p.config.Global.Titleization.WordLists.OrdinalPatterns,
	}

	p.titleizer = newTitleizeContext(&cfg)
}

func initializePool(cfg *poolConfig) *poolContext {
	p := poolContext{}

	p.config = cfg
	p.randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

	p.initVersion()
	p.initSolr()
	p.initTranslations()
	p.initIdentity()
	p.initProviders()
	p.initCitationFormats()
	p.initTitleizer()
	p.initRelators()

	p.initSorts()
	p.initFields()
	p.initFilters()
	p.initResourceTypes()

	p.validateConfig()

	return &p
}
