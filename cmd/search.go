package main

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uvalib/virgo4-api/v4api"
)

type virgoDialog struct {
	req           v4api.SearchRequest
	poolRes       *v4api.PoolResult
	facetsRes     *v4api.PoolFacets
	recordRes     *v4api.Record
	solrQuery     string          // holds the solr query (either parsed or specified)
	parserInfo    *solrParserInfo // holds the information for parsed queries
	requestFacets bool            // set to true for non-speculative searches
}

type solrDialog struct {
	req    *solrRequest
	res    *solrResponse
	client *http.Client // points to appropriate http client
}

type searchContext struct {
	pool        *poolContext
	client      *clientContext
	virgo       virgoDialog
	solr        solrDialog
	confidence  string
	itemDetails bool
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
	s.solr.client = p.solr.serviceClient
}

func (s *searchContext) copySearchContext() *searchContext {
	// performs a copy somewhere between shallow and deep
	// (just enough to let this context be used for another search
	// without potentially clobbering the original context by leaving
	// pointers into the original context)

	sc := &searchContext{}

	sc.pool = s.pool
	sc.solr.client = s.solr.client

	// copy client (modified for speculative searches)
	c := *s.client
	sc.client = &c

	v := s.virgo.req
	sc.virgo.req = v

	return sc
}

func (s *searchContext) log(format string, args ...interface{}) {
	s.client.log(format, args...)
}

func (s *searchContext) err(format string, args ...interface{}) {
	s.client.err(format, args...)
}

