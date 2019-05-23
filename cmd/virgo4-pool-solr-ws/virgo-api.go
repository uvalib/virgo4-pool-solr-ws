package main

// schemas

// based loosely on: https://github.com/uvalib/v4-api/blob/master/search-api-OAS3.json

type VirgoSearchOptions struct {
	Id      string             // not in API, but used internally for detailed record search
	Keyword string             `json:"keyword,omitempty"`
	Author  string             `json:"author,omitempty"`
	Title   string             `json:"title,omitempty"`
	Subject string             `json:"subject,omitempty"`
	Sort    *VirgoSortCriteria `json:"sort,omitempty"`
}

type VirgoSearchRequest struct {
	Query             *VirgoSearchOptions     `json:"query,omitempty" binding:"exists"`
	Pagination        *VirgoPaginationRequest `json:"pagination,omitempty"`
	SearchPreferences *VirgoSearchPreferences `json:"search_preferences,omitempty"`
}

type VirgoSearchResponse struct {
	ActualRequest    *VirgoSearchRequest `json:"actual_request,omitempty"`
	EffectiveRequest *VirgoSearchRequest `json:"effective_request,omitempty"`
	ResultsPools     VirgoPoolResultList `json:"results_pools,omitempty"`
}

type VirgoPoolResult struct {
	PoolId     string           `json:"pool_id,omitempty"`     // required
	ServiceUrl string           `json:"service_url,omitempty"` // required
	Summary    string           `json:"summary,omitempty"`
	Pagination *VirgoPagination `json:"pagination,omitempty"`
	RecordList VirgoRecordList  `json:"record_list,omitempty"`
	Filters    *VirgoFilters    `json:"filters,omitempty"`
	Confidence string           `json:"confidence,omitempty"` // required; i.e. low, medium, high, exact
}

type VirgoPoolResultList []VirgoPoolResult

type VirgoRecord struct {
	Id     string `json:"id,omitempty"`
	Title  string `json:"title,omitempty"`
	Author string `json:"author,omitempty"`
}

type VirgoRecordList []VirgoRecord

type VirgoPaginationRequest struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
}

type VirgoPagination struct {
	VirgoPaginationRequest
	Total int `json:"total"`
}

type VirgoFilters struct {
}

type VirgoUser struct {
	Preferences *VirgoUserPreferences `json:"preferences,omitempty"`
	Info        *VirgoUserInfo        `json:"info,omitempty"`
}

type VirgoUserPreferences struct {
}

type VirgoSearchPreferences struct {
	DefaultSearchPool string             `json:"default_search_pool,omitempty"`
	ExcludedPools     []string           `json:"excluded_pools,omitempty"`
	DefaultSort       *VirgoSortCriteria `json:"default_sort,omitempty"`
}

type VirgoUserInfo struct {
}

type VirgoSortCriteria struct {
	Field string `json:"field,omitempty"` // e.g. title, author, subject, ...
	Order string `json:"order,omitempty"` // i.e. asc, desc, none
}

type VirgoPoolRegistration struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}
