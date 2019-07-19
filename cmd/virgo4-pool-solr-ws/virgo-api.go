package main

// schemas

// based loosely on internal discussions, will solidify here:
// https://github.com/uvalib/v4-api/blob/master/pool-search-api-OAS3.json

// VirgoSearchRequest holds the contents of a search request as parsed
// from JSON defined in the Virgo API.
type VirgoSearchRequest struct {
	Query      string           `json:"query,omitempty"`
	solrQuery  string           // used internally
	Pagination *VirgoPagination `json:"pagination,omitempty"`
	Facets     *VirgoFacetList  `json:"facets,omitempty"`
	Filters    *VirgoFacetList  `json:"filters,omitempty"`
}

// VirgoPoolResultDebug is an arbitrary set of key-value pairs of debugging
// info for the overall pool result (the response to a search request).
// The client can request this via the "debug" query parameter.
type VirgoPoolResultDebug struct {
	MaxScore float32 `json:"max_score"`
}

// VirgoPoolResultWarn is an arbitrary list of strings containing any non-fatal
// warnings that should be reported back to the client.
type VirgoPoolResultWarn []string

// VirgoPoolResult contains the full response to a search request
type VirgoPoolResult struct {
	ServiceURL string                `json:"service_url,omitempty"` // required
	Pagination *VirgoPagination      `json:"pagination,omitempty"`
	RecordList *VirgoRecordList      `json:"record_list,omitempty"`
	Facets     *VirgoFacetList       `json:"facets,omitempty"`  // available facets advertised to the client
	FacetList  *VirgoFacetList       `json:"facet_list,omitempty"` // facet values for client-requested facets
	Confidence string                `json:"confidence,omitempty"` // required; i.e. low, medium, high, exact
	Debug      *VirgoPoolResultDebug `json:"debug,omitempty"`
	Warn       *VirgoPoolResultWarn  `json:"warn,omitempty"`
}

// VirgoRecordDebug is an arbitrary set of key-value pairs of debugging
// info for a particular record in a search result set.
// The client can request this via the "debug" query parameter.
type VirgoRecordDebug struct {
	Score float32 `json:"score"`
}

// VirgoRecord contains the fields for a single record in a search result set.
type VirgoRecord struct {
	ID       string            `json:"id,omitempty"`
	Title    string            `json:"title,omitempty"`
	Subtitle string            `json:"subtitle,omitempty"`
	Author   string            `json:"author,omitempty"`
	Debug    *VirgoRecordDebug `json:"debug,omitempty"`
}

// VirgoRecordList is a list of records generated from a search (the "search result set").
type VirgoRecordList []VirgoRecord

// VirgoFacetBucket contains the fields for an individual bucket for a facet.
type VirgoFacetBucket struct {
	Val   string `json:"val"`
	Count int    `json:"count"`
}

// VirgoFacet contains the fields for a single facet.
type VirgoFacet struct {
	Name    string             `json:"name"`
	Type    string             `json:"type,omitempty"` // when advertised as part of a non-faceted/non-filtered search response
	Value   string             `json:"value,omitempty"` // when used as a filter in a search request
	Sort    string             `json:"sort,omitempty"` // when used as a facet or filter in a search request
	Offset  int                `json:"offset,omitempty"` // when used as a facet or filter in a search request
	Limit   int                `json:"limit,omitempty"` // when used as a facet or filter in a search request
	Buckets []VirgoFacetBucket `json:"buckets,omitempty"` // when returned as part of a facted search response
}

// VirgoFacetList is a list of facets or filters either requested by the client or returned from a search.
type VirgoFacetList []VirgoFacet

// VirgoPagination defines a page (contiguous subset) of records for a given search.
type VirgoPagination struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
	Total int `json:"total"`
}
