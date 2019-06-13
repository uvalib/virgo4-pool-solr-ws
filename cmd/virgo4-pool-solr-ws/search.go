package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

var confidenceIndex map[string]int

type searchContext struct {
	client   *clientOptions
	virgoReq VirgoSearchRequest
	virgoRes *VirgoPoolResult
	solrReq  *solrRequest
	solrRes  *solrResponse
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

func (s *searchContext) performSearch() error {
	var err error

	if s.solrReq, err = solrSearchRequest(s.virgoReq); err != nil {
		s.err("query creation error: %s", err.Error())
		return err
	}

	s.log("Titles   : [%s] (%v)", strings.Join(s.solrReq.parserInfo.parser.Titles, "; "), s.solrReq.parserInfo.isTitleSearch)
	s.log("Authors  : [%s]", strings.Join(s.solrReq.parserInfo.parser.Authors, "; "))
	s.log("Subjects : [%s]", strings.Join(s.solrReq.parserInfo.parser.Subjects, "; "))
	s.log("Keywords : [%s] (%v)", strings.Join(s.solrReq.parserInfo.parser.Keywords, "; "), s.solrReq.parserInfo.isKeywordSearch)

	if s.solrRes, err = solrQuery(s.solrReq, *s.client); err != nil {
		s.err("query execution error: %s", err.Error())
		return err
	}

	if s.virgoRes, err = virgoSearchResponse(s.solrRes, *s.client); err != nil {
		s.err("result parsing error: %s", err.Error())
		return err
	}

	return nil
}

func (s *searchContext) getTopResult(query string) (*searchContext, error) {
	// returns a new search context with the top result of the supplied query
	top := s.copySearchContext()

	top.virgoReq.Query = query
	top.virgoReq.Pagination = &VirgoPagination{Start: 0, Rows: 1}

	if err := top.performSearch(); err != nil {
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
	if confidenceIndex[b.results[i].virgoRes.Confidence] < confidenceIndex[b.results[j].virgoRes.Confidence] {
		return false
	}
	if confidenceIndex[b.results[i].virgoRes.Confidence] > confidenceIndex[b.results[j].virgoRes.Confidence] {
		return true
	}
	// confidence is equal; sort by score
	return b.results[i].solrRes.Response.MaxScore > b.results[j].solrRes.Response.MaxScore
}

func (s *searchContext) intuitIntendedSearch() (*searchContext, error) {
	var err error
	var parsedQuery *solrParserInfo
	var keyword, author, title *searchContext
	var searchResults []*searchContext

	// parse original query to determine query type
	if parsedQuery, err = virgoQueryConvertToSolr(s.virgoReq.Query); err != nil {
		return nil, err
	}

	// if the query is not a keyword search, we are done
	if parsedQuery.isKeywordSearch == false {
		return s, nil
	}

	// keyword search: get top result for the supplied search term as a keyword, author, and title

	searchTerm := firstElementOf(parsedQuery.parser.Keywords)

	s.log("checking if keyword search term [%s] might be a title or author search...", searchTerm)

	if keyword, err = s.getTopResult(s.virgoReq.Query); err != nil {
		return nil, err
	}

	s.log("keyword: confidence = [%s]  maxScore = [%0.2f]", keyword.virgoRes.Confidence, keyword.solrRes.Response.MaxScore)
	searchResults = append(searchResults, keyword)

	if title, err = keyword.getTopResult(fmt.Sprintf("title:{%s}", searchTerm)); err == nil {
		s.log("title: confidence = [%s]  maxScore = [%0.2f]", title.virgoRes.Confidence, title.solrRes.Response.MaxScore)
		searchResults = append(searchResults, title)
	}

	if author, err = keyword.getTopResult(fmt.Sprintf("author:{%s}", searchTerm)); err == nil {
		s.log("author: confidence = [%s]  maxScore = [%0.2f]", author.virgoRes.Confidence, author.solrRes.Response.MaxScore)
		searchResults = append(searchResults, author)
	}

	confidenceSort := byConfidence{results: searchResults}
	sort.Sort(&confidenceSort)

	return confidenceSort.results[0], nil
}

func (s *searchContext) handleSearchRequest() (*VirgoPoolResult, error) {
	var best *searchContext
	var err error

	if best, err = s.intuitIntendedSearch(); err != nil {
		return nil, err
	}

	// copy specific values from intuited search
	s.virgoReq.Query = best.virgoReq.Query

	confidence := ""
	if best.virgoRes != nil {
		confidence = best.virgoRes.Confidence
	}

	if err := s.performSearch(); err != nil {
		return nil, err
	}

	// copy certain intuited values back to results
	if confidence != "" {
		s.virgoRes.Confidence = confidence
	}

	return s.virgoRes, nil
}

func (s *searchContext) handleRecordRequest() (*VirgoRecord, error) {
	var err error

	if s.solrReq, err = solrRecordRequest(s.virgoReq); err != nil {
		s.err("query creation error: %s", err.Error())
		return nil, err
	}

	if s.solrRes, err = solrQuery(s.solrReq, *s.client); err != nil {
		s.err("query execution error: %s", err.Error())
		return nil, err
	}

	var virgoRes *VirgoRecord

	if virgoRes, err = virgoRecordResponse(s.solrRes, *s.client); err != nil {
		s.err("result parsing error: %s", err.Error())
		return nil, err
	}

	return virgoRes, nil
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
