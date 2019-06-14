package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

var confidenceIndex map[string]int

type searchContext struct {
	client         *clientOptions
	virgoReq       VirgoSearchRequest
	virgoPoolRes   *VirgoPoolResult
	virgoRecordRes *VirgoRecord
	solrReq        *solrRequest
	solrRes        *solrResponse
	confidence     string
}

func newSearchContext(c *gin.Context) *searchContext {
	s := searchContext{}

	s.client = getClientOptions(c)

	return &s
}

func (s *searchContext) copySearchContext() *searchContext {
	// performs a copy somewhere between shallow and deep
	// (just enough to let this context be used for another search
	// without clobbering the original context)

	sc := &searchContext{}

	c := *s.client
	sc.client = &c

	v := s.virgoReq
	sc.virgoReq = v

	if s.virgoReq.Pagination != nil {
		p := *s.virgoReq.Pagination
		sc.virgoReq.Pagination = &p
	}

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

	if s.solrReq, err = solrSearchRequest(s.virgoReq); err != nil {
		s.err("query creation error: %s", err.Error())
		return err
	}

	if s.solrReq.parserInfo != nil {
		s.log("Titles      : { %v } (%v)", strings.Join(s.solrReq.parserInfo.parser.Titles, "; "), s.solrReq.parserInfo.isTitleSearch)
		s.log("Authors     : { %v }", strings.Join(s.solrReq.parserInfo.parser.Authors, "; "))
		s.log("Subjects    : { %v }", strings.Join(s.solrReq.parserInfo.parser.Subjects, "; "))
		s.log("Keywords    : { %v } (%v)", strings.Join(s.solrReq.parserInfo.parser.Keywords, "; "), s.solrReq.parserInfo.isKeywordSearch)
		s.log("Identifiers : { %v }", strings.Join(s.solrReq.parserInfo.parser.Identifiers, "; "))
	}

	if s.solrRes, err = solrQuery(s.solrReq, *s.client); err != nil {
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

	if s.virgoPoolRes, err = virgoSearchResponse(s.solrRes, *s.client); err != nil {
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

	if s.virgoRecordRes, err = virgoRecordResponse(s.solrRes, *s.client); err != nil {
		s.err("result parsing error: %s", err.Error())
		return err
	}

	return nil
}

func (s *searchContext) newSearchWithTopResult(query string) (*searchContext, error) {
	// returns a new search context with the top result of the supplied query
	top := s.copySearchContext()

	top.virgoReq.Query = query
	top.virgoReq.Pagination = &VirgoPagination{Start: 0, Rows: 1}

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

	if confidenceIndex[b.results[i].virgoPoolRes.Confidence] < confidenceIndex[b.results[j].virgoPoolRes.Confidence] {
		return false
	}

	if confidenceIndex[b.results[i].virgoPoolRes.Confidence] > confidenceIndex[b.results[j].virgoPoolRes.Confidence] {
		return true
	}

	// confidence is equal; sort by score

	return b.results[i].solrRes.Response.MaxScore > b.results[j].solrRes.Response.MaxScore
}

func (s *searchContext) performSpeculativeTitleSearch() (*searchContext, error) {
	// if the query is not for the first page, return the top result for correct
	// confidence level; otherwise, let the original query determine it

	s.log("TITLE SEARCH: determining true confidence level")

	if s.virgoReq.Pagination != nil && s.virgoReq.Pagination.Start != 0 {
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

	s.log("keyword: confidence = [%s]  maxScore = [%0.2f]", keyword.virgoPoolRes.Confidence, keyword.solrRes.Response.MaxScore)
	searchResults = append(searchResults, keyword)

	if title, err = keyword.newSearchWithTopResult(fmt.Sprintf("title:{%s}", searchTerm)); err == nil {
		s.log("title: confidence = [%s]  maxScore = [%0.2f]", title.virgoPoolRes.Confidence, title.solrRes.Response.MaxScore)
		searchResults = append(searchResults, title)
	}

	if author, err = keyword.newSearchWithTopResult(fmt.Sprintf("author:{%s}", searchTerm)); err == nil {
		s.log("author: confidence = [%s]  maxScore = [%0.2f]", author.virgoPoolRes.Confidence, author.solrRes.Response.MaxScore)
		searchResults = append(searchResults, author)
	}

	confidenceSort := byConfidence{results: searchResults}
	sort.Sort(&confidenceSort)

	return confidenceSort.results[0], nil
}

func (s *searchContext) performSpeculativeSearches() (*searchContext, error) {
	var err error
	var parsedQuery *solrParserInfo

	// parse original query to determine query type

	if parsedQuery, err = virgoQueryConvertToSolr(s.virgoReq.Query); err != nil {
		return nil, err
	}

	// single-term title-only search special handling

	if parsedQuery.isTitleSearch {
		return s.performSpeculativeTitleSearch()
	}

	// single-term keyword search special handling

	if parsedQuery.isKeywordSearch {
		return s.performSpeculativeKeywordSearch(firstElementOf(parsedQuery.parser.Keywords))
	}

	// fallthrough: just return original query

	return s, nil
}

func (s *searchContext) handleSearchRequest() (*VirgoPoolResult, error) {
	var err error
	var top *searchContext

	if top, err = s.performSpeculativeSearches(); err != nil {
		return nil, err
	}

	// use query syntax from chosen search
	s.virgoReq.Query = top.virgoReq.Query

	if err = s.getPoolQueryResults(); err != nil {
		return nil, err
	}

	// restore actual confidence
	if top.confidence != "" {
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

	if s.solrReq, err = solrRecordRequest(s.virgoReq); err != nil {
		s.err("query creation error: %s", err.Error())
		return err
	}

	if s.solrRes, err = solrQuery(s.solrReq, *s.client); err != nil {
		s.err("query execution error: %s", err.Error())
		return err
	}

	// we don't care if there are no results, this is just a connectivity test

	return nil
}

func init() {
	// invalid values will be 0 (less than "low")
	confidenceIndex = map[string]int{
		"low":    1,
		"medium": 2,
		"high":   3,
		"exact":  4,
	}
}
