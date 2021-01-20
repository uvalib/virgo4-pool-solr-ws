package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/uvalib/virgo4-api/v4api"
)

type virgoFlags struct {
	groupResults     bool
	requestFacets    bool
	facetCache       bool
	globalFacetCache bool
	firstRecordOnly  bool
}

type virgoDialog struct {
	req            v4api.SearchRequest
	poolRes        *v4api.PoolResult
	facetsRes      *v4api.PoolFacets
	recordRes      *v4api.Record
	solrQuery      string          // holds the solr query (either parsed or specified)
	parserInfo     *solrParserInfo // holds the information for parsed queries
	skipQuery      bool            // should we skip Solr communcation and just return empty results?
	flags          virgoFlags
	endpoint       string
	body           string
	currentFacet   string // which facet to consider when iterating over facets to build response
	totalFilters   int    // number of (valid) filters in the request
	invalidFilters bool   // whether the request contains an unsupported filter
}

type solrDialog struct {
	req solrRequest
	res solrResponse
}

type searchContext struct {
	pool            *poolContext
	client          *clientContext
	virgo           virgoDialog
	solr            solrDialog
	resourceTypeCtx *poolConfigResourceTypeContext
	confidence      string
	itemDetails     bool
}

type searchResponse struct {
	status int         // http status code
	data   interface{} // data to return as JSON
	err    error       // error, if any
}

func confidenceIndex(s string) int {
	// invalid values will be 0 (less than "low")
	return map[string]int{
		"low":    1,
		"medium": 2,
		"high":   3,
		"exact":  4,
	}[s]
}

func (s *searchContext) init(p *poolContext, c *clientContext) {
	s.pool = p
	s.client = c
	s.virgo.flags.groupResults = true
	s.resourceTypeCtx = s.pool.maps.resourceTypeContexts[s.pool.config.Global.ResourceTypes.DefaultContext]
}

func (s *searchContext) copySearchContext() *searchContext {
	// performs a copy somewhere between shallow and deep
	// (just enough to let this context be used for another search
	// without potentially clobbering the original context by leaving
	// pointers into the original context)

	sc := &searchContext{}

	sc.pool = s.pool

	// copy client (modified for speculative searches)
	c := *s.client
	sc.client = &c

	v := s.virgo.req
	sc.virgo.req = v

	sc.virgo.endpoint = s.virgo.endpoint
	sc.virgo.flags = s.virgo.flags

	sc.resourceTypeCtx = s.resourceTypeCtx

	return sc
}

func (s *searchContext) log(format string, args ...interface{}) {
	s.client.log(format, args...)
}

func (s *searchContext) warn(format string, args ...interface{}) {
	s.client.warn(format, args...)
}

func (s *searchContext) err(format string, args ...interface{}) {
	s.client.err(format, args...)
}

func (s *searchContext) verbose(format string, args ...interface{}) {
	s.client.verbose(format, args...)
}

func (s *searchContext) performQuery() searchResponse {
	//s.log("**********  START SOLR QUERY  **********")

	if resp := s.solrSearchRequest(); resp.err != nil {
		s.err("query creation error: %s", resp.err.Error())
		return resp
	}

	err := s.solrQuery()

	//s.log("**********   END SOLR QUERY   **********")

	if err != nil {
		s.err("query execution error: %s", err.Error())
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) getPoolQueryResults() searchResponse {
	if resp := s.performQuery(); resp.err != nil {
		return resp
	}

	if resp := s.buildPoolSearchResponse(); resp.err != nil {
		s.err("result parsing error: %s", resp.err.Error())
		return resp
	}

	s.confidence = s.virgo.poolRes.Confidence

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) getRecordQueryResults() searchResponse {
	if resp := s.getSingleDocument(); resp.err != nil {
		return resp
	}

	if resp := s.buildPoolRecordResponse(); resp.err != nil {
		s.err("result parsing error: %s", resp.err.Error())
		return resp
	}

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) getSingleDocument() searchResponse {
	// override these values from defaults.  specify two rows to catch
	// the (impossible?) scenario of multiple records with the same id
	s.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 2}

	if resp := s.performQuery(); resp.err != nil {
		return resp
	}

	switch s.solr.res.meta.numRecords {
	case 0:
		return searchResponse{status: http.StatusNotFound, err: fmt.Errorf("record not found")}

	case 1:
		return searchResponse{status: http.StatusOK}

	default:
		return searchResponse{status: http.StatusInternalServerError, err: fmt.Errorf("multiple records found")}
	}
}