func (s *searchContext) performQuery() searchResponse {
	s.log("**********  START SOLR QUERY  **********")

	if resp := s.solrSearchRequest(); resp.err != nil {
		s.err("query creation error: %s", resp.err.Error())
		return resp
	}

	err := s.solrQuery()

	s.log("**********   END SOLR QUERY   **********")

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

	if err := s.buildPoolSearchResponse(); err != nil {
		s.err("result parsing error: %s", err.Error())
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	s.confidence = s.virgo.poolRes.Confidence

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) getRecordQueryResults() searchResponse {
	var err error

	if resp := s.performQuery(); resp.err != nil {
		return resp
	}

	if err = s.buildPoolRecordResponse(); err != nil {
		s.err("result parsing error: %s", err.Error())
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) newSearchWithTopResult(query string) (*searchContext, error) {
	// returns a new search context with the top result of the supplied query
	top := s.copySearchContext()

	// just want first result, not first result group
	top.client.opts.grouped = false

	top.virgo.req.Query = query
	top.virgo.solrQuery = ""
	top.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 1}

	if resp := top.getPoolQueryResults(); resp.err != nil {
		return nil, resp.err
	}

	return top, nil
}

type byConfidence struct {
	results []*searchContext
}

func (b *byConfidence) Len() int {
	return len(b.results)
}

func (b *byConfidence) Swap(i, j int) {
	b.results[i], b.results[j] = b.results[j], b.results[i]
}

func (b *byConfidence) Less(i, j int) bool {
	// sort by confidence index

	if confidenceIndex(b.results[i].virgo.poolRes.Confidence) < confidenceIndex(b.results[j].virgo.poolRes.Confidence) {
		return false
	}

	if confidenceIndex(b.results[i].virgo.poolRes.Confidence) > confidenceIndex(b.results[j].virgo.poolRes.Confidence) {
		return true
	}

	// confidence is equal; sort by score

	return b.results[i].solr.res.meta.maxScore > b.results[j].solr.res.meta.maxScore
}

func (s *searchContext) performSpeculativeTitleSearch() (*searchContext, error) {
	// if the original query will include the top result, no need to check it here,
	// as the original query will determine the correct confidence level when it runs.
	// otherwise, query the top result so that we can use its potentially better/more
	// accurate confidence level in the original query's results.

	if s.virgo.req.Pagination.Start != 0 {
		s.log("[SEARCH] determining true confidence level for title search")

		return s.newSearchWithTopResult(s.virgo.req.Query)
	}

	return s, nil
}

func (s *searchContext) performSpeculativeKeywordSearch(origSearchTerm string) (*searchContext, error) {
	var err error
	var title, author, keyword *searchContext
	var searchResults []*searchContext

	// if the client specified no intuition, just return original query
	if s.client.opts.intuit != true {
		return s, nil
	}

	// first, unescape the search term (which may have been escaped by the query parser)

	searchTerm := origSearchTerm

	var unquotedSearchTerm string
	if unquotedSearchTerm, err = strconv.Unquote(fmt.Sprintf(`"%s"`, origSearchTerm)); err == nil {
		searchTerm = unquotedSearchTerm
	}

	s.log("[INTUIT] KEYWORD SEARCH: intuiting best search for [%s]", searchTerm)

	s.log("[INTUIT] checking if keyword search term [%s] might be a title or author search...", searchTerm)

	if keyword, err = s.newSearchWithTopResult(s.virgo.req.Query); err != nil {
		return nil, err
	}

	s.log("[INTUIT] keyword: confidence = [%s]  maxScore = [%0.2f]", keyword.virgo.poolRes.Confidence, keyword.solr.res.meta.maxScore)
	searchResults = append(searchResults, keyword)

	if title, err = keyword.newSearchWithTopResult(fmt.Sprintf("title:{%s}", searchTerm)); err == nil {
		s.log("[INTUIT] title: confidence = [%s]  maxScore = [%0.2f]", title.virgo.poolRes.Confidence, title.solr.res.meta.maxScore)
		searchResults = append(searchResults, title)
	}

	if author, err = keyword.newSearchWithTopResult(fmt.Sprintf("author:{%s}", searchTerm)); err == nil {
		s.log("[INTUIT] author: confidence = [%s]  maxScore = [%0.2f]", author.virgo.poolRes.Confidence, author.solr.res.meta.maxScore)
		searchResults = append(searchResults, author)
	}

	confidenceSort := byConfidence{results: searchResults}
	sort.Sort(&confidenceSort)

	return confidenceSort.results[0], nil
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

	if parsedQuery, err = virgoQueryConvertToSolr(s.virgo.req.Query); err != nil {
		return nil, err
	}

	// single-term title-only search special handling

	if parsedQuery.isSingleTitleSearch == true {
		return s.performSpeculativeTitleSearch()
	}

	// single-term keyword search special handling

	if parsedQuery.isSingleKeywordSearch == true {
		return s.performSpeculativeKeywordSearch(firstElementOf(parsedQuery.keywords))
	}

	// fallthrough: just return original query

	return s, nil
}

func (s *searchContext) newSearchWithRecordListForGroups(initialQuery string, groups []string) (*searchContext, error) {
	// NOTE: groups passed in are quoted strings

	c := s.copySearchContext()

	// just want records
	c.client.opts.grouped = false

	// wrap groups for safer querying
	var safeGroups []string

	for _, group := range groups {
		safeGroups = append(safeGroups, strconv.Quote(group))
	}

	// build group-restricted query from initial query
	groupClause := fmt.Sprintf(`%s:(%s)`, s.pool.config.Local.Solr.Grouping.Field, strings.Join(safeGroups, " OR "))

	// prepend existing query, if defined
	newQuery := groupClause
	if initialQuery != "" {
		newQuery = fmt.Sprintf(`%s AND %s`, initialQuery, groupClause)
	}

	c.virgo.req.Query = ""
	c.virgo.solrQuery = newQuery
	c.virgo.requestFacets = false

	// get everything!  even bible (5000+)
	c.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 100000}

	// intra-group sorting
	c.virgo.req.Sort = v4api.SortOrder{
		SortID: s.pool.config.Local.Solr.Grouping.Sort.XID,
		Order:  s.pool.config.Local.Solr.Grouping.Sort.Order,
	}

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
	if s.virgo.requestFacets == true {
		return nil
	}

	// no need to populate or wrap results when there are none
	if len(s.virgo.poolRes.Groups) == 0 {
		return nil
	}

	// non-grouped results need to be wrapped in groups
	if s.client.opts.grouped == false {
		s.wrapRecordsInGroups()
		return nil
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

	r, err := s.newSearchWithRecordListForGroups(s.virgo.solrQuery, groupValues)
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

	// loop through groups to assign count and fields

	for i := range groups {
		group := &groups[i]
		group.Count = len(group.Records)
	}

	s.virgo.poolRes.Groups = groups

	return nil
}

func (s *searchContext) validateSearchRequest() error {
	// quick validations we can do up front

	// ensure we received either zero or one filter group,
	// and that any filters provided are supported

	numFilterGroups := len(s.virgo.req.Filters)

	switch {
	case numFilterGroups > 1:
		return errors.New("received too many filter groups")

	case numFilterGroups == 1:
		// the pool id in the filter group is not associated with anything
		// in our config, so the best we can do is ensure just one filter
		// group was passed, and that it contains filters that we know about

		filterGroup := s.virgo.req.Filters[0]

		availableFacets := s.solrAvailableFacets()

		for _, filter := range filterGroup.Facets {
			if _, ok := availableFacets[filter.FacetID]; ok == false {
				return fmt.Errorf("received unrecognized filter: [%s]", filter.FacetID)
			}
		}
	}

	return nil
}

func (s *searchContext) handleSearchOrFacetsRequest(c *gin.Context) searchResponse {
	var err error
	var top *searchContext

	if err = c.BindJSON(&s.virgo.req); err != nil {
		// "Invalid Request" instead?
		return searchResponse{status: http.StatusBadRequest, err: err}
	}

	s.log("[SEARCH] query: [%s]", s.virgo.req.Query)

	if err = s.validateSearchRequest(); err != nil {
		return searchResponse{status: http.StatusBadRequest, err: err}
	}

	// save original facet request flag
	requestFacets := s.virgo.requestFacets

	if top, err = s.performSpeculativeSearches(); err != nil {
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	// use query syntax from chosen search
	s.virgo.req.Query = top.virgo.req.Query

	// restore original facet request flag
	s.virgo.requestFacets = requestFacets

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
		s.log("[SEARCH] overriding confidence [%s] with [%s]", s.virgo.poolRes.Confidence, top.confidence)
		s.virgo.poolRes.Confidence = top.confidence
	}

	// add sort info for these results

	s.virgo.poolRes.Sort = s.solr.req.meta.sort

	// finally fill out elapsed time

	s.virgo.poolRes.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) handleSearchRequest(c *gin.Context) searchResponse {
	if resp := s.handleSearchOrFacetsRequest(c); resp.err != nil {
		resp.data = v4api.PoolResult{StatusCode: resp.status, StatusMessage: resp.err.Error()}
		return resp
	}

	s.virgo.poolRes.FacetList = []v4api.Facet{}
	s.virgo.poolRes.StatusCode = http.StatusOK

	return searchResponse{status: http.StatusOK, data: s.virgo.poolRes}
}

func (s *searchContext) handleFacetsRequest(c *gin.Context) searchResponse {
	// override these values from the original search query, since we are
	// only interested in facets, not records
	s.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 0}

	s.virgo.requestFacets = true

	if resp := s.handleSearchOrFacetsRequest(c); resp.err != nil {
		resp.data = v4api.PoolFacets{StatusCode: resp.status, StatusMessage: resp.err.Error()}
		return resp
	}

	s.virgo.facetsRes = &v4api.PoolFacets{
		FacetList:  s.virgo.poolRes.FacetList,
		ElapsedMS:  s.virgo.poolRes.ElapsedMS,
		StatusCode: http.StatusOK,
	}

	return searchResponse{status: http.StatusOK, data: s.virgo.facetsRes}
}

func (s *searchContext) handleRecordRequest() searchResponse {
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
				ID:              firstElementOf(doc.getValuesByTag(s.pool.config.Local.Related.Image.IDField)),
				IIIFManifestURL: firstElementOf(doc.getValuesByTag(s.pool.config.Local.Related.Image.IIIFManifestField)),
				IIIFImageURL:    firstElementOf(doc.getValuesByTag(s.pool.config.Local.Related.Image.IIIFImageField)),
				IIIFBaseURL:     s.getIIIFBaseURL(&doc, s.pool.config.Local.Related.Image.IdentifierField),
			}

			related = append(related, rr)
		}

		s.virgo.recordRes.Related = related
	}

	return searchResponse{status: http.StatusOK, data: s.virgo.recordRes}
}

func (s *searchContext) handlePingRequest() searchResponse {
	s.solr.client = s.pool.solr.healthcheckClient

	// override these values from defaults.  we are not interested
	// in records, just connectivity
	s.virgo.req.Pagination = v4api.Pagination{Start: 0, Rows: 0}

	if resp := s.performQuery(); resp.err != nil {
		return resp
	}

	return searchResponse{status: http.StatusOK}
}
