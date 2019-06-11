package main

// schemas

// based loosely on internal discussions, will solidify here:
// https://github.com/uvalib/v4-api/blob/master/search-api-OAS3.json

type VirgoSearchRequest struct {
	Query      string           `json:"query,omitempty"`
	solrQuery  string           // used internally
	Pagination *VirgoPagination `json:"pagination,omitempty"`
}

type VirgoPoolResultDebug struct {
	MaxScore float32 `json:"max_score"`
}

type VirgoPoolResultWarn []string

type VirgoPoolResult struct {
	ServiceUrl string                `json:"service_url,omitempty"` // required
	Pagination *VirgoPagination      `json:"pagination,omitempty"`
	RecordList *VirgoRecordList      `json:"record_list,omitempty"`
	Confidence string                `json:"confidence,omitempty"` // required; i.e. low, medium, high, exact
	Debug      *VirgoPoolResultDebug `json:"debug,omitempty"`
	Warn       *VirgoPoolResultWarn  `json:"warn,omitempty"`
}

type VirgoRecordDebug struct {
	Score float32 `json:"score"`
}

type VirgoRecord struct {
	Id       string            `json:"id,omitempty"`
	Title    string            `json:"title,omitempty"`
	Subtitle string            `json:"subtitle,omitempty"`
	Author   string            `json:"author,omitempty"`
	Debug    *VirgoRecordDebug `json:"debug,omitempty"`
}

type VirgoRecordList []VirgoRecord

type VirgoPagination struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
	Total int `json:"total"`
}

type VirgoPoolRegistration struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}