func (s *searchContext) newSearchWithTopResult(query string) (*searchContext, error) {
	// returns a new search context with the top result of the supplied query
	top := s.copySearchContext()

	// just want first result, not first result group
	top.virgo.flags.groupResults = false

	top.virgo.req.Query = query
	top.virgo.solrQuery = ""
	top.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 1}

	if resp := top.getPoolQueryResults(); resp.err != nil {
		return nil, resp.err
	}

	return top, nil
}

func (s *searchContext) performSpeculativeTitleSearch() (*searchContext, error) {
	// if the original query will include the top result, no need to check it here,
	// as the original query will determine the correct confidence level when it runs.
	// otherwise, query the top result so that we can use its potentially better/more
	// accurate confidence level in the original query's results.

	if s.virgo.req.Pagination.Start != 0 {
		s.log("SEARCH: determining true confidence level for title search")

		return s.newSearchWithTopResult(s.virgo.req.Query)
	}

	return s, nil
}

func (s *searchContext) performSpeculativeSearches() (*searchContext, error) {
	var err error
	var parsedQuery *solrParserInfo

	// maybe facet buckets might differ based on speculative search?
	/*
		// facet-only requests don't need speculation, as the client is only looking for buckets
		if s.virgo.req.Pagination.Rows == 0 {
			return s, nil
		}
	*/

	// parse original query to determine query type

	if parsedQuery, err = s.virgoQueryConvertToSolr(s.virgo.req.Query); err != nil {
		return nil, err
	}

	// single-term title-only search special handling

	if parsedQuery.isSingleTitleSearch == true {
		return s.performSpeculativeTitleSearch()
	}

	// fallthrough: just return original query

	return s, nil
}

func (s *searchContext) newSearchWithRecordCountOnly() (*searchContext, error) {
	// NOTE: groups passed in are quoted strings

	c := s.copySearchContext()

	// just want record count
	c.virgo.flags.groupResults = false
	c.virgo.req.Pagination.Rows = 0

	if resp := c.getPoolQueryResults(); resp.err != nil {
		return nil, resp.err
	}

	return c, nil
}

func (s *searchContext) newSearchWithRecordListForGroups(initialQuery string, groups []string) (*searchContext, error) {
	// NOTE: groups passed in are quoted strings

	c := s.copySearchContext()

	// just want records
	c.virgo.flags.groupResults = false

	// wrap groups for safer querying
	var safeGroups []string

	for _, group := range groups {
		safeGroups = append(safeGroups, strconv.Quote(group))
	}

	// build group-restricted query from initial query
	groupClause := fmt.Sprintf(`%s:(%s)`, s.pool.config.Local.Solr.GroupField, strings.Join(safeGroups, " OR "))

	// prepend existing query, if defined
	newQuery := groupClause
	if initialQuery != "" {
		newQuery = fmt.Sprintf(`(%s) AND (%s)`, initialQuery, groupClause)
	}

	c.virgo.req.Query = ""
	c.virgo.solrQuery = newQuery

	// get everything!  even bible (5000+)
	c.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 100000}

	// intra-group sorting:
	// * inherit sort from original search;
	// * if that sort xid's definition specifies a record sort xid:
	//   * use that sort xid instead; and if it specifies a record sort order; use that instead

	sortOpt := s.virgo.req.Sort

	sortDef := s.pool.maps.definedSorts[sortOpt.SortID]

	if sortDef != nil && sortDef.RecordXID != "" {
		sortOpt.SortID = sortDef.RecordXID

		if sortDef.RecordOrder != "" {
			sortOpt.Order = sortDef.RecordOrder
		}
	}

	//s.log("SORT: intra-group sort: %#v", sortOpt)

	c.virgo.req.Sort = sortOpt

	if resp := c.getPoolQueryResults(); resp.err != nil {
		return nil, resp.err
	}

	return c, nil
}

func (s *searchContext) wrapRecordsInGroups() {
	var groups []v4api.Group

	for _, record := range s.virgo.poolRes.Groups[0].Records {
		group := v4api.Group{
			Value:   "",
			Count:   1,
			Records: []v4api.Record{record},
		}

		groups = append(groups, group)
	}

	s.virgo.poolRes.Groups = groups
}

