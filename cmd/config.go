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

type poolConfigMain struct {
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
	XID           string `json:"xid,omitempty"`
	Field         string `json:"field,omitempty"`
	Name          string `json:"name,omitempty"`
	Type          string `json:"type,omitempty"`
	Display       string `json:"display,omitempty"`
	Visibility    string `json:"visibility,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	OnShelfOnly   bool   `json:"onshelf_only,omitempty"`
	DetailsOnly   bool   `json:"details_only,omitempty"`
	Provider      string `json:"provider,omitempty"`
	URLField      string `json:"url_field,omitempty"`
	LabelField    string `json:"label_field,omitempty"`
	ProviderField string `json:"provider_field,omitempty"`
}

type poolConfigAvailability struct {
	Field         string   `json:"field,omitempty"`
	FieldAuth     string   `json:"field_auth,omitempty"` // used instead if defined, and IsUVA
	Facet         string   `json:"facet,omitempty"`
	FacetAuth     string   `json:"facet_auth,omitempty"` // used instead if defined, and IsUVA
	ValuesOnShelf []string `json:"values_onshelf,omitempty"`
	ValuesOnline  []string `json:"values_online,omitempty"`
	ValuesOther   []string `json:"values_other,omitempty"`
	ExposedValues []string // derived from above values
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

type poolConfig struct {
	Identity     poolConfigIdentity     `json:"identity,omitempty"`
	Main         poolConfigMain         `json:"main,omitempty"`
	Solr         poolConfigSolr         `json:"solr,omitempty"`
	Providers    []poolConfigProvider   `json:"providers,omitempty"`
	Fields       []poolConfigField      `json:"fields,omitempty"`
	Facets       []poolConfigFacet      `json:"facets,omitempty"`
	Availability poolConfigAvailability `json:"availability,omitempty"`
}

func loadConfig() *poolConfig {
	cfg := poolConfig{}

	// main config

	if err := json.Unmarshal([]byte(os.Getenv("VIRGO4_SOLR_POOL_WS_CONFIG")), &cfg); err != nil {
		log.Printf("error parsing pool config json: %s", err.Error())
		os.Exit(1)
	}

	// overrides
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
