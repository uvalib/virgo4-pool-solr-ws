package main

// schemas

// based loosely on internal discussions, will solidify here:
// https://github.com/uvalib/v4-api/blob/master/search-api-OAS3.json

type VirgoSearchRequest struct {
	Query      string           `json:"query,omitempty"`
	Pagination *VirgoPagination `json:"pagination,omitempty"`
}

type VirgoPoolResult struct {
	ServiceUrl string           `json:"service_url,omitempty"` // required
	Pagination *VirgoPagination `json:"pagination,omitempty"`
	RecordList VirgoRecordList  `json:"record_list,omitempty"`
	Confidence string           `json:"confidence,omitempty"` // required; i.e. low, medium, high, exact
}

type VirgoRecord struct {
	Id     string `json:"id,omitempty"`
	Title  string `json:"title,omitempty"`
	Author string `json:"author,omitempty"`
}

type VirgoRecordList []VirgoRecord

type VirgoPagination struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
	Total int `json:"total"`
}

type VirgoPoolRegistration struct {
	Url string `json:"url"`
}
