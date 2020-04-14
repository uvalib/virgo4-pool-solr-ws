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
)

type searchContext struct {
	pool           *poolContext
	client         *clientContext
	virgoReq       VirgoSearchRequest
	virgoPoolRes   *VirgoPoolResult
	virgoRecordRes *VirgoRecord
	solrReq        *solrRequest
	solrRes        *solrResponse
	confidence     string
	itemDetails    bool
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

	s.virgoReq.meta.client = s.client
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

	v := s.virgoReq
	sc.virgoReq = v

	return sc
}

func (s *searchContext) log(format string, args ...interface{}) {
	s.client.log(format, args...)
}

func (s *searchContext) err(format string, args ...interface{}) {
	s.client.err(format, args...)
}

func (s *searchContext) performQuery() searchResponse {
	if resp := s.solrSearchRequest(); resp.err != nil {
		s.err("query creation error: %s", resp.err.Error())
		return resp
	}

	if err := s.solrQuery(); err != nil {
		s.err("query execution error: %s", err.Error())
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) getPoolQueryResults() searchResponse {
	if resp := s.performQuery(); resp.err != nil {
		return resp
	}

	if err := s.virgoSearchResponse(); err != nil {
		s.err("result parsing error: %s", err.Error())
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	s.confidence = s.virgoPoolRes.Confidence

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) getRecordQueryResults() searchResponse {
	var err error

	if resp := s.performQuery(); resp.err != nil {
		return resp
	}

	if err = s.virgoRecordResponse(); err != nil {
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

	top.virgoReq.Query = query
	top.virgoReq.meta.solrQuery = ""
	top.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 1}

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

	if confidenceIndex(b.results[i].virgoPoolRes.Confidence) < confidenceIndex(b.results[j].virgoPoolRes.Confidence) {
		return false
	}

	if confidenceIndex(b.results[i].virgoPoolRes.Confidence) > confidenceIndex(b.results[j].virgoPoolRes.Confidence) {
		return true
	}

	// confidence is equal; sort by score

	return b.results[i].solrRes.meta.maxScore > b.results[j].solrRes.meta.maxScore
}

func (s *searchContext) performSpeculativeTitleSearch() (*searchContext, error) {
	// if the original query will include the top result, no need to check it here,
	// as the original query will determine the correct confidence level when it runs.
	// otherwise, query the top result so that we can use its potentially better/more
	// accurate confidence level in the original query's results.

	if s.virgoReq.Pagination.Start != 0 {
		s.log("TITLE SEARCH: determining true confidence level")

		return s.newSearchWithTopResult(s.virgoReq.Query)
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

	if keyword, err = s.newSearchWithTopResult(s.virgoReq.Query); err != nil {
		return nil, err
	}

	s.log("[INTUIT] keyword: confidence = [%s]  maxScore = [%0.2f]", keyword.virgoPoolRes.Confidence, keyword.solrRes.meta.maxScore)
	searchResults = append(searchResults, keyword)

	if title, err = keyword.newSearchWithTopResult(fmt.Sprintf("title:{%s}", searchTerm)); err == nil {
		s.log("[INTUIT] title: confidence = [%s]  maxScore = [%0.2f]", title.virgoPoolRes.Confidence, title.solrRes.meta.maxScore)
		searchResults = append(searchResults, title)
	}

	if author, err = keyword.newSearchWithTopResult(fmt.Sprintf("author:{%s}", searchTerm)); err == nil {
		s.log("[INTUIT] author: confidence = [%s]  maxScore = [%0.2f]", author.virgoPoolRes.Confidence, author.solrRes.meta.maxScore)
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
		if s.virgoReq.Pagination.Rows == 0 {
			return s, nil
		}
	*/

	// parse original query to determine query type

	if parsedQuery, err = virgoQueryConvertToSolr(s.virgoReq.Query); err != nil {
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
	groupClause := fmt.Sprintf(`%s:(%s)`, s.pool.config.Solr.GroupField, strings.Join(safeGroups, " OR "))

	// prepend existing query, if defined
	newQuery := groupClause
	if initialQuery != "" {
		newQuery = fmt.Sprintf(`%s AND %s`, initialQuery, groupClause)
	}

	c.virgoReq.Query = ""
	c.virgoReq.meta.solrQuery = newQuery
	c.virgoReq.meta.requestFacets = false

	// get everything!  even bible (5000+)
	c.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 100000}

	if resp := c.getPoolQueryResults(); resp.err != nil {
		return nil, resp.err
	}

	return c, nil
}

func (s *searchContext) wrapRecordsInGroups() {
	// non-grouped results with no records don't need wrapping
	if s.virgoPoolRes.RecordList == nil {
		return
	}

	var groups VirgoGroups

	for _, record := range *s.virgoPoolRes.RecordList {
		group := VirgoGroup{
			Value:      "",
			Count:      1,
			RecordList: []VirgoRecord{record},
		}

		groups = append(groups, group)
	}

	s.virgoPoolRes.GroupList = &groups

	// remove record list, as results are now in groups
	s.virgoPoolRes.RecordList = nil
}

func (s *searchContext) populateGroups() error {
	// populate record list for each group (i.e. entry in initial record list)
	// by querying for all group records in one request, and plinko'ing the results to the correct groups

	// no need to group facet endpoint results
	if s.virgoReq.meta.requestFacets == true {
		return nil
	}

	// non-grouped results need to be wrapped in groups
	if s.client.opts.grouped == false {
		s.wrapRecordsInGroups()
		return nil
	}

	// grouped results with no results don't need populating
	if s.solrRes.meta.numGroups == 0 {
		return nil
	}

	var groups VirgoGroups

	var groupValues []string

	groupValueMap := make(map[string]int)

	for i, groupRecord := range s.solrRes.Response.Docs {
		groupValue := s.getSolrGroupFieldValue(&groupRecord)
		groupValues = append(groupValues, groupValue)
		groupValueMap[groupValue] = i
		var records VirgoRecords
		groups = append(groups, VirgoGroup{Value: groupValue, RecordList: records})
	}

	r, err := s.newSearchWithRecordListForGroups(s.virgoReq.meta.solrQuery, groupValues)
	if err != nil {
		return err
	}

	// loop through records to route to correct group

	start := time.Now()
	for _, record := range *r.virgoPoolRes.RecordList {
		v := groupValueMap[record.groupValue]
		groups[v].RecordList = append(groups[v].RecordList, record)
	}
	s.log("[GROUP] map groups: %5d ms", int64(time.Since(start)/time.Millisecond))

	// loop through groups to assign count and fields

	start = time.Now()
	for i := range groups {
		group := &groups[i]

		group.Count = len(group.RecordList)

		// cover image url
		// just use url from first grouped result that has one
		// (they all will for now, but we check properly anyway)

		gotCover := false

		for _, r := range group.RecordList {
			for _, f := range r.Fields {
				if f.Name == "cover_image" {
					group.addField(&f)
					gotCover = true
					break
				}
			}

			if gotCover == true {
				break
			}
		}

	}
	s.log("[GROUP] group vals: %5d ms", int64(time.Since(start)/time.Millisecond))

	s.virgoPoolRes.GroupList = &groups

	// remove record list, as results are now in groups
	s.virgoPoolRes.RecordList = nil

	return nil
}

func (s *searchContext) validateSearchRequest() error {
	// quick validations we can do up front

	// ensure we received either zero or one filter group,
	// and that any filters provided are supported

	if s.virgoReq.Filters != nil {
		numFilterGroups := len(*s.virgoReq.Filters)

		switch {
		case numFilterGroups > 1:
			return errors.New("received too many filter groups")

		case numFilterGroups == 1:
			availableFacets := s.solrAvailableFacets()

			filterGroup := (*s.virgoReq.Filters)[0]

			s.log("received filter group: [%s]", filterGroup.PoolID)

			for _, filter := range filterGroup.Facets {
				if _, ok := availableFacets[filter.FacetID]; ok == false {
					return fmt.Errorf("received unrecognized filter: [%s]", filter.FacetID)
				}
			}
		}
	}

	return nil
}

func (s *searchContext) handleSearchOrFacetsRequest(c *gin.Context) searchResponse {
	var err error
	var top *searchContext

	if err = c.BindJSON(&s.virgoReq); err != nil {
		// "Invalid Request" instead?
		return searchResponse{status: http.StatusBadRequest, err: err}
	}

	s.log("query: [%s]", s.virgoReq.Query)

	if err = s.validateSearchRequest(); err != nil {
		return searchResponse{status: http.StatusBadRequest, err: err}
	}

	// save original facet request flag
	requestFacets := s.virgoReq.meta.requestFacets

	if top, err = s.performSpeculativeSearches(); err != nil {
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	// use query syntax from chosen search
	s.virgoReq.Query = top.virgoReq.Query

	// restore original facet request flag
	s.virgoReq.meta.requestFacets = requestFacets

	// now do the search
	if resp := s.getPoolQueryResults(); resp.err != nil {
		return resp
	}

	// populate group list, if this is a grouped request
	if err = s.populateGroups(); err != nil {
		return searchResponse{status: http.StatusInternalServerError, err: err}
	}

	// restore actual confidence
	if confidenceIndex(top.confidence) > confidenceIndex(s.virgoPoolRes.Confidence) {
		s.log("overriding confidence [%s] with [%s]", s.virgoPoolRes.Confidence, top.confidence)
		s.virgoPoolRes.Confidence = top.confidence
	}

	// add sort info for these results

	s.virgoPoolRes.Sort = &s.solrReq.meta.sort

	// finally fill out elapsed time

	s.virgoPoolRes.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) handleSearchRequest(c *gin.Context) searchResponse {
	if resp := s.handleSearchOrFacetsRequest(c); resp.err != nil {
		return resp
	}

	s.virgoPoolRes.FacetList = nil

	return searchResponse{status: http.StatusOK, data: s.virgoPoolRes}
}

func (s *searchContext) handleFacetsRequest(c *gin.Context) searchResponse {
	// override these values from the original search query, since we are
	// only interested in facets, not records
	s.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 0}

	s.virgoReq.meta.requestFacets = true

	if resp := s.handleSearchOrFacetsRequest(c); resp.err != nil {
		return resp
	}

	virgoFacetsRes := VirgoFacetsResult{
		FacetList: s.virgoPoolRes.FacetList,
		ElapsedMS: s.virgoPoolRes.ElapsedMS,
	}

	return searchResponse{status: http.StatusOK, data: virgoFacetsRes}
}

func (s *searchContext) handleRecordRequest() searchResponse {
	// override these values from defaults.  specify two rows to catch
	// the (impossible?) scenario of multiple records with the same id
	s.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 2}

	if resp := s.getRecordQueryResults(); resp.err != nil {
		return resp
	}

	// per-mode tweaks to this record
	switch s.pool.config.Identity.Mode {
	case "image":
		record := &s.solrRes.Response.Docs[0]

		group := s.getSolrGroupFieldValue(record)
		groupValues := []string{group}

		r, err := s.newSearchWithRecordListForGroups("", groupValues)
		if err != nil {
			break
		}

		// put related values in a separate section of the record response

		var related VirgoRelatedRecords

		for _, doc := range r.solrRes.Response.Docs {
			rr := VirgoRelatedRecord{
				ID:              firstElementOf(doc.getValuesByTag(s.pool.config.Related.Image.IDField)),
				IIIFManifestURL: firstElementOf(doc.getValuesByTag(s.pool.config.Related.Image.IIIFManifestField)),
				IIIFImageURL:    firstElementOf(doc.getValuesByTag(s.pool.config.Related.Image.IIIFImageField)),
				IIIFBaseURL:     getIIIFBaseURL(&doc, s.pool.config.Related.Image.IdentifierField),
			}

			related = append(related, rr)
		}

		s.virgoRecordRes.Related = &related
	}

	return searchResponse{status: http.StatusOK, data: s.virgoRecordRes}
}

func (s *searchContext) handlePingRequest() searchResponse {
	// override these values from defaults.  we are not interested
	// in records, just connectivity
	s.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 0}

	if resp := s.performQuery(); resp.err != nil {
		return resp
	}

	return searchResponse{status: http.StatusOK}
}
