package main

import (
	"fmt"
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

func (s *searchContext) intuitBestSearch() (*searchContext, error) {
	var err error
	var orig, author, title *searchContext

	// get top result for original search
	if orig, err = s.getTopResult(s.virgoReq.Query); err != nil {
		return nil, err
	}

	// if original is not a keyword search, we are done
	if orig.solrReq.parserInfo.isKeywordSearch == false {
		return orig, nil
	}

	// see if title or author top result is better

	best := orig

	keyword := firstElementOf(orig.solrReq.parserInfo.parser.Keywords)
	var thisIndex, bestIndex int

	s.log("checking if keyword [%s] might be a title or author search...", keyword)

	s.log("orig: confidence = [%s]  maxScore = [%0.2f]", orig.virgoRes.Confidence, orig.solrRes.Response.MaxScore)

	// check title

	if title, err = orig.getTopResult(fmt.Sprintf("title:{%s}", keyword)); err == nil {
		s.log("title: confidence = [%s]  maxScore = [%0.2f]", title.virgoRes.Confidence, title.solrRes.Response.MaxScore)

		thisIndex = confidenceIndex[title.virgoRes.Confidence]
		bestIndex = confidenceIndex[best.virgoRes.Confidence]

		switch {
		case thisIndex > bestIndex:
			s.log("title: wins on confidence")
			best = title
		case thisIndex == bestIndex && (title.solrRes.Response.MaxScore > best.solrRes.Response.MaxScore):
			s.log("title: wins on score")
			best = title
		}
	}

	// check author

	if author, err = orig.getTopResult(fmt.Sprintf("author:{%s}", keyword)); err == nil {
		s.log("author: confidence = [%s]  maxScore = [%0.2f]", author.virgoRes.Confidence, author.solrRes.Response.MaxScore)

		thisIndex = confidenceIndex[author.virgoRes.Confidence]
		bestIndex = confidenceIndex[best.virgoRes.Confidence]

		switch {
		case thisIndex > bestIndex:
			s.log("author: wins on confidence")
			best = author
		case thisIndex == bestIndex && (author.solrRes.Response.MaxScore > best.solrRes.Response.MaxScore):
			s.log("author: wins on score")
			best = author
		}
	}

	return best, nil
}

func (s *searchContext) handleSearchRequest() (*VirgoPoolResult, error) {
	var best *searchContext
	var err error

	if best, err = s.intuitBestSearch(); err != nil {
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