func (s *searchContext) populateGroups() error {
	// populate record list for each group (i.e. entry in initial record list)
	// by querying for all group records in one request, and plinko'ing the results to the correct groups

	// no need to group facet endpoint results
	if s.virgo.flags.requestFacets == true {
		return nil
	}

	// no need to populate or wrap results when there are none
	if len(s.virgo.poolRes.Groups) == 0 {
		return nil
	}

	// non-grouped results need to be wrapped in groups
	if s.virgo.flags.groupResults == false {
		s.wrapRecordsInGroups()
		return nil
	}

	// image pool only presents first result in UI, but we still want counts.
	// set up to query solr for counts but don't waste time populating anything
	// beyond the first record for each group.
	if s.pool.config.Local.Identity.Mode == "image" {
		s.virgo.flags.firstRecordOnly = true
	}

	// grouped results need to be populated per group value
	var groups []v4api.Group

	var groupValues []string

	groupValueMap := make(map[string]int)

	for i, groupRecord := range s.solr.res.Response.Docs {
		groupValue := s.getSolrGroupFieldValue(&groupRecord)
		groupValues = append(groupValues, groupValue)
		groupValueMap[groupValue] = i
		groups = append(groups, v4api.Group{Value: groupValue, Records: []v4api.Record{}})
	}

	// client might load more than 20 at a time, for instance when reloading a specific page of a search.
	// process groups in batches of 1000 to avoid Solr maxBooleanClause error

	chunks := chunkStrings(groupValues, 1000)

	for _, chunk := range chunks {
		r, err := s.newSearchWithRecordListForGroups(s.virgo.solrQuery, chunk)
		if err != nil {
			return err
		}

		// loop through records to route to correct group

		for i, record := range r.virgo.poolRes.Groups[0].Records {
			groupRecord := r.solr.res.Response.Docs[i]
			groupValue := s.getSolrGroupFieldValue(&groupRecord)
			v := groupValueMap[groupValue]
			groups[v].Records = append(groups[v].Records, record)
		}
	}

	// loop through groups to assign count and fields

	for i := range groups {
		group := &groups[i]
		group.Count = len(group.Records)

		if s.virgo.flags.firstRecordOnly == true {
			group.Records = []v4api.Record{group.Records[0]}
		}
	}

	s.virgo.poolRes.Groups = groups

	// finally replace group counts with record counts

	r, err := s.newSearchWithRecordCountOnly()
	if err != nil {
		return err
	}

	s.virgo.poolRes.Pagination.Total = r.solr.res.meta.totalRows

	return nil
}

func (s *searchContext) validateSearchRequest() error {
	// quick validations we can do up front

	// ensure we received either zero or one filter group,
	// and that any filters provided are supported

	// NOTE: we also set the search-level resource type context here.
	// this primarily controls what facets (if any) are included in the response.
	// if the request contains a single "resource type" facet, we can use that
	// resource type's defined facet list; otherwise we fall back to a default list.

	numFilterGroups := len(s.virgo.req.Filters)

	switch {
	case numFilterGroups > 1:
		return errors.New("received too many filter groups")

	case numFilterGroups == 1:
		// the pool id in the filter group is not associated with anything
		// in our config, so the best we can do is ensure just one filter
		// group was passed, and that it contains filters that we know about

		filterGroup := s.virgo.req.Filters[0]

		// first pass: determine resource type context

		resourceTypeFacets := 0
		resourceType := ""

		for _, filter := range filterGroup.Facets {
			if filter.FacetID == s.pool.config.Global.ResourceTypes.FilterXID {
				resourceTypeFacets++
				resourceType = filter.Value
			}
		}

		if resourceTypeFacets == 1 {
			pool, err := s.getInternalSolrValue(s.pool.config.Global.ResourceTypes.Field, resourceType)
			if err != nil {
				s.warn(err.Error())
			} else {
				s.resourceTypeCtx = s.pool.maps.resourceTypeContexts[pool]
			}
		}

		s.log("VALIDATE: using resource type context [%s] based on selected facets", s.resourceTypeCtx.Value)

		// second pass: ensure filter(s) are present in the resource type context facet list

		s.virgo.invalidFilters = false
		s.virgo.totalFilters = 0

		for _, filter := range filterGroup.Facets {
			if _, rok := s.resourceTypeCtx.filterMap[filter.FacetID]; rok == false {
				s.warn("received known filter [%s] that is not present in resource type context [%s]", filter.FacetID, s.resourceTypeCtx.Value)
				s.virgo.invalidFilters = true
			} else {
				s.virgo.totalFilters++
			}
		}

	default:
		s.log("VALIDATE: using resource type context [%s] by default", s.resourceTypeCtx.Value)
	}

	return nil
}

