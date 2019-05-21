package main

// schemas

// based on: https://github.com/uvalib/v4-api/blob/c4ec4962d77e91d8a74f9d626b6091574ec0298c/search-api-OAS3.json

type VirgoSearchOptions struct {
	SearchType string `json:"search_type"` // i.e. basic, advanced
	Id         string `json:"id"`
	Keyword    string `json:"keyword"`
	Author     string `json:"author"`
	Title      string `json:"title"`
	Subject    string `json:"subject"`
	SortField  string `json:"sort_field"` // e.g. title, author, subject, ...
	SortOrder  string `json:"sort_order"` // i.e. asc, desc, none
}

type VirgoSearchRequest struct {
	Query             *VirgoSearchOptions    `json:"query" binding:"exists"`
	CurrentPool       string                 `json:"current_pool"`
	Pagination        VirgoPagination        `json:"pagination"`
	SearchPreferences VirgoSearchPreferences `json:"search_preferences"`
}

type VirgoSearchResponse struct {
	ActualRequest    VirgoSearchRequest   `json:"actual_request"`
	EffectiveRequest VirgoSearchRequest   `json:"effective_request"`
	PoolResultList   VirgoPoolResultList  `json:"pool_result_list"`
	PoolSummaryList  VirgoPoolSummaryList `json:"pool_summary_list"`
}

type VirgoPoolResult struct {
	Pagination VirgoPagination  `json:"pagination"`
	Filters    VirgoFilters     `json:"filters"`
	RecordList VirgoRecordList  `json:"record_list"`
	Summary    VirgoPoolSummary `json:"summary"`
}

type VirgoPoolResultList []VirgoPoolResult

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

type VirgoFilters struct {
}

type VirgoPoolSummary struct {
	Name       string `json:"name,omitempty"`
	Link       string `json:"link,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Confidence string `json:"confidence,omitempty"` // i.e. low, medium, high, exact
}

type VirgoPoolSummaryList []VirgoPoolSummary

type VirgoUser struct {
	Preferences VirgoUserPreferences `json:"preferences"`
	Info        VirgoUserInfo        `json:"info"`
}

type VirgoUserPreferences struct {
}

type VirgoSearchPreferences struct {
	DefaultSearchPool string   `json:"default_search_pool"`
	ExcludedPools     []string `json:"excluded_pools"`
	DefaultSort       string   `json:"default_sort"`
}

type VirgoUserInfo struct {
}
