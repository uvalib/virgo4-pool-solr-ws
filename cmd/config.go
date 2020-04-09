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

type poolConfigSolr struct {
	Host                 string               `json:"host,omitempty"`
	Core                 string               `json:"core,omitempty"`
	Handler              string               `json:"handler,omitempty"`
	ConnTimeout          string               `json:"conn_timeout,omitempty"`
	ReadTimeout          string               `json:"read_timeout,omitempty"`
	GroupField           string               `json:"group_field,omitempty"`
	ScoreThresholdMedium float32              `json:"score_threshold_medium,omitempty"`
	ScoreThresholdHigh   float32              `json:"score_threshold_high,omitempty"`
	Params               poolConfigSolrParams `json:"params,omitempty"`
}

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

type poolConfigFacet struct {
	XID                string   `json:"xid,omitempty"` // translation ID
	Field              string   `json:"field,omitempty"`
	FieldAuth          string   `json:"field_auth,omitempty"`
	Type               string   `json:"type,omitempty"`
	Sort               string   `json:"sort,omitempty"`
	Limit              int      `json:"limit,omitempty"`
	Offset             int      `json:"offset,omitempty"`
	ExposedValues      []string `json:"exposed_values,omitempty"`
	DependentFacetXIDs []string `json:"dependent_facets,omitempty"`
	IsAvailability     bool     `json:"is_availability,omitempty"`
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
	Facets       []poolConfigFacet      `json:"facets,omitempty"`
	Availability poolConfigAvailability `json:"availability,omitempty"`
	Related      poolConfigRelated      `json:"related,omitempty"`
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

	return &cfg
}
