package main

// schemas

type VirgoSearchRequest struct {
	Query             string                 `json:"query" binding:"exists"`
	CurrentPool       string                 `json:"current_pool"`
	Start             int                    `json:"start"`
	Rows              int                    `json:"rows"`
	SearchPreferences VirgoSearchPreferences `json:"search_preferences"`
}

type VirgoSearchResponse struct {
	ActualRequest    VirgoSearchRequest   `json:"actual_request"`
	EffectiveRequest VirgoSearchRequest   `json:"effective_request"`
	Results          VirgoSearchResultSet `json:"results"`
	PoolSummaryList  VirgoPoolSummaryList `json:"pool_summary_list"`
}

type VirgoSearchResultSet struct {
	ResultCount int             `json:"result_count"`
	Pagination  VirgoPagination `json:"pagination"`
	Filters     VirgoFilters    `json:"filters"`
	RecordSet   VirgoRecordSet  `json:"record_set"`
}

type VirgoRecordSet []VirgoRecord

type VirgoRecord struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

type VirgoPagination struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
	Total int `json:"total"`
}

type VirgoFilters struct {
}

type VirgoPoolSummaryList []VirgoPoolSummary

type VirgoPoolSummary struct {
	Name    string `json:"name"`
	Link    string `json:"link"`
	Summary string `json:"summary"`
}

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
