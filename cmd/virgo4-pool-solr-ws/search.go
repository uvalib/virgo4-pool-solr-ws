package main

import (
	"fmt"
	"sort"
)

const defaultStart = 0
const minimumStart = 0

const defaultRows = 10
const minimumRows = 0

type searchContext struct {
	pool           *poolContext
	client         *clientOptions
	virgoReq       VirgoSearchRequest
	virgoPoolRes   *VirgoPoolResult
	virgoRecordRes *VirgoRecord
	solrReq        *solrRequest
	solrRes        *solrResponse
	confidence     string
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

func (s *searchContext) init(p *poolContext, c *clientOptions) {
	s.pool = p

	s.client = c

	s.virgoReq.meta.client = s.client
	s.virgoReq.Pagination.Start = defaultStart
	s.virgoReq.Pagination.Rows = defaultRows
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

func (s *searchContext) performQuery() error {
	var err error

	if err = s.solrSearchRequest(); err != nil {
		s.err("query creation error: %s", err.Error())
		return err
	}

	if err = s.solrQuery(); err != nil {
		s.err("query execution error: %s", err.Error())
		return err
	}

	return nil
}

func (s *searchContext) getPoolQueryResults() error {
	var err error

	if err = s.performQuery(); err != nil {
		return err
	}

	if err = s.virgoSearchResponse(); err != nil {
		s.err("result parsing error: %s", err.Error())
		return err
	}

	s.confidence = s.virgoPoolRes.Confidence

	return nil
}

func (s *searchContext) getRecordQueryResults() error {
	var err error

	if err = s.performQuery(); err != nil {
		return err
	}

	if err = s.virgoRecordResponse(); err != nil {
		s.err("result parsing error: %s", err.Error())
		return err
	}

	return nil
}

func (s *searchContext) newSearchWithTopResult(query string) (*searchContext, error) {
	// returns a new search context with the top result of the supplied query
	top := s.copySearchContext()

	// just want first result, not first result group
	top.client.grouped = false

	top.virgoReq.Query = query
	top.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 1}

	if err := top.getPoolQueryResults(); err != nil {
		return nil, err
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

func (s *searchContext) performSpeculativeKeywordSearch(searchTerm string) (*searchContext, error) {
	var err error
	var title, author, keyword *searchContext
	var searchResults []*searchContext

	// if the client specified no intuition, just return original query
	if s.client.intuit != true {
		return s, nil
	}

	s.log("KEYWORD SEARCH: intuiting best search for [%s]", searchTerm)

	s.log("checking if keyword search term [%s] might be a title or author search...", searchTerm)

	if keyword, err = s.newSearchWithTopResult(s.virgoReq.Query); err != nil {
		return nil, err
	}

	s.log("keyword: confidence = [%s]  maxScore = [%0.2f]", keyword.virgoPoolRes.Confidence, keyword.solrRes.meta.maxScore)
	searchResults = append(searchResults, keyword)

	if title, err = keyword.newSearchWithTopResult(fmt.Sprintf("title:{%s}", searchTerm)); err == nil {
		s.log("title: confidence = [%s]  maxScore = [%0.2f]", title.virgoPoolRes.Confidence, title.solrRes.meta.maxScore)
		searchResults = append(searchResults, title)
	}

	if author, err = keyword.newSearchWithTopResult(fmt.Sprintf("author:{%s}", searchTerm)); err == nil {
		s.log("author: confidence = [%s]  maxScore = [%0.2f]", author.virgoPoolRes.Confidence, author.solrRes.meta.maxScore)
		searchResults = append(searchResults, author)
	}

	confidenceSort := byConfidence{results: searchResults}
	sort.Sort(&confidenceSort)

	return confidenceSort.results[0], nil
}

func (s *searchContext) performSpeculativeSearches() (*searchContext, error) {
	var err error
	var parsedQuery *solrParserInfo

	// facet-only requests don't need speculation, as the client is only looking for buckets
	if s.virgoReq.Pagination.Rows == 0 {
		return s, nil
	}

	// parse original query to determine query type

	if parsedQuery, err = virgoQueryConvertToSolr(s.virgoReq.Query); err != nil {
		return nil, err
	}

	// single-term title-only search special handling

	if parsedQuery.isTitleSearch == true {
		return s.performSpeculativeTitleSearch()
	}

	// single-term keyword search special handling

	if parsedQuery.isKeywordSearch == true {
		return s.performSpeculativeKeywordSearch(firstElementOf(parsedQuery.parser.Keywords))
	}

	// fallthrough: just return original query

	return s, nil
}

func (s *searchContext) newSearchWithRecordListForGroup(group string) (*searchContext, error) {
	c := s.copySearchContext()

	// just want records
	c.client.grouped = false

	c.virgoReq.meta.solrQuery = fmt.Sprintf(`%s AND %s:"%s"`, c.virgoReq.meta.solrQuery, s.pool.config.solrGroupField, group)
	c.virgoReq.meta.requestFacets = false

	// get everything!  even bible (5000+)
	c.virgoReq.Pagination = VirgoPagination{Start: 0, Rows: 10000}

	if err := c.getPoolQueryResults(); err != nil {
		return nil, err
	}

	return c, nil
}

func (s *searchContext) populateGroups() error {
	// populate record list for each group (i.e. entry in initial record list)

	if s.client.grouped == false || s.solrRes.meta.numGroups == 0 {
		return nil
	}

	var groups []VirgoGroup

	for _, g := range s.solrRes.Response.Docs {
		group := VirgoGroup{
			Value: g.WorkTitle2KeySort,
		}

		// perform subsearch for this group's records
		r, err := s.newSearchWithRecordListForGroup(group.Value)
		if err != nil {
			return err
		}

		group.RecordList = *r.virgoPoolRes.RecordList

		// set count (this may be wrong if there are more records than are queried in the subsearch above)
		group.Count = len(group.RecordList)

		groups = append(groups, group)
	}

	s.virgoPoolRes.GroupList = &groups

	// remove record list, as results are now in groups
	s.virgoPoolRes.RecordList = nil

	return nil
}

func (s *searchContext) handleSearchRequest() (*VirgoPoolResult, error) {
	var err error
	var top *searchContext

	if top, err = s.performSpeculativeSearches(); err != nil {
		return nil, err
	}

	// use query syntax from chosen search
	s.virgoReq.Query = top.virgoReq.Query

	// set variables for the actual search
	s.virgoReq.meta.requestFacets = true

	if err = s.getPoolQueryResults(); err != nil {
		return nil, err
	}

	// populate group list, if this is a grouped request
	if err = s.populateGroups(); err != nil {
		return nil, err
	}

	// restore actual confidence
	if confidenceIndex(top.confidence) > confidenceIndex(s.virgoPoolRes.Confidence) {
		s.log("overriding confidence [%s] with [%s]", s.virgoPoolRes.Confidence, top.confidence)
		s.virgoPoolRes.Confidence = top.confidence
	}

	return s.virgoPoolRes, nil
}

func (s *searchContext) handleRecordRequest() (*VirgoRecord, error) {
	var err error

	if err = s.getRecordQueryResults(); err != nil {
		return nil, err
	}

	return s.virgoRecordRes, nil
}

func (s *searchContext) handlePingRequest() error {
	var err error

	if err = s.solrRecordRequest(); err != nil {
		s.err("query creation error: %s", err.Error())
		return err
	}

	if err = s.solrQuery(); err != nil {
		s.err("query execution error: %s", err.Error())
		return err
	}

	// we don't care if there are no results, this is just a connectivity test

	return nil
}
