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
	ShelfBrowse    poolConfigURLTemplate `json:"shelf_browse,omitempty"`
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
	Code  string   `json:"code,omitempty"`
	Terms []string `json:"terms,omitempty"`
}

type poolConfigRelators struct {
	AuthorCodes     []string            `json:"author_codes,omitempty"`
	AdvisorCodes    []string            `json:"advisor_codes,omitempty"`
	EditorCodes     []string            `json:"editor_codes,omitempty"`
	CompilerCodes   []string            `json:"compiler_codes,omitempty"`
	TranslatorCodes []string            `json:"translator_codes,omitempty"`
	Map             []poolConfigRelator `json:"map,omitempty"`
}

type poolConfigTitleizationCharacterSets struct {
	WordDelimiters string `json:"word_delimiters,omitempty"`
	PartDelimiters string `json:"part_delimiters,omitempty"`
}

type poolConfigTitleizationWordLists struct {
	MixedCaseWords  []string `json:"mixed_case_words,omitempty"`
	UpperCaseWords  []string `json:"upper_case_words,omitempty"`
	LowerCaseWords  []string `json:"lower_case_words,omitempty"`
	MultiPartWords  []string `json:"multi_part_words,omitempty"`
	OrdinalPatterns []string `json:"ordinal_patterns,omitempty"`
}

