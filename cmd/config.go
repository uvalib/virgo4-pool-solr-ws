package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"sort"
	"strings"
)

type poolConfigURLTemplate struct {
	Pattern  string `json:"pattern,omitempty"`
	Template string `json:"template,omitempty"`
}

type poolConfigURLTemplates struct {
	Sirsi       poolConfigURLTemplate `json:"sirsi,omitempty"`
	CoverImages poolConfigURLTemplate `json:"cover_images,omitempty"`
}

type poolConfigService struct {
	Port         string                 `json:"port,omitempty"`
	JWTKey       string                 `json:"jwt_key,omitempty"`
	URLTemplates poolConfigURLTemplates `json:"url_templates,omitempty"`
}

type poolConfigSolrParams struct {
	Qt      string   `json:"qt,omitempty"`
	DefType string   `json:"deftype,omitempty"`
	Fq      []string `json:"fq,omitempty"` // pool definition should go here
	Fl      []string `json:"fl,omitempty"`
}

type poolConfigSolrGrouping struct {
	Field     string `json:"field,omitempty"`
	SortXID   string `json:"sort_xid,omitempty"`
	SortOrder string `json:"sort_order,omitempty"`
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

type poolConfigFieldProperties struct {
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	Display    string `json:"display,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

type poolConfigFieldTypeAccessURL struct {
	URLField       string `json:"url_field,omitempty"`
	LabelField     string `json:"label_field,omitempty"`
	ProviderField  string `json:"provider_field,omitempty"`
	DefaultItemXID string `json:"default_item_xid,omitempty"`
}

type poolConfigFieldTypeIIIFBaseURL struct {
	IdentifierField string `json:"identifier_field,omitempty"`
	BaseURL         string `json:"base_url,omitempty"`
	FallbackID      string `json:"fallback_id,omitempty"`
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
}

type poolConfigFieldTypeSirsiURL struct {
	IDField  string `json:"id_field,omitempty"`
	IDPrefix string `json:"id_prefix,omitempty"`
}

type poolConfigField struct {
	XID           string                            `json:"xid,omitempty"`
	Field         string                            `json:"field,omitempty"`
	Properties    poolConfigFieldProperties         `json:"properties,omitempty"`
	Limit         int                               `json:"limit,omitempty"`
	OnShelfOnly   bool                              `json:"onshelf_only,omitempty"`
	DetailsOnly   bool                              `json:"details_only,omitempty"`
	Format        string                            `json:"format,omitempty"` // controlled vocabulary; drives special handling
	AccessURL     *poolConfigFieldTypeAccessURL     `json:"access_url,omitempty"`
	IIIFBaseURL   *poolConfigFieldTypeIIIFBaseURL   `json:"iiif_base_url,omitempty"`
	CoverImageURL *poolConfigFieldTypeCoverImageURL `json:"cover_image_url,omitempty"`
	SirsiURL      *poolConfigFieldTypeSirsiURL      `json:"sirsi_url,omitempty"`
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
	Type               string              `json:"type,omitempty"` // "checkbox" implies a filter value will be applied if checked, otherwise nothing
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

type poolConfig struct {
	Identity     poolConfigIdentity     `json:"identity,omitempty"`
	Service      poolConfigService      `json:"service,omitempty"`
	Solr         poolConfigSolr         `json:"solr,omitempty"`
	Providers    []poolConfigProvider   `json:"providers,omitempty"`
	Fields       []poolConfigField      `json:"fields,omitempty"`
	GlobalFacets []poolConfigFacet      `json:"global_facets,omitempty"`
	LocalFacets  []poolConfigFacet      `json:"local_facets,omitempty"`
	Availability poolConfigAvailability `json:"availability,omitempty"`
	Related      poolConfigRelated      `json:"related,omitempty"`
	Facets       []poolConfigFacet      `json:"-"` // global + local, for convenience
}

func getSortedJSONEnvVars() []string {
	var keys []string

	for _, keyval := range os.Environ() {
		key := strings.Split(keyval, "=")[0]
		if strings.HasPrefix(key, "VIRGO4_SOLR_POOL_WS_JSON_") {
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
	if host := os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_HOST"); host != "" {
		cfg.Solr.Host = host
	}

	//bytes, err := json.MarshalIndent(cfg, "", "  ")
	bytes, err := json.Marshal(cfg)
	if err != nil {
		log.Printf("error encoding pool config json: %s", err.Error())
		os.Exit(1)
	}

	log.Printf("[CONFIG] composite json:")
	log.Printf("\n%s", string(bytes))

	cfg.Facets = append(cfg.GlobalFacets, cfg.LocalFacets...)

	return &cfg
}
