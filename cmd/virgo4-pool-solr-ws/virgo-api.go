package main

// schemas

// based on https://app.swaggerhub.com/apis/jlj5aj/VirgoSearchAPI/0.2

type VirgoSearchOptions struct {
	SearchType string `json:"search_type"` // "basic" or "advanced"
	Id         string `json:"id"`
	Keyword    string `json:"keyword"`
	Author     string `json:"author"`
	Title      string `json:"title"`
	Subject    string `json:"subject"`
	SortField  string `json:"sort_field"`
	SortOrder  string `json:"sort_order"`
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
	ResultCount int              `json:"result_count"`
	Pagination  VirgoPagination  `json:"pagination"`
	Filters     VirgoFilters     `json:"filters"`
	RecordList  VirgoRecordList  `json:"record_list"`
	Summary     VirgoPoolSummary `json:"summary"`
}

type VirgoPoolResultList []VirgoPoolResult

type VirgoRecord struct {
	Id     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
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
	Name    string `json:"name"`
	Link    string `json:"link"`
	Summary string `json:"summary"`
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

/*
// requests/responses

// essentially stripped-down version of VirgoSearchRequest
type VirgoPoolResultsRequest struct {
	VirgoSearchRequest
}

type VirgoPoolResultsResponse struct {
	VirgoSearchResultSet
}

type VirgoPoolResultsRecordRequest struct {
	Id string `json:"id" binding:"required"`
}

type VirgoPoolResultsRecordResponse struct {
	VirgoSearchResultSet
}

type VirgoPoolSummaryRequest struct {
	VirgoSearchRequest
}

type VirgoPoolSummaryResponse struct {
	VirgoPoolSummary
}
*/
