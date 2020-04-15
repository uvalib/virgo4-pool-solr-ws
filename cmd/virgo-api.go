package main

// schemas

// based loosely on internal discussions, will solidify here:
// https://github.com/uvalib/v4-api/blob/master/pool-search-api-OAS3.json

type virgoSearchMeta struct {
	client        *clientContext
	solrQuery     string          // holds the solr query (either parsed or specified)
	parserInfo    *solrParserInfo // holds the information for parsed queries
	requestFacets bool            // set to true for non-speculative searches
}

// VirgoSearchRequest holds the contents of a search request as parsed
// from JSON defined in the Virgo API.
type VirgoSearchRequest struct {
	Query      string          `json:"query"`
	Pagination VirgoPagination `json:"pagination"`
	Sort       *VirgoSort      `json:"sort,omitempty"`
	Filters    *VirgoFilters   `json:"filters,omitempty"`
	meta       virgoSearchMeta // used internally
}

// VirgoPoolResultDebug is an arbitrary set of key-value pairs of debugging
// info for the overall pool result (the response to a search request).
// The client can request this via the "debug" query parameter.
type VirgoPoolResultDebug struct {
	RequestID string  `json:"request_id"`
	MaxScore  float32 `json:"max_score"`
}

// VirgoPoolAttribute holds attributes that this pool does or does not support
type VirgoPoolAttribute struct {
	Name      string `json:"name"`
	Supported bool   `json:"supported"`
	Value     string `json:"value,omitempty"`
}

// VirgoPoolAttributes is a slice of VirgoPoolAttribute structs
type VirgoPoolAttributes []VirgoPoolAttribute

// VirgoProvider contains the attributes for a single provider
type VirgoProvider struct {
	Provider    string `json:"provider"`
	Label       string `json:"label,omitempty"`
	HomepageURL string `json:"homepage_url,omitempty"`
	LogoURL     string `json:"logo_url,omitempty"`
}

// VirgoProviders is a slice of VirgoProvider structs
type VirgoProviders []VirgoProvider

// VirgoPoolProviders holds information about any provider this pool may return
type VirgoPoolProviders struct {
	Providers VirgoProviders `json:"providers"`
}

// VirgoPoolIdentity holds localized information about this pool (same as returned by /identify endpoint)
type VirgoPoolIdentity struct {
	Name        string              `json:"name,omitempty"`         // localized pool name
	Description string              `json:"description,omitempty"`  // localized pool description (detailed information about what the pool contains)
	Mode        string              `json:"mode,omitempty"`         // pool mode (what it is, e.g. "record" (default), "image", etc.)
	Attributes  VirgoPoolAttributes `json:"attributes,omitempty"`   // pool attributes (what it supports)
	SortOptions VirgoSortOptions    `json:"sort_options,omitempty"` // available sort options
}

// VirgoPoolResult contains the full response to a search request
type VirgoPoolResult struct {
	Identity   VirgoPoolIdentity     `json:"identity"`              // localized identity
	Pagination *VirgoPagination      `json:"pagination,omitempty"`  // pagination info for results
	Sort       *VirgoSort            `json:"sort,omitempty"`        // sort info for results
	RecordList *VirgoRecords         `json:"record_list,omitempty"` // ungrouped records
	GroupList  *VirgoGroups          `json:"group_list,omitempty"`  // grouped records
	FacetList  *VirgoFacets          `json:"facet_list,omitempty"`  // facet values for client-requested facets
	Confidence string                `json:"confidence,omitempty"`  // required; i.e. low, medium, high, exact
	ElapsedMS  int64                 `json:"elapsed_ms,omitempty"`  // total round-trip time for this request
	Debug      *VirgoPoolResultDebug `json:"debug,omitempty"`
	Warn       *[]string             `json:"warn,omitempty"`
}

// VirgoFacetsResult contains the full response to a facets request
type VirgoFacetsResult struct {
	FacetList *VirgoFacets `json:"facet_list,omitempty"` // facet values for client-requested facets
	ElapsedMS int64        `json:"elapsed_ms,omitempty"` // total round-trip time for this request
}

// VirgoRecordDebug is an arbitrary set of key-value pairs of debugging
// info for a particular record in a search result set.
// The client can request this via the "debug" query parameter.
type VirgoRecordDebug struct {
	Score float32 `json:"score"`
}