func (s *searchContext) parseRequest(into interface{}) searchResponse {
	body, err := s.client.ginCtx.GetRawData()
	if err != nil {
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	s.virgo.body = string(body)
	s.log("SEARCH: raw body: [%s]", s.virgo.body)

	dec := json.NewDecoder(bytes.NewReader(body))

	if err = dec.Decode(into); err != nil {
		// "Invalid Request" instead?
		return searchResponse{status: http.StatusBadRequest, err: err}
	}

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) performSearchRequest() searchResponse {
	var err error
	var top *searchContext

	s.log("SEARCH: v4 query: [%s]", s.virgo.req.Query)

	if err = s.validateSearchRequest(); err != nil {
		return searchResponse{status: http.StatusBadRequest, err: err}
	}

	// if request contains invalid filters, set up to return 0 results
	if s.virgo.invalidFilters == true {
		s.virgo.poolRes = &v4api.PoolResult{Confidence: "low"}
	} else {
		// save original request flags
		flags := s.virgo.flags

		if top, err = s.performSpeculativeSearches(); err != nil {
			return searchResponse{status: http.StatusInternalServerError, err: err}
		}

		// use query syntax from chosen search
		s.virgo.req.Query = top.virgo.req.Query

		// restore original request flags
		s.virgo.flags = flags

		// now do the search
		if resp := s.getPoolQueryResults(); resp.err != nil {
			return resp
		}

		// populate group list, if this is a grouped request
		if err = s.populateGroups(); err != nil {
			return searchResponse{status: http.StatusInternalServerError, err: err}
		}

		// restore actual confidence
		if confidenceIndex(top.confidence) > confidenceIndex(s.virgo.poolRes.Confidence) {
			s.log("SEARCH: overriding confidence [%s] with [%s]", s.virgo.poolRes.Confidence, top.confidence)
			s.virgo.poolRes.Confidence = top.confidence
		}
	}

	// add sort info for these results

	s.virgo.poolRes.Sort = s.virgo.req.Sort

	// finally fill out elapsed time

	s.virgo.poolRes.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	return searchResponse{status: http.StatusOK}
}

type facetResponse struct {
	index  int
	facets []v4api.Facet
	resp   searchResponse
}

func (s *searchContext) getFacetResults(index int, channel chan *facetResponse) {
	res := facetResponse{index: index}

	if res.resp = s.getPoolQueryResults(); res.resp.err == nil {
		res.facets = s.virgo.poolRes.FacetList
	}

	channel <- &res
}

func (s *searchContext) performFacetsRequest() ([]v4api.Facet, searchResponse) {
	var err error

	s.log("FACETS: v4 query: [%s]", s.virgo.req.Query)

	if err = s.validateSearchRequest(); err != nil {
		return nil, searchResponse{status: http.StatusBadRequest, err: err}
	}

	// if request contains invalid filters, return supported filters with zero-count values
	if s.virgo.invalidFilters == true {
		// we know there is exactly one filter group at this point.
		// iterate over it and create mappings

		reqMap := make(map[string][]string)

		for _, filter := range s.virgo.req.Filters[0].Facets {
			if _, rok := s.resourceTypeCtx.filterMap[filter.FacetID]; rok == true {
				reqMap[filter.FacetID] = append(reqMap[filter.FacetID], filter.Value)
			}
		}

		// build facet response

		var facetList []v4api.Facet

		for _, xid := range s.resourceTypeCtx.filterXIDs {
			vals := reqMap[xid]

			if len(vals) == 0 {
				continue
			}

			facetDef := s.resourceTypeCtx.filterMap[xid]
			facet := s.newFacetFromDefinition(facetDef)

			for _, val := range vals {
				facet.Buckets = append(facet.Buckets, v4api.FacetBucket{Value: val, Count: 0, Selected: true})
			}

			facetList = append(facetList, facet)
		}

		return facetList, searchResponse{status: http.StatusOK}
	}

	// pre-populate solr query so that parallel facet requests don't waste time duplicating this work

	if resp := s.populateSolrQuery(); resp.err != nil {
		return nil, resp
	}

	// short-circuit: empty/* single-keyword searches with no filters in the request
	// can simply use cached filters.  if errors encountered, just fall back to lookups.

	if s.virgo.totalFilters == 0 {
		// parse original query to determine query type

		if s.virgo.parserInfo.isSingleKeywordSearch == true {
			keyword := s.virgo.parserInfo.keywords[0]
			if keyword == "" || keyword == "*" {
				if filters, fErr := s.pool.localFacetCache.getLocalizedFilters(s.client, s.resourceTypeCtx.filterXIDs); fErr == nil {
					s.log("FACETS: keyword * query using facet cache for response")
					return filters, searchResponse{status: http.StatusOK}
				}
			}
		}
	}

	// for each filter, request solr facets for that filter by applying all current
	// filters EXCEPT those of its own type.  combine these into full filter response.

	// run facet searches in parallel

	channel := make(chan *facetResponse)
	facetRequests := 0

	for i := range s.resourceTypeCtx.filters {
		filter := s.resourceTypeCtx.filters[i]

		// if this is a hidden filter, only return it if it was part of the request
		filterDef := s.pool.maps.definedFilters[filter.XID]
		if filterDef.Hidden == true {
			filterRequested := false
			for _, filterGroup := range s.virgo.req.Filters {
				for _, filterFacet := range filterGroup.Facets {
					if filterFacet.FacetID == filter.XID {
						filterRequested = true
					}
				}
			}

			if filterRequested == false {
				continue
			}
		}

		f := s.copySearchContext()
		f.virgo.solrQuery = s.virgo.solrQuery
		f.virgo.parserInfo = s.virgo.parserInfo
		f.virgo.currentFacet = filter.XID
		facetRequests++
		go f.getFacetResults(i, channel)
	}

	// collect responses

	var facetResps []*facetResponse

	for facetRequests > 0 {
		facetResp := <-channel
		facetResps = append(facetResps, facetResp)
		facetRequests--
	}

	// sort responses

	sort.Slice(facetResps, func(i, j int) bool {
		return facetResps[i].index < facetResps[j].index
	})

	// process responses

	var facetList []v4api.Facet

	for i := range facetResps {
		facetResp := facetResps[i]

		// just return first error encountered
		if facetResp.resp.err != nil {
			return nil, facetResp.resp
		}

		// don't return filters if they have no values (there will be at most one filter)
		if len(facetResp.facets) == 0 || len(facetResp.facets[0].Buckets) == 0 {
			continue
		}

		facetList = append(facetList, facetResp.facets[0])
	}

	return facetList, searchResponse{status: http.StatusOK}
}

func (s *searchContext) determineSortOptions() searchResponse {
	// determine if specified sort and order is valid, or if we should use a default

	sortOpt := v4api.SortOrder{
		SortID: s.pool.config.Global.Service.DefaultSort.XID,
		Order:  s.pool.config.Global.Service.DefaultSort.Order,
	}

	sortReq := s.virgo.req.Sort

	if sortReq.SortID != "" || sortReq.Order != "" {
		// sort was specified; validate it
		sortDef := s.pool.maps.definedSorts[sortReq.SortID]

		if sortDef.XID == "" {
			return searchResponse{status: http.StatusBadRequest, err: errors.New("invalid sort id")}
		}

		if isValidSortOrder(sortReq.Order) == false {
			return searchResponse{status: http.StatusBadRequest, err: errors.New("invalid sort order")}
		}

		if sortDef.Order != "" && sortReq.Order != sortDef.Order {
			return searchResponse{status: http.StatusBadRequest, err: errors.New("sort order not valid for this sort id")}
		}

		sortOpt = s.virgo.req.Sort
	}

	s.virgo.req.Sort = sortOpt

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) handleSearchRequest() searchResponse {
	s.virgo.endpoint = "search"

	var errData v4api.PoolResult

	if resp := s.parseRequest(&s.virgo.req); resp.err != nil {
		errData = v4api.PoolResult{StatusCode: resp.status, StatusMessage: resp.err.Error()}
		resp.data = errData
		return resp
	}

	s.virgo.flags.requestFacets = false

	if resp := s.determineSortOptions(); resp.err != nil {
		errData = v4api.PoolResult{StatusCode: resp.status, StatusMessage: resp.err.Error()}
		resp.data = errData
		return resp
	}

	// group or not based on sort being applied
	s.virgo.flags.groupResults = s.pool.maps.definedSorts[s.virgo.req.Sort.SortID].GroupResults

	if resp := s.performSearchRequest(); resp.err != nil {
		errData = v4api.PoolResult{StatusCode: resp.status, StatusMessage: resp.err.Error()}
		resp.data = errData
		return resp
	}

	s.virgo.poolRes.FacetList = []v4api.Facet{}
	s.virgo.poolRes.StatusCode = http.StatusOK

	return searchResponse{status: http.StatusOK, data: s.virgo.poolRes}
}

func (s *searchContext) handleFacetsRequest() searchResponse {
	s.virgo.endpoint = "facets"

	var errData v4api.PoolFacets

	if resp := s.parseRequest(&s.virgo.req); resp.err != nil {
		errData = v4api.PoolFacets{StatusCode: resp.status, StatusMessage: resp.err.Error()}
		resp.data = errData
		return resp
	}

	// override these values from the original search query, since we are
	// only interested in facets, not records

	s.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 0}
	s.virgo.flags.requestFacets = true

	facets, resp := s.performFacetsRequest()

	if resp.err != nil {
		errData = v4api.PoolFacets{StatusCode: resp.status, StatusMessage: resp.err.Error()}
		resp.data = errData
		return resp
	}

	s.virgo.facetsRes = &v4api.PoolFacets{
		FacetList:  facets,
		ElapsedMS:  int64(time.Since(s.client.start) / time.Millisecond),
		StatusCode: http.StatusOK,
	}

	return searchResponse{status: http.StatusOK, data: s.virgo.facetsRes}
}

