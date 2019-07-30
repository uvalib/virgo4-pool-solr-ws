package main

// schemas

// based loosely on internal discussions, will solidify here:
// https://github.com/uvalib/v4-api/blob/master/pool-search-api-OAS3.json

type virgoSearchMeta struct {
	client        *clientOptions
	solrQuery     string // holds the parsed solr query
	requestFacets bool   // set to true for non-speculative searches
}

// VirgoSearchRequest holds the contents of a search request as parsed
// from JSON defined in the Virgo API.
type VirgoSearchRequest struct {
	Query      string          `json:"query"`
	Pagination VirgoPagination `json:"pagination"`
	Facet      string          `json:"facet"`
	Filters    *[]VirgoFilter  `json:"filters,omitempty"`
	meta       virgoSearchMeta // used internally
}

// VirgoPoolResultDebug is an arbitrary set of key-value pairs of debugging
// info for the overall pool result (the response to a search request).
// The client can request this via the "debug" query parameter.
type VirgoPoolResultDebug struct {
	MaxScore float32 `json:"max_score"`
}

// VirgoPoolResult contains the full response to a search request
type VirgoPoolResult struct {
	ServiceURL      string                `json:"service_url,omitempty"` // required
	Pagination      *VirgoPagination      `json:"pagination,omitempty"`
	GroupList       *[]VirgoGroup         `json:"group_list,omitempty"`
	AvailableFacets *[]string             `json:"available_facets,omitempty"` // available facets advertised to the client
	FacetList       *[]VirgoFacet         `json:"facet_list,omitempty"`       // facet values for client-requested facets
	Confidence      string                `json:"confidence,omitempty"`       // required; i.e. low, medium, high, exact
	Debug           *VirgoPoolResultDebug `json:"debug,omitempty"`
	Warn            *[]string             `json:"warn,omitempty"`
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
	Type       string `json:"type"` // assume simple string if not provided
	Label      string `json:"label"`
	Value      string `json:"value"`      // could be any type
	Visibility string `json:"visibility"` // e.g. "basic" (or empty) as opposed to "detailed"
}

// VirgoRecord contains the fields for a single record in a search result set.
type VirgoRecord struct {
	Debug    *VirgoRecordDebug   `json:"debug,omitempty"`
	Fields   []VirgoNuancedField `json:"fields,omitempty"`
}

// VirgoGroup contains the records for a single group in a search result set.
type VirgoGroup struct {
	Value      string        `json:"value,omitempty"`
	Count      int           `json:"count,omitempty"`
	RecordList []VirgoRecord `json:"record_list,omitempty"`
}

// VirgoFacetBucket contains the fields for an individual bucket for a facet.
type VirgoFacetBucket struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// VirgoFilter contains the fields for a single filter.
type VirgoFilter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// VirgoFacet contains the fields for a single facet.
type VirgoFacet struct {
	Name    string             `json:"name"`
	Type    string             `json:"type,omitempty"`    // when advertised as part of a non-faceted/non-filtered search response
	Value   string             `json:"value,omitempty"`   // when used as a filter in a search request
	Sort    string             `json:"sort,omitempty"`    // when used as a facet or filter in a search request
	Offset  int                `json:"offset,omitempty"`  // when used as a facet or filter in a search request
	Limit   int                `json:"limit,omitempty"`   // when used as a facet or filter in a search request
	Buckets []VirgoFacetBucket `json:"buckets,omitempty"` // when returned as part of a facted search response
}

// VirgoPagination defines a page (contiguous subset) of records for a given search.
type VirgoPagination struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
	Total int `json:"total"`
}