type poolConfigTitleization struct {
	CharacterSets poolConfigTitleizationCharacterSets `json:"character_sets,omitempty"`
	WordLists     poolConfigTitleizationWordLists     `json:"word_lists,omitempty"`
	Exclusions    []poolConfigFieldComparison         `json:"exclusions,omitempty"`
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

type poolConfigHTTPClient struct {
	Enabled     bool   `json:"enabled"` // only used for serials solutions client
	URL         string `json:"url,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	ConnTimeout string `json:"conn_timeout,omitempty"`
	ReadTimeout string `json:"read_timeout,omitempty"`
}

type poolConfigService struct {
	Port             string                 `json:"port,omitempty"`
	JWTKey           string                 `json:"jwt_key,omitempty"`
	DefaultSort      poolConfigSort         `json:"default_sort,omitempty"`
	URLTemplates     poolConfigURLTemplates `json:"url_templates,omitempty"`
	SerialsSolutions poolConfigHTTPClient   `json:"serials_solutions,omitempty"`
}

type poolConfigSolrParams struct {
	Qt       string   `json:"qt,omitempty"`
	DefType  string   `json:"deftype,omitempty"`
	GlobalFq []string `json:"global_fq,omitempty"` // global filter queries should go here
	PoolFq   []string `json:"pool_fq,omitempty"`   // pool definition should go here
	Fl       []string `json:"fl,omitempty"`
}

type poolConfigSolrHighlighting struct {
	Method            string   `json:"method,omitempty"`
	Fl                []string `json:"fl,omitempty"`
	Snippets          string   `json:"snippets,omitempty"`
	Fragsize          string   `json:"fragsize,omitempty"`
	FragsizeIsMinimum string   `json:"fragsizeIsMinimum,omitempty"`
	FragAlignRatio    string   `json:"fragAlignRatio,omitempty"`
	MaxAnalyzedChars  string   `json:"maxAnalyzedChars,omitempty"`
	MultiTermQuery    string   `json:"multiTermQuery,omitempty"`
	TagPre            string   `json:"tag.pre,omitempty"`
	TagPost           string   `json:"tag.post,omitempty"`
}

type poolConfigSolrClients struct {
	Service     poolConfigHTTPClient `json:"service,omitempty"`
	HealthCheck poolConfigHTTPClient `json:"healthcheck,omitempty"`
}

type poolConfigAuthorFields struct {
	Preferred []string `json:"preferred,omitempty"`
	Fallback  []string `json:"fallback,omitempty"`
}

type poolConfigSolr struct {
	Host                    string                     `json:"host,omitempty"`
	Core                    string                     `json:"core,omitempty"`
	Clients                 poolConfigSolrClients      `json:"clients,omitempty"`
	Params                  poolConfigSolrParams       `json:"params,omitempty"`
	Highlighting            poolConfigSolrHighlighting `json:"highlighting,omitempty"`
	IdentifierField         string                     `json:"identifier_field,omitempty"`
	GroupField              string                     `json:"group_field,omitempty"`
	RelevanceIntraGroupSort poolConfigSort             `json:"relevance_intra_group_sort,omitempty"`
	ExactMatchTitleField    string                     `json:"exact_match_title_field,omitempty"`
	ScoreThresholdMedium    float32                    `json:"score_threshold_medium,omitempty"`
	ScoreThresholdHigh      float32                    `json:"score_threshold_high,omitempty"`
}

type poolConfigFieldProperties struct {
	Type          string `json:"type,omitempty"`
	Separator     string `json:"separator,omitempty"`
	Display       string `json:"display,omitempty"`
	Visibility    string `json:"visibility,omitempty"`
	Provider      string `json:"provider,omitempty"`
	CitationPart  string `json:"citation_part,omitempty"`
	SearchDisplay string `json:"search_display,omitempty"`
}

type poolConfigFieldComparison struct {
	Field    string     `json:"field,omitempty"`
	Contains [][]string `json:"contains,omitempty"`
	Matches  [][]string `json:"matches,omitempty"`
	Value    string     `json:"value,omitempty"`
}

type poolConfigFieldCustomConfig struct {
	AlternateField   string                      `json:"alternate_field,omitempty"` // field names
	CallNumberField  string                      `json:"call_number_field,omitempty"`
	EditionField     string                      `json:"edition_field,omitempty"`
	FormatField      string                      `json:"format_field,omitempty"`
	ISBNField        string                      `json:"isbn_field,omitempty"`
	ISSNField        string                      `json:"issn_field,omitempty"`
	LCCNField        string                      `json:"lccn_field,omitempty"`
	LabelField       string                      `json:"label_field,omitempty"`
	OCLCField        string                      `json:"oclc_field,omitempty"`
	PoolField        string                      `json:"pool_field,omitempty"`
	ProviderField    string                      `json:"provider_field,omitempty"`
	SubtitleField    string                      `json:"subtitle_field,omitempty"`
	TitleField       string                      `json:"title_field,omitempty"`
	UPCField         string                      `json:"upc_field,omitempty"`
	URLField         string                      `json:"url_field,omitempty"`
	AlternateXID     string                      `json:"alternate_xid,omitempty"` // translation ids
	DefaultItemXID   string                      `json:"default_item_xid,omitempty"`
	ValueXID         string                      `json:"value_xid,omitempty"`
	AlternateType    string                      `json:"alternate_type,omitempty"` // misc
	IDPrefix         string                      `json:"id_prefix,omitempty"`
	MusicPool        string                      `json:"music_pool,omitempty"`
	ProxyPrefix      string                      `json:"proxy_prefix,omitempty"`
	ProxyDomains     []string                    `json:"proxy_domains,omitempty"`
	NoProxyProviders []string                    `json:"noproxy_providers,omitempty"`
	ComparisonFields []poolConfigFieldComparison `json:"comparison_fields,omitempty"`
	MaxSupported     int                         `json:"max_supported,omitempty"`
	handler          customFieldHandler          // pointer to this field's handler function
}

type poolConfigField struct {
	Name               string                       `json:"name,omitempty"` // required; v4 field name, and key for common fields
	XID                string                       `json:"xid,omitempty"`
	Field              string                       `json:"field,omitempty"`
	Properties         poolConfigFieldProperties    `json:"properties,omitempty"`
	Limit              int                          `json:"limit,omitempty"`
	SplitOn            string                       `json:"split_on,omitempty"`
	OnShelfOnly        bool                         `json:"onshelf_only,omitempty"`
	DigitalContentOnly bool                         `json:"digital_content_only,omitempty"`
	CitationOnly       bool                         `json:"citation_only,omitempty"`
	Value              string                       `json:"value,omitempty"`
	MinimalRole        string                       `json:"minimal_role,omitempty"`
	CustomConfig       *poolConfigFieldCustomConfig `json:"custom_config,omitempty"` // extra info for certain custom formats
}

type poolConfigAvailabilityValues struct {
	OnShelf  []string `json:"onshelf,omitempty"`
	Online   []string `json:"online,omitempty"`
	Other    []string `json:"other,omitempty"`
	Combined []string `json:"-"` // derived from above values
}

type poolConfigAvailabilityTypeConfig struct {
	FieldAnon     string                       `json:"anon,omitempty"`
	FieldAuth     string                       `json:"auth,omitempty"`
	ExposedValues poolConfigAvailabilityValues `json:"exposed_values,omitempty"`
}

type poolConfigAvailability struct {
	FieldConfig  poolConfigAvailabilityTypeConfig `json:"field_config,omitempty"`
	FilterConfig poolConfigAvailabilityTypeConfig `json:"filter_config,omitempty"`
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

type poolConfigFilter struct {
	XID              string                 `json:"xid,omitempty"` // translation ID
	Solr             poolConfigFacetSolr    `json:"solr,omitempty"`
	Type             string                 `json:"type,omitempty"`
	Format           string                 `json:"format,omitempty"`
	ExposedValues    []string               `json:"exposed_values,omitempty"`
	ComponentQueries []poolConfigFacetQuery `json:"component_queries,omitempty"`
	IsAvailability   bool                   `json:"is_availability,omitempty"`
	BucketSort       string                 `json:"bucket_sort,omitempty"`
	Hidden           bool                   `json:"hidden,omitempty"`
	Index            int                    `json:"-"`
	queryMap         map[string]*poolConfigFacetQuery
}

type poolConfigSort struct {
	XID          string `json:"xid,omitempty"`      // translation ID
	AscXID       string `json:"asc_xid,omitempty"`  // translation ID
	DescXID      string `json:"desc_xid,omitempty"` // translation ID
	Field        string `json:"field,omitempty"`
	Order        string `json:"order,omitempty"`
	RecordXID    string `json:"record_xid,omitempty"` // translation ID
	RecordOrder  string `json:"record_order,omitempty"`
	GroupResults bool   `json:"group_results,omitempty"`
	IsRelevance  bool   `json:"is_relevance,omitempty"`
}

type poolConfigIdentity struct {
	NameXID        string   `json:"name_xid,omitempty"`   // translation ID
	DescXID        string   `json:"desc_xid,omitempty"`   // translation ID
	Mode           string   `json:"mode,omitempty"`       // pool mode (what it is, e.g. "record" (default), "image", etc.)
	Source         string   `json:"source,omitempty"`     // pool source (where its data comes from -- probably should be a unique value per core, e.g. "solr" for catalog stuff, "solr-images" for image stuff, etc.)
	Attributes     []string `json:"attributes,omitempty"` // pool attributes (what it supports)
	CitationFormat string   `json:"citation_format,omitempty"`
}

type poolConfigProvider struct {
	Name    string `json:"name,omitempty"`
	XID     string `json:"xid,omitempty"` // translation ID
	URL     string `json:"url,omitempty"`
	Logo    string `json:"logo,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	re      *regexp.Regexp
}

type poolConfigRelatedImage struct {
	IIIFManifestField string `json:"iiif_manifest_field,omitempty"`
	IIIFImageField    string `json:"iiif_image_field,omitempty"`
}

type poolConfigRelated struct {
	Image *poolConfigRelatedImage `json:"image,omitempty"`
}

type poolConfigMappingsDefinitions struct {
	Fields  []poolConfigField  `json:"fields,omitempty"`
	Filters []poolConfigFilter `json:"filters,omitempty"`
	Sorts   []poolConfigSort   `json:"sorts,omitempty"`
}

type poolConfigMappingsHeadingField struct {
	Name         string `json:"name,omitempty"`
	Type         string `json:"type,omitempty"`
	CitationPart string `json:"citation_part,omitempty"`
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
	FilterXIDs []string                           `json:"filter_xids,omitempty"`
	SortXIDs   []string                           `json:"sort_xids,omitempty"`
}

type poolConfigMappings struct {
	Definitions poolConfigMappingsDefinitions `json:"definitions,omitempty"`
	Configured  poolConfigMappingsConfigured  `json:"configured,omitempty"`
}

type poolConfigCitationFormat struct {
	Format  string `json:"format,omitempty"`
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

type resourceTypeFields struct {
	basic    []poolConfigField
	detailed []poolConfigField
}

type poolConfigFilterOverride struct {
	XID                 string   `json:"xid,omitempty"`
	DependentFilterXIDs []string `json:"dependent_filter_xids,omitempty"`
}

type poolConfigResourceTypeContext struct {
	Value                string                              `json:"value,omitempty"`
	XID                  string                              `json:"xid,omitempty"`
	AuthorFields         poolConfigAuthorFields              `json:"author_fields,omitempty"`
	FieldNames           poolConfigMappingsConfiguredFields  `json:"field_names,omitempty"`
	AdditionalFilterXIDs []string                            `json:"additional_filter_xids,omitempty"`
	FilterOverrides      map[string]poolConfigFilterOverride `json:"filter_overrides,omitempty"`
	filters              []poolConfigFilter
	filterMap            map[string]*poolConfigFilter
	filterXIDs           []string
	fields               resourceTypeFields
}

type poolConfigResourceTypes struct {
	Field             string                          `json:"field,omitempty"`
	FilterXID         string                          `json:"filter_xid,omitempty"`
	DefaultContext    string                          `json:"default_context,omitempty"`
	SupportedContexts []string                        `json:"supported_contexts,omitempty"`
	Contexts          []poolConfigResourceTypeContext `json:"contexts,omitempty"`
}

type poolConfigGlobal struct {
	Service          poolConfigService          `json:"service,omitempty"`
	Solr             poolConfigSolr             `json:"solr,omitempty"`
	Attributes       []string                   `json:"attributes,omitempty"`
	Providers        []poolConfigProvider       `json:"providers,omitempty"`
	Availability     poolConfigAvailability     `json:"availability,omitempty"`
	CitationFormats  []poolConfigCitationFormat `json:"citation_formats,omitempty"`
	RecordAttributes poolConfigRecordAttributes `json:"record_attributes,omitempty"`
	Publishers       []poolConfigPublisher      `json:"publishers,omitempty"`
	Relators         poolConfigRelators         `json:"relators,omitempty"`
	Titleization     poolConfigTitleization     `json:"titleization,omitempty"`
	Copyrights       []poolConfigCopyright      `json:"copyrights,omitempty"`
	Mappings         poolConfigMappings         `json:"mappings,omitempty"`
	ResourceTypes    poolConfigResourceTypes    `json:"resource_types,omitempty"`
}

type poolConfigLocal struct {
	Identity poolConfigIdentity `json:"identity,omitempty"`
	Solr     poolConfigSolr     `json:"solr,omitempty"`
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
