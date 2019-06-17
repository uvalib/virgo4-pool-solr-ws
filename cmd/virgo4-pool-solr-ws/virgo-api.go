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

// VirgoPagination defines a page (contiguous subset) of records for a given search.
type VirgoPagination struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
	Total int `json:"total"`
}

// VirgoPoolRegistration contains the information reported when registrering
// with the interpool web service.
type VirgoPoolRegistration struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
