package main

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

type searchContext struct {
	client *clientOptions
	virgoReq VirgoSearchRequest
	virgoRes *VirgoPoolResult
	solrReq *solrRequest
	solrRes *solrResponse
}

func newSearchContext(c *gin.Context) *searchContext {
	s := searchContext{}

	s.client = getClientOptions(c)

	return &s
}

func (s *searchContext) copySearchContext() (*searchContext) {
	// performs a copy somewhere between shallow and deep
	// (just enough to let this context be used for another search
	// without clobbering the original context)

	sc := &searchContext{}

	c := *s.client
	v := s.virgoReq
	p := *s.virgoReq.Pagination

	sc.client = &c
	sc.virgoReq = v
	sc.virgoReq.Pagination = &p
	sc.virgoRes = nil
	sc.solrReq = nil
	sc.solrRes = nil

	return sc
}

func (s *searchContext) log(format string, args ...interface{}) {
	s.client.log(format, args...)
}

func (s *searchContext) err(format string, args ...interface{}) {
	s.client.err(format, args...)
}

func (s *searchContext) performSearch() error {
	sc := searchContext{}

	sc.client = s.client

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

func confidenceIndex(confidence string) int {
	conf := []string{"low", "medium", "high", "exact"}
	for idx, val := range conf {
		if val == confidence {
			return idx
		}
	}
	// No confidence match. Assume worst value
	return 0
}

func (s *searchContext) intuitIntendedSearch() (*searchContext) {
	var err error

	// get top result for original search
	o := s.copySearchContext()
	o.virgoReq.Pagination.Start = 0
	o.virgoReq.Pagination.Rows = 1
	if err = o.performSearch(); err != nil {
		// just return original search context (which will also likely fail)
		return s
	}

	// if original was a keyword search, see if title or author top result is better

	s.log("o: confidence = [%s]  maxScore = [%0.2f]", o.virgoRes.Confidence, o.solrRes.Response.MaxScore)

	best := o

	if o.solrReq.parserInfo.isKeywordSearch {
		keyword := firstElementOf(o.solrReq.parserInfo.parser.Keywords)
		var bidx int

		s.log("is a keyword search for: [%s]", keyword)

		t := o.copySearchContext()
		t.virgoReq.Query = fmt.Sprintf("title:{%s}", keyword)
		if err = t.performSearch(); err == nil {
			s.log("t: confidence = [%s]  maxScore = [%0.2f]", t.virgoRes.Confidence, t.solrRes.Response.MaxScore)
			tidx := confidenceIndex(t.virgoRes.Confidence)
			bidx = confidenceIndex(best.virgoRes.Confidence)
			switch {
			case tidx > bidx:
				s.log("t: wins on confidence")
				best = t
			case tidx == bidx:
				if (t.solrRes.Response.MaxScore > best.solrRes.Response.MaxScore) {
					s.log("t: wins on score")
					best = t
				}
			}
		}

		a := o.copySearchContext()
		a.virgoReq.Query = fmt.Sprintf("author:{%s}", keyword)
		if err = a.performSearch(); err == nil {
			s.log("a: confidence = [%s]  maxScore = [%0.2f]", a.virgoRes.Confidence, a.solrRes.Response.MaxScore)
			aidx := confidenceIndex(a.virgoRes.Confidence)
			bidx = confidenceIndex(best.virgoRes.Confidence)
			switch {
			case aidx > bidx:
				s.log("a: wins on confidence")
				best = a
			case aidx == bidx:
				if (a.solrRes.Response.MaxScore > best.solrRes.Response.MaxScore) {
					s.log("a: wins on score")
					best = a
				}
			}
		}
	}

	return best
}

func (s *searchContext) handleSearchRequest() (*VirgoPoolResult, error) {
	sc := s.intuitIntendedSearch()

	// copy specific values from intuited search
	s.virgoReq.Query = sc.virgoReq.Query
	confidence := sc.virgoRes.Confidence

	if err := s.performSearch(); err != nil {
		return nil, err
	}

	// copy certain intuited values back to results
	s.virgoRes.Confidence = confidence

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