// VirgoNuancedField contains metadata for a single field in a record.
type VirgoNuancedField struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"` // assume simple string if not provided
	Label      string `json:"label,omitempty"`
	Value      string `json:"value"`
	Visibility string `json:"visibility,omitempty"` // e.g. "basic" (or empty) as opposed to "detailed"
	Display    string `json:"display,omitempty"`    // e.g. "optional" (or empty)
	Provider   string `json:"provider,omitempty"`   // for URLs (e.g. "hathitrust", "proquest")
	Item       string `json:"item,omitempty"`       // for certain URLs (currently hathitrust)
}

// VirgoNuancedFields is a slice of VirgoNuancedField structs
type VirgoNuancedFields []VirgoNuancedField

// VirgoRelatedRecord contains basic info for records with the same group value
type VirgoRelatedRecord struct {
	ID              string `json:"id,omitempty"`
	IIIFManifestURL string `json:"iiif_manifest_url,omitempty"`
	IIIFImageURL    string `json:"iiif_image_url,omitempty"`
	IIIFBaseURL     string `json:"iiif_base_url,omitempty"`
}

// VirgoRelatedRecords is a slice of VirgoRelatedRecord structs
type VirgoRelatedRecords []VirgoRelatedRecord

// VirgoRecord contains the fields for a single record in a search result set.
type VirgoRecord struct {
	Fields     VirgoNuancedFields   `json:"fields,omitempty"`
	Related    *VirgoRelatedRecords `json:"related,omitempty"`
	Exact      bool                 `json:"exact,omitempty"`
	Debug      *VirgoRecordDebug    `json:"debug,omitempty"`
	groupValue string               // used internally
}

// VirgoRecords is a slice of VirgoRecord structs
type VirgoRecords []VirgoRecord

// VirgoGroup contains the records for a single group in a search result set.
type VirgoGroup struct {
	Value      string             `json:"value,omitempty"`
	Count      int                `json:"count,omitempty"`
	Fields     VirgoNuancedFields `json:"fields,omitempty"`
	RecordList VirgoRecords       `json:"record_list,omitempty"`
}

// VirgoGroups is a slice of VirgoGroup structs
type VirgoGroups []VirgoGroup

// VirgoFacetBucket contains the fields for an individual bucket for a facet.
type VirgoFacetBucket struct {
	Value    string `json:"value,omitempty"`
	Count    int    `json:"count,omitempty"`
	Selected bool   `json:"selected"`
}

// VirgoFacetBuckets is a slice of VirgoFacetBucket structs
type VirgoFacetBuckets []VirgoFacetBucket

// VirgoFilter contains the fields for a single filter.
type VirgoFilter struct {
	PoolID string `json:"pool_id"`
	Facets []struct {
		FacetID string `json:"facet_id"`
		Value   string `json:"value"`
	} `json:"facets"`
}

// VirgoFilters is a slice of VirgoFilter structs
type VirgoFilters []VirgoFilter

// VirgoFacet contains the fields for a single facet.
type VirgoFacet struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type,omitempty"`    // when advertised as part of a non-faceted/non-filtered search response
	Value   string            `json:"value,omitempty"`   // when used as a filter in a search request
	Sort    string            `json:"sort,omitempty"`    // when used as a facet or filter in a search request
	Offset  int               `json:"offset,omitempty"`  // when used as a facet or filter in a search request
	Limit   int               `json:"limit,omitempty"`   // when used as a facet or filter in a search request
	Buckets VirgoFacetBuckets `json:"buckets,omitempty"` // when returned as part of a facted search response
}

// VirgoFacets is a slice of VirgoFacet structs
type VirgoFacets []VirgoFacet

// VirgoPagination defines a page (contiguous subset) of records for a given search.
type VirgoPagination struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
	Total int `json:"total"`
}

// VirgoSort specifies sort options for a given search.
type VirgoSort struct {
	SortID string `json:"sort_id"`
	Order  string `json:"order"`
}

// VirgoSortOption defines an available sort option.
type VirgoSortOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// VirgoSortOptions is a slice of VirgoSortOption structs
type VirgoSortOptions []VirgoSortOption
