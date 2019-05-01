package main

// schemas

type VirgoSearchRequest struct {
	Query string `json:"query,omitempty" binding:"exists"`
	CurrentPool string `json:"current_pool,omitempty"`
	Start int `json:"start,omitempty"`
	Rows int `json:"rows,omitempty"`
	SearchPreferences VirgoSearchPreferences `json:"search_preferences,omitempty"`
}

type VirgoSearchResponse struct {
	ActualRequest VirgoSearchRequest `json:"actual_request,omitempty"`
	EffectiveRequest VirgoSearchRequest `json:"effective_request,omitempty"`
	Results VirgoSearchResultSet `json:"results,omitempty"`
	PoolSummaryList VirgoPoolSummaryList `json:"pool_summary_list,omitempty"`
}

type VirgoSearchResultSet struct {
	ResultCount int `json:"result_count,omitempty"`
	Pagination VirgoPagination `json:"pagination,omitempty"`
	Filters VirgoFilters `json:"filters,omitempty"`
	RecordSet VirgoRecordSet `json:"record_set,omitempty"`
}

type VirgoRecordSet []VirgoRecord

type VirgoRecord struct {
	Title string `json:"title,omitempty"`
}

type VirgoPagination struct {
	Start int `json:"start,omitempty"`
	Rows int `json:"rows,omitempty"`
	Total int `json:"total,omitempty"`
}

type VirgoFilters struct {
}

type VirgoPoolSummaryList []VirgoPoolSummary

type VirgoPoolSummary struct {
	Name string `json:"name,omitempty"`
	Link string `json:"link,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type VirgoUser struct {
	Preferences VirgoUserPreferences `json:"preferences,omitempty"`
	Info VirgoUserInfo `json:"info,omitempty"`
}

type VirgoUserPreferences struct {
}

type VirgoSearchPreferences struct {
	DefaultSearchPool string `json:"default_search_pool,omitempty"`
	ExcludedPools []string `json:"excluded_pools,omitempty"`
	DefaultSort string `json:"default_sort,omitempty"`
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
	Id string `json:"id,omitempty" binding:"required"`
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