func (s *searchContext) handleFiltersRequest() searchResponse {
	filters, err := s.pool.globalFacetCache.getPreSearchFilters()

	if err != nil {
		resp := searchResponse{status: http.StatusServiceUnavailable, err: err}
		resp.data = v4api.PoolFacets{StatusCode: resp.status, StatusMessage: resp.err.Error()}
		return resp
	}

	s.virgo.facetsRes = &v4api.PoolFacets{
		FacetList:  filters,
		ElapsedMS:  0,
		StatusCode: http.StatusOK,
	}

	return searchResponse{status: http.StatusOK, data: s.virgo.facetsRes}
}

func (s *searchContext) handleRecordRequest() searchResponse {
	s.virgo.endpoint = "resource"

	// fill out Solr query directly, bypassing query syntax parser
	s.virgo.solrQuery = fmt.Sprintf(`id:"%s"`, s.client.ginCtx.Param("id"))
	s.virgo.flags.groupResults = false

	// mark this as a resource request
	s.itemDetails = true

	// override these values from defaults.  specify two rows to catch
	// the (impossible?) scenario of multiple records with the same id
	s.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 2}

	if resp := s.getRecordQueryResults(); resp.err != nil {
		return resp
	}

	// per-mode tweaks to this record
	switch s.pool.config.Local.Identity.Mode {
	case "image":
		record := &s.solr.res.Response.Docs[0]

		group := s.getSolrGroupFieldValue(record)
		groupValues := []string{group}

		r, err := s.newSearchWithRecordListForGroups("", groupValues)
		if err != nil {
			break
		}

		// put related values in a separate section of the record response

		var related []v4api.RelatedRecord

		for _, doc := range r.solr.res.Response.Docs {
			rr := v4api.RelatedRecord{
				ID:              doc.getFirstString(s.pool.config.Local.Related.Image.IDField),
				IIIFManifestURL: doc.getFirstString(s.pool.config.Local.Related.Image.IIIFManifestField),
				IIIFImageURL:    doc.getFirstString(s.pool.config.Local.Related.Image.IIIFImageField),
			}

			related = append(related, rr)
		}

		s.virgo.recordRes.Related = related
	}

	return searchResponse{status: http.StatusOK, data: s.virgo.recordRes}
}

func (s *searchContext) handlePingRequest() searchResponse {
	s.virgo.endpoint = "ping"

	if err := s.solrPing(); err != nil {
		s.err("ping execution error: %s", err.Error())
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	return searchResponse{status: http.StatusOK}
}
