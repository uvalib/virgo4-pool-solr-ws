package main

import (
	"encoding/json"
	"log"
	"os"
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
}

/*
type poolConfigFieldProperties struct {
	XID        string `json:"xid,omitempty"`
	Field      string `json:"field,omitempty"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	Display    string `json:"display,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

type poolConfigFieldTypeAccessURL struct {
	URLField      string `json:"url_field,omitempty"`
	LabelField    string `json:"label_field,omitempty"`
	ProviderField string `json:"provider_field,omitempty"`
}

type poolConfigFieldTypeIIIF struct {
	IdentifierField string `json:"identifier_field,omitempty"`
}

type poolConfigField struct {
	Properties  poolConfigFieldProperties      `json:"properties,omitempty"`
	Type        string                         `json:"type,omitempty"` // controlled vocabulary; drives special handling
	Limit       int                            `json:"limit,omitempty"`
	OnShelfOnly bool                           `json:"onshelf_only,omitempty"`
	DetailsOnly bool                           `json:"details_only,omitempty"`
	// config options for specific types
	AccessURL   poolConfigFieldTypeAccessURL   `json:"access_url,omitempty"`    // if type == "access_url"
	IIIFBaseURL poolConfigFieldTypeIIIFBaseURL `json:"iiif_base_url,omitempty"` // if type == "iiif_base_url"
}
*/

type poolConfigField struct {
	XID             string `json:"xid,omitempty"`
	Field           string `json:"field,omitempty"`
	Name            string `json:"name,omitempty"`
	Type            string `json:"type,omitempty"`
	Display         string `json:"display,omitempty"`
	Visibility      string `json:"visibility,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	OnShelfOnly     bool   `json:"onshelf_only,omitempty"`
	DetailsOnly     bool   `json:"details_only,omitempty"`
	Provider        string `json:"provider,omitempty"`
	URLField        string `json:"url_field,omitempty"`
	LabelField      string `json:"label_field,omitempty"`
	ProviderField   string `json:"provider_field,omitempty"`
	IdentifierField string `json:"identifier_field,omitempty"`
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
	ExposedValues []string                     // derived from above values
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
	Image poolConfigRelatedImage `json:"image,omitempty"`
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
	Facets       []poolConfigFacet      // global + local, for convenience
}

func loadConfig() *poolConfig {
	cfg := poolConfig{}

	// json configs

	// load IDENTITY last as it will contain pool-specific data, and can
	// override any of the static data within the other config vars
	envs := []string{
		"VIRGO4_SOLR_POOL_WS_JSON_AVAILABILITY",
		"VIRGO4_SOLR_POOL_WS_JSON_PROVIDERS",
		"VIRGO4_SOLR_POOL_WS_JSON_SERVICE",
		"VIRGO4_SOLR_POOL_WS_JSON_FACETS",
		"VIRGO4_SOLR_POOL_WS_JSON_IDENTITY",
	}

	for _, env := range envs {
		if val := os.Getenv(env); val != "" {
			if err := json.Unmarshal([]byte(val), &cfg); err != nil {
				log.Printf("error parsing %s config json: %s", env, err.Error())
				os.Exit(1)
			}
		}
	}

	// optional convenience override to simplify terraform config
	if host := os.Getenv("VIRGO4_SOLR_POOL_WS_SOLR_HOST"); host != "" {
		cfg.Solr.Host = host
	}

	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("error regenerating pool config json: %s", err.Error())
		os.Exit(1)
	}

	log.Printf("[CONFIG] json:")
	log.Printf("\n%s", string(bytes))

	cfg.Facets = append(cfg.GlobalFacets, cfg.LocalFacets...)

	return &cfg
}
