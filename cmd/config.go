package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"sort"
	"strings"
)

const envPrefix = "VIRGO4_SOLR_POOL_WS"

type poolConfigURLTemplate struct {
	Pattern  string   `json:"pattern,omitempty"`
	Template string   `json:"template,omitempty"`
	Fallback string   `json:"fallback,omitempty"`
	Prefixes []string `json:"prefixes,omitempty"`
}

type poolConfigURLTemplates struct {
	Sirsi          poolConfigURLTemplate `json:"sirsi,omitempty"`
	CoverImages    poolConfigURLTemplate `json:"cover_images,omitempty"`
	IIIF           poolConfigURLTemplate `json:"iiif,omitempty"`
	DigitalContent poolConfigURLTemplate `json:"digital_content,omitempty"`
}

type poolConfigDigitalContent struct {
	FeatureField string   `json:"feature_field,omitempty"`
	Features     []string `json:"features,omitempty"`
}

type poolConfigService struct {
	Port           string                   `json:"port,omitempty"`
	JWTKey         string                   `json:"jwt_key,omitempty"`
	DefaultSort    poolConfigSort           `json:"default_sort,omitempty"`
	URLTemplates   poolConfigURLTemplates   `json:"url_templates,omitempty"`
	DigitalContent poolConfigDigitalContent `json:"digital_content,omitempty"`
	Pdf            poolConfigPdf            `json:"pdf,omitempty"`
}

type poolConfigSolrParams struct {
	Qt      string   `json:"qt,omitempty"`
	DefType string   `json:"deftype,omitempty"`
	Fq      []string `json:"fq,omitempty"` // pool definition should go here
	Fl      []string `json:"fl,omitempty"`
}

type poolConfigSort struct {
	XID   string `json:"xid,omitempty"`
	Order string `json:"order,omitempty"`
}

type poolConfigSolrGrouping struct {
	Field string         `json:"field,omitempty"`
	Sort  poolConfigSort `json:"sort,omitempty"`
}

