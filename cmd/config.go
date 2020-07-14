package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

const envPrefix = "VIRGO4_SOLR_POOL_WS"

type poolConfigURLTemplate struct {
	Host    string `json:"host,omitempty"`
	Path    string `json:"path,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

type poolConfigURLTemplates struct {
	Sirsi          poolConfigURLTemplate `json:"sirsi,omitempty"`
	CoverImages    poolConfigURLTemplate `json:"cover_images,omitempty"`
	DigitalContent poolConfigURLTemplate `json:"digital_content,omitempty"`
}

type poolConfigPublisher struct {
	ID        string `json:"id,omitempty"`
	Field     string `json:"field,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Publisher string `json:"publisher,omitempty"`
	Place     string `json:"place,omitempty"`
	re        *regexp.Regexp
}

type poolConfigRelator struct {
	Code string `json:"code,omitempty"`
	Term string `json:"term,omitempty"`
}

type poolConfigRelators struct {
	AuthorCodes  []string            `json:"author_codes,omitempty"`
	AdvisorCodes []string            `json:"advisor_codes,omitempty"`
	EditorCodes  []string            `json:"editor_codes,omitempty"`
	Map          []poolConfigRelator `json:"map,omitempty"`
}

type poolConfigCopyrightLabel struct {
	Text  string `json:"text,omitempty"`
	Label string `json:"label,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

type poolConfigCopyrightLabels struct {
	Prefix      string                     `json:"prefix,omitempty"`
	Suffix      string                     `json:"suffix,omitempty"`
	Split       string                     `json:"split,omitempty"`
	Join        string                     `json:"join,omitempty"`
	Labels      []poolConfigCopyrightLabel `json:"labels,omitempty"`
	DefaultIcon string                     `json:"default_icon,omitempty"`
}

type poolConfigCopyright struct {
	Field      string                    `json:"field,omitempty"`
	Pattern    string                    `json:"pattern,omitempty"`
	URL        string                    `json:"url,omitempty"`
	Label      string                    `json:"label,omitempty"`
	Icon       string                    `json:"icon,omitempty"`
	IconPath   string                    `json:"icon_path,omitempty"`
	URLGroup   int                       `json:"url_group,omitempty"`
	PathGroup  int                       `json:"path_group,omitempty"`
	PathLabels poolConfigCopyrightLabels `json:"path_labels,omitempty"`
	CodeGroup  int                       `json:"code_group,omitempty"`
	CodeLabels poolConfigCopyrightLabels `json:"code_labels,omitempty"`
	re         *regexp.Regexp
}

type poolConfigService struct {
	Port         string                 `json:"port,omitempty"`
	JWTKey       string                 `json:"jwt_key,omitempty"`
	DefaultSort  poolConfigSort         `json:"default_sort,omitempty"`
	URLTemplates poolConfigURLTemplates `json:"url_templates,omitempty"`
	Pdf          poolConfigPdf          `json:"pdf,omitempty"`
}

type poolConfigSolrParams struct {
	Qt      string   `json:"qt,omitempty"`
	DefType string   `json:"deftype,omitempty"`
	Fq      []string `json:"fq,omitempty"` // pool definition should go here
	Fl      []string `json:"fl,omitempty"`
}

type poolConfigSolrClient struct {
	Endpoint    string `json:"endpoint,omitempty"`
	ConnTimeout string `json:"conn_timeout,omitempty"`
	ReadTimeout string `json:"read_timeout,omitempty"`
}

type poolConfigSolrClients struct {
	Service     poolConfigSolrClient `json:"service,omitempty"`
	HealthCheck poolConfigSolrClient `json:"healthcheck,omitempty"`
}

type poolConfigAuthorFields struct {
	PreferredHeaderField string `json:"preferred_header_field,omitempty"`
	InitialAuthorField   string `json:"initial_author_field,omitempty"`
	PreferredAuthorField string `json:"preferred_author_field,omitempty"`
	FallbackAuthorField  string `json:"fallback_author_field,omitempty"`
}

type poolConfigSolr struct {
	Host                    string                 `json:"host,omitempty"`
	Core                    string                 `json:"core,omitempty"`
	Clients                 poolConfigSolrClients  `json:"clients,omitempty"`
	Params                  poolConfigSolrParams   `json:"params,omitempty"`
	GroupField              string                 `json:"group_field,omitempty"`
	RelevanceIntraGroupSort poolConfigSort         `json:"relevance_intra_group_sort,omitempty"`
	ExactMatchTitleField    string                 `json:"exact_match_title_field,omitempty"`
	ScoreThresholdMedium    float32                `json:"score_threshold_medium,omitempty"`
	ScoreThresholdHigh      float32                `json:"score_threshold_high,omitempty"`
	AuthorFields            poolConfigAuthorFields `json:"author_fields,omitempty"`
}

type poolConfigPdfEndpoints struct {
	Generate string `json:"generate,omitempty"`
	Status   string `json:"status,omitempty"`
	Download string `json:"download,omitempty"`
	Delete   string `json:"delete,omitempty"`
}

type poolConfigPdf struct {
	ConnTimeout string                 `json:"conn_timeout,omitempty"`
	ReadTimeout string                 `json:"read_timeout,omitempty"`
	Endpoints   poolConfigPdfEndpoints `json:"endpoints,omitempty"`
	ReadyValues []string               `json:"ready_values,omitempty"`
}

type poolConfigFieldProperties struct {
	Type       string `json:"type,omitempty"`
	Separator  string `json:"separator,omitempty"`
	Display    string `json:"display,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

type poolConfigFieldTypeCustom struct {
	AlternateField string   `json:"alternate_field,omitempty"` // field names
	EditionField   string   `json:"edition_field,omitempty"`
	FormatField    string   `json:"format_field,omitempty"`
	IDField        string   `json:"id_field,omitempty"`
	ISBNField      string   `json:"isbn_field,omitempty"`
	LCCNField      string   `json:"lccn_field,omitempty"`
	LabelField     string   `json:"label_field,omitempty"`
	OCLCField      string   `json:"oclc_field,omitempty"`
	PIDField       string   `json:"pid_field,omitempty"`
	PoolField      string   `json:"pool_field,omitempty"`
	ProviderField  string   `json:"provider_field,omitempty"`
	SubtitleField  string   `json:"subtitle_field,omitempty"`
	ThumbnailField string   `json:"thumbnail_field,omitempty"`
	TitleField     string   `json:"title_field,omitempty"`
	UPCField       string   `json:"upc_field,omitempty"`
	URLField       string   `json:"url_field,omitempty"`
	DefaultItemXID string   `json:"default_item_xid,omitempty"` // translation ids
	ValueXID       string   `json:"value_xid,omitempty"`
	IDPrefix       string   `json:"id_prefix,omitempty"` // misc
	MusicPool      string   `json:"music_pool,omitempty"`
	ProxyPrefix    string   `json:"proxy_prefix,omitempty"`
	ProxyDomains   []string `json:"proxy_domains,omitempty"`
	MaxSupported   int      `json:"max_supported,omitempty"`
}

type poolConfigFieldCustomInfo struct {
	Abstract                  *poolConfigFieldTypeCustom `json:"abstract,omitempty"`
	AccessURL                 *poolConfigFieldTypeCustom `json:"access_url,omitempty"`
	CoverImageURL             *poolConfigFieldTypeCustom `json:"cover_image_url,omitempty"`
	DigitalContentURL         *poolConfigFieldTypeCustom `json:"digital_content_url,omitempty"`
	PdfDownloadURL            *poolConfigFieldTypeCustom `json:"pdf_download_url,omitempty"`
	PublisherName             *poolConfigFieldTypeCustom `json:"publisher_name,omitempty"`
	RISType                   *poolConfigFieldTypeCustom `json:"ris_type,omitempty"`
	SirsiURL                  *poolConfigFieldTypeCustom `json:"sirsi_url,omitempty"`
	ThumbnailURL              *poolConfigFieldTypeCustom `json:"thumbnail_url,omitempty"`
	TitleSubtitleEdition      *poolConfigFieldTypeCustom `json:"title_subtitle_edition,omitempty"`
	WSLSCollectionDescription *poolConfigFieldTypeCustom `json:"wsls_collection_description,omitempty"`
}

type poolConfigField struct {
	Name               string                     `json:"name,omitempty"` // required; v4 field name, and key for common fields
	XID                string                     `json:"xid,omitempty"`
	WSLSXID            string                     `json:"wsls_xid,omitempty"` // for wsls fields with alternate labels
	Field              string                     `json:"field,omitempty"`
	Properties         poolConfigFieldProperties  `json:"properties,omitempty"`
	RISCodes           []string                   `json:"ris_codes,omitempty"`
	Limit              int                        `json:"limit,omitempty"`
	SplitOn            string                     `json:"split_on,omitempty"`
	OnShelfOnly        bool                       `json:"onshelf_only,omitempty"`
	DigitalContentOnly bool                       `json:"digital_content_only,omitempty"`
	Custom             bool                       `json:"custom,omitempty"`      // if true, the Name drives custom handling
	CustomInfo         *poolConfigFieldCustomInfo `json:"custom_info,omitempty"` // extra info for certain custom formats
}

type poolConfigAvailabilityFields struct {
	Field string `json:"field,omitempty"`
	Facet string `json:"facet,omitempty"`
}

type poolConfigAvailabilityValues struct {
	OnShelf []string `json:"onshelf,omitempty"`
	Online  []string `json:"online,omitempty"`
	Other   []string `json:"other,omitempty"`
}

type poolConfigAvailability struct {
	Anon          poolConfigAvailabilityFields `json:"anon,omitempty"`
	Auth          poolConfigAvailabilityFields `json:"auth,omitempty"`
	Values        poolConfigAvailabilityValues `json:"values,omitempty"`
	ExposedValues []string                     `json:"-"` // derived from above values
}

type poolConfigFacetSolr struct {
	Field     string `json:"field,omitempty"`
	FieldAuth string `json:"field_auth,omitempty"`
	Value     string `json:"value,omitempty"`
	Type      string `json:"type,omitempty"`
	Sort      string `json:"sort,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

type poolConfigFacetQuery struct {
	XID   string `json:"xid,omitempty"`
	Query string `json:"query,omitempty"`
}

type poolConfigFacet struct {
	XID                string                 `json:"xid,omitempty"` // translation ID
	Solr               poolConfigFacetSolr    `json:"solr,omitempty"`
	Type               string                 `json:"type,omitempty"`
	Format             string                 `json:"format,omitempty"`
	ExposedValues      []string               `json:"exposed_values,omitempty"`
	DependentFacetXIDs []string               `json:"dependent_facet_xids,omitempty"`
	ComponentQueries   []poolConfigFacetQuery `json:"component_queries,omitempty"`
	IsAvailability     bool                   `json:"is_availability,omitempty"`
	BucketSort         string                 `json:"bucket_sort,omitempty"`
	Index              int                    `json:"-"`
	queryMap           map[string]*poolConfigFacetQuery
}

type poolConfigSort struct {
	XID         string `json:"xid,omitempty"` // translation ID
	Field       string `json:"field,omitempty"`
	Order       string `json:"order,omitempty"`
	RecordXID   string `json:"record_xid,omitempty"`
	RecordOrder string `json:"record_order,omitempty"`
}

type poolConfigIdentity struct {
	NameXID    string   `json:"name_xid,omitempty"`   // translation ID
	DescXID    string   `json:"desc_xid,omitempty"`   // translation ID
	Mode       string   `json:"mode,omitempty"`       // pool mode (what it is, e.g. "record" (default), "image", etc.)
	Attributes []string `json:"attributes,omitempty"` // pool attributes (what it supports)
	RISType    string   `json:"ris_type,omitempty"`
}

type poolConfigProvider struct {
	Name string `json:"name,omitempty"`
	XID  string `json:"xid,omitempty"` // translation ID
	URL  string `json:"url,omitempty"`
	Logo string `json:"logo,omitempty"`
}

type poolConfigRelatedImage struct {
	IDField           string `json:"id_field,omitempty"`
	IIIFManifestField string `json:"iiif_manifest_field,omitempty"`
	IIIFImageField    string `json:"iiif_image_field,omitempty"`
}

type poolConfigRelated struct {
	Image *poolConfigRelatedImage `json:"image,omitempty"`
}

type poolConfigMappingsDefinitions struct {
	Fields []poolConfigField `json:"fields,omitempty"`
	Facets []poolConfigFacet `json:"facets,omitempty"`
	Sorts  []poolConfigSort  `json:"sorts,omitempty"`
}

type poolConfigMappingsHeadingField struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	RISCode string `json:"ris_code,omitempty"`
}

type poolConfigMappingsConfiguredFields struct {
	Title            poolConfigMappingsHeadingField `json:"title,omitempty"`
	TitleVernacular  poolConfigMappingsHeadingField `json:"title_vernacular,omitempty"`
	Author           poolConfigMappingsHeadingField `json:"author,omitempty"`
	AuthorVernacular poolConfigMappingsHeadingField `json:"author_vernacular,omitempty"`
	Basic            []string                       `json:"basic,omitempty"`
	Detailed         []string                       `json:"detailed,omitempty"`
}

type poolConfigMappingsConfigured struct {
	FieldNames poolConfigMappingsConfiguredFields `json:"field_names,omitempty"`
	FacetXIDs  []string                           `json:"facet_xids,omitempty"`
	SortXIDs   []string                           `json:"sort_xids,omitempty"`
}

type poolConfigMappings struct {
	Definitions poolConfigMappingsDefinitions `json:"definitions,omitempty"`
	Configured  poolConfigMappingsConfigured  `json:"configured,omitempty"`
}

type poolConfigRISType struct {
	Type    string `json:"type,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	re      *regexp.Regexp
}

type poolConfigRecordAttribute struct {
	Field    string   `json:"field,omitempty"`
	Contains []string `json:"contains,omitempty"`
}

type poolConfigRecordAttributes struct {
	DigitalContent poolConfigRecordAttribute `json:"digital_content,omitempty"`
	Sirsi          poolConfigRecordAttribute `json:"sirsi,omitempty"`
	WSLS           poolConfigRecordAttribute `json:"wsls,omitempty"`
}

type poolConfigGlobal struct {
	Service          poolConfigService          `json:"service,omitempty"`
	Attributes       []string                   `json:"attributes,omitempty"`
	Providers        []poolConfigProvider       `json:"providers,omitempty"`
	Availability     poolConfigAvailability     `json:"availability,omitempty"`
	RISTypes         []poolConfigRISType        `json:"ris_types,omitempty"`
	RecordAttributes poolConfigRecordAttributes `json:"record_attributes,omitempty"`
	Publishers       []poolConfigPublisher      `json:"publishers,omitempty"`
	Relators         poolConfigRelators         `json:"relators,omitempty"`
	Copyrights       []poolConfigCopyright      `json:"copyrights,omitempty"`
	Mappings         poolConfigMappings         `json:"mappings,omitempty"`
}

type poolConfigLocal struct {
	Identity poolConfigIdentity `json:"identity,omitempty"`
	Solr     poolConfigSolr     `json:"solr,omitempty"`
	Mappings poolConfigMappings `json:"mappings,omitempty"`
	Related  *poolConfigRelated `json:"related,omitempty"`
}

type poolConfig struct {
	Global poolConfigGlobal `json:"global,omitempty"`
	Local  poolConfigLocal  `json:"local,omitempty"`
}

func getSortedJSONEnvVars() []string {
	var keys []string

	for _, keyval := range os.Environ() {
		key := strings.Split(keyval, "=")[0]
		if strings.HasPrefix(key, envPrefix+"_JSON_") {
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)

	return keys
}

func loadConfig() *poolConfig {
	cfg := poolConfig{}

	// contents are either plain text json, or base64-encoded gzipped json

	envs := getSortedJSONEnvVars()

	valid := true

	for _, env := range envs {
		if val := os.Getenv(env); val != "" {
			input := bytes.NewReader([]byte(val))

			// attempt to read as plain text first

			jsDec := json.NewDecoder(input)
			jsDec.DisallowUnknownFields()

			if jsErr := jsDec.Decode(&cfg); jsErr == nil {
				log.Printf("[CONFIG] loaded %s (plain text; %d bytes)", env, len(val))
				continue
			}

			// fall back to decoding as base64-encoded gzipped data

			input = bytes.NewReader([]byte(val))

			b64Dec := base64.NewDecoder(base64.StdEncoding, input)

			gzDec, gzErr := gzip.NewReader(b64Dec)
			if gzErr != nil {
				log.Printf("[CONFIG] %s: error decoding gzipped data: %s", env, gzErr.Error())
				valid = false
				continue
			}

			jsDec = json.NewDecoder(gzDec)
			jsDec.DisallowUnknownFields()

			if jsErr := jsDec.Decode(&cfg); jsErr != nil {
				log.Printf("[CONFIG] %s: error decoding json data: %s", env, jsErr.Error())
				valid = false
				continue
			}

			log.Printf("[CONFIG] loaded %s (base64-encoded gzipped data; %d bytes)", env, len(val))
		}
	}

	if valid == false {
		log.Printf("exiting due to json decode error(s) above")
		os.Exit(1)
	}

	// optional convenience overrides to simplify terraform config
	if host := os.Getenv(envPrefix + "_SOLR_HOST"); host != "" {
		cfg.Local.Solr.Host = host
	}

	if host := os.Getenv(envPrefix + "_DCON_HOST"); host != "" {
		cfg.Global.Service.URLTemplates.DigitalContent.Host = host
	}

	// log accumulated config
	bytes, err := json.Marshal(cfg)
	if err != nil {
		log.Printf("error encoding config json: %s", err.Error())
		os.Exit(1)
	}

	log.Printf("[CONFIG] composite json:\n%s", string(bytes))

	return &cfg
}