type poolConfigSolr struct {
	Host                 string                 `json:"host,omitempty"`
	Core                 string                 `json:"core,omitempty"`
	Handler              string                 `json:"handler,omitempty"`
	ConnTimeout          string                 `json:"conn_timeout,omitempty"`
	ReadTimeout          string                 `json:"read_timeout,omitempty"`
	ScoreThresholdMedium float32                `json:"score_threshold_medium,omitempty"`
	ScoreThresholdHigh   float32                `json:"score_threshold_high,omitempty"`
	Params               poolConfigSolrParams   `json:"params,omitempty"`
	Grouping             poolConfigSolrGrouping `json:"grouping,omitempty"`
	ExactMatchTitleField string                 `json:"exact_match_title_field,omitempty"`
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
	Display    string `json:"display,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Provider   string `json:"provider,omitempty"`
	RISCode    string `json:"ris_code,omitempty"` // can override mapping
}

type poolConfigFieldTypeAccessURL struct {
	URLField       string `json:"url_field,omitempty"`
	LabelField     string `json:"label_field,omitempty"`
	ProviderField  string `json:"provider_field,omitempty"`
	DefaultItemXID string `json:"default_item_xid,omitempty"`
}

type poolConfigFieldTypeIIIFBaseURL struct {
	IdentifierField string `json:"identifier_field,omitempty"`
}

type poolConfigFieldTypeCoverImageURL struct {
	ThumbnailField string `json:"thumbnail_field,omitempty"`
	IDField        string `json:"id_field,omitempty"`
	TitleField     string `json:"title_field,omitempty"`
	PoolField      string `json:"pool_field,omitempty"`
	ISBNField      string `json:"isbn_field,omitempty"`
	OCLCField      string `json:"oclc_field,omitempty"`
	LCCNField      string `json:"lccn_field,omitempty"`
	UPCField       string `json:"upc_field,omitempty"`
	MusicPool      string `json:"music_pool,omitempty"`
}

type poolConfigFieldTypeSirsiURL struct {
	IDField  string `json:"id_field,omitempty"`
	IDPrefix string `json:"id_prefix,omitempty"`
}

type poolConfigFieldTypeDigitalContentURL struct {
	IDField string `json:"id_field,omitempty"`
}

type poolConfigFieldTypePdfDownloadURL struct {
	URLField     string `json:"url_field,omitempty"`
	PIDField     string `json:"pid_field,omitempty"`
	MaxSupported int    `json:"max_supported,omitempty"`
}

type poolConfigFieldTypeThumbnailURL struct {
	URLField     string `json:"url_field,omitempty"`
	MaxSupported int    `json:"max_supported,omitempty"`
}

type poolConfigFieldCustomInfo struct {
	AccessURL         *poolConfigFieldTypeAccessURL         `json:"access_url,omitempty"`
	IIIFBaseURL       *poolConfigFieldTypeIIIFBaseURL       `json:"iiif_base_url,omitempty"`
	CoverImageURL     *poolConfigFieldTypeCoverImageURL     `json:"cover_image_url,omitempty"`
	SirsiURL          *poolConfigFieldTypeSirsiURL          `json:"sirsi_url,omitempty"`
	DigitalContentURL *poolConfigFieldTypeDigitalContentURL `json:"digital_content_url,omitempty"`
	PdfDownloadURL    *poolConfigFieldTypePdfDownloadURL    `json:"pdf_download_url,omitempty"`
	ThumbnailURL      *poolConfigFieldTypeThumbnailURL      `json:"thumbnail_url,omitempty"`
}

type poolConfigField struct {
	Name               string                     `json:"name,omitempty"` // required; v4 field name, and key for common fields
	XID                string                     `json:"xid,omitempty"`
	Field              string                     `json:"field,omitempty"`
	Properties         poolConfigFieldProperties  `json:"properties,omitempty"`
	Limit              int                        `json:"limit,omitempty"`
	OnShelfOnly        bool                       `json:"onshelf_only,omitempty"`
	DetailsOnly        bool                       `json:"details_only,omitempty"`
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

type poolConfigFacet struct {
	XID                string              `json:"xid,omitempty"` // translation ID
	Solr               poolConfigFacetSolr `json:"solr,omitempty"`
	Type               string              `json:"type,omitempty"`
	Format             string              `json:"format,omitempty"`
	ExposedValues      []string            `json:"exposed_values,omitempty"`
	DependentFacetXIDs []string            `json:"dependent_facets,omitempty"`
	IsAvailability     bool                `json:"is_availability,omitempty"`
}

type poolConfigSortOptions struct {
	XID   string `json:"xid,omitempty"` // translation ID
	Field string `json:"field,omitempty"`
}

type poolConfigIdentity struct {
	NameXID     string                  `json:"name_xid,omitempty"`     // translation ID
	DescXID     string                  `json:"desc_xid,omitempty"`     // translation ID
	Mode        string                  `json:"mode,omitempty"`         // pool mode (what it is, e.g. "record" (default), "image", etc.)
	Attributes  VirgoPoolAttributes     `json:"attributes,omitempty"`   // pool attributes (what it supports)
	SortOptions []poolConfigSortOptions `json:"sort_options,omitempty"` // available sort options
}

type poolConfigProvider struct {
	Name string `json:"name,omitempty"`
	XID  string `json:"xid,omitempty"` // translation ID
	URL  string `json:"url,omitempty"`
	Logo string `json:"logo,omitempty"`
}

type poolConfigRelatedImage struct {
	IDField           string `json:"id_field,omitempty"`
	IdentifierField   string `json:"identifier_field,omitempty"`
	IIIFManifestField string `json:"iiif_manifest_field,omitempty"`
	IIIFImageField    string `json:"iiif_image_field,omitempty"`
}

type poolConfigRelated struct {
	Image *poolConfigRelatedImage `json:"image,omitempty"`
}

type poolConfigRISCode struct {
	Field string `json:"field,omitempty"`
	Code  string `json:"code,omitempty"`
}

type poolConfigMappings struct {
	Fields     []poolConfigField `json:"fields,omitempty"`
	FieldNames []string          `json:"field_names,omitempty"`
	Facets     []poolConfigFacet `json:"facets,omitempty"`
	FacetXIDs  []string          `json:"facet_xids,omitempty"`
}

type poolConfigGlobal struct {
	Service      poolConfigService      `json:"service,omitempty"`
	Providers    []poolConfigProvider   `json:"providers,omitempty"`
	Availability poolConfigAvailability `json:"availability,omitempty"`
	Mappings     poolConfigMappings     `json:"mappings,omitempty"`
	RISCodes     []poolConfigRISCode    `json:"ris_codes,omitempty"`
}

type poolConfigLocal struct {
	Identity poolConfigIdentity `json:"identity,omitempty"`
	Solr     poolConfigSolr     `json:"solr,omitempty"`
	Mappings poolConfigMappings `json:"mappings,omitempty"`
	Related  *poolConfigRelated `json:"related,omitempty"`
}

type poolConfig struct {
	Global   poolConfigGlobal   `json:"global,omitempty"`
	Local    poolConfigLocal    `json:"local,omitempty"`
	Mappings poolConfigMappings `json:"-"` // built from global/local mappings
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

	// json configs

	envs := getSortedJSONEnvVars()

	valid := true

	for _, env := range envs {
		log.Printf("[CONFIG] loading %s ...", env)
		if val := os.Getenv(env); val != "" {
			dec := json.NewDecoder(bytes.NewReader([]byte(val)))
			dec.DisallowUnknownFields()

			if err := dec.Decode(&cfg); err != nil {
				log.Printf("error decoding %s: %s", env, err.Error())
				valid = false
			}
		}
	}

	if valid == false {
		log.Printf("exiting due to json decode error(s) above")
		os.Exit(1)
	}

	// optional convenience override to simplify terraform config
	if host := os.Getenv(envPrefix + "_SOLR_HOST"); host != "" {
		cfg.Local.Solr.Host = host
	}

	//bytes, err := json.MarshalIndent(cfg, "", "  ")
	bytes, err := json.Marshal(cfg)
	if err != nil {
		log.Printf("error encoding config json: %s", err.Error())
		os.Exit(1)
	}

	log.Printf("[CONFIG] composite json:\n%s", string(bytes))

	return &cfg
}
