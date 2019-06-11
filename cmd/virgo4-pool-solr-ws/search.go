package main

import (
	"github.com/gin-gonic/gin"
)

type searchContext struct {
	client clientOptions
	virgoReq VirgoSearchRequest
	solrReq *solrRequest
	solrRes *solrResponse
}

func newSearchContext(c *gin.Context) *searchContext {
	s := searchContext{}

	s.client = getClientOptions(c)

	return &s
}

func (s *searchContext) log(format string, args ...interface{}) {
	s.client.log(format, args...)
}

func (s *searchContext) handleSearchRequest() (*VirgoPoolResult, error) {
	var err error

	// do speculative searches here; also, check title-only searches here

	if s.solrReq, err = solrSearchRequest(s.virgoReq); err != nil {
		s.log("query creation error: %s", err.Error())
		return nil, err
	}

	s.log("handleSearchRequest(): keywords: %d, titles: %d, authors: %d, subjects: %d",
		s.solrReq.parserInfo.parser.Keywords, s.solrReq.parserInfo.parser.Titles, s.solrReq.parserInfo.parser.Authors, s.solrReq.parserInfo.parser.Subjects)

	if s.solrRes, err = solrQuery(s.solrReq, s.client); err != nil {
		s.log("query execution error: %s", err.Error())
		return nil, err
	}

	var virgoRes *VirgoPoolResult

	if virgoRes, err = virgoSearchResponse(s.solrRes, s.client); err != nil {
		s.log("result parsing error: %s", err.Error())
		return nil, err
	}

	return virgoRes, nil
}

func (s *searchContext) handleRecordRequest() (*VirgoRecord, error) {
	var err error

	if s.solrReq, err = solrRecordRequest(s.virgoReq); err != nil {
		s.log("query creation error: %s", err.Error())
		return nil, err
	}

	if s.solrRes, err = solrQuery(s.solrReq, s.client); err != nil {
		s.log("query execution error: %s", err.Error())
		return nil, err
	}

	var virgoRes *VirgoRecord

	if virgoRes, err = virgoRecordResponse(s.solrRes, s.client); err != nil {
		s.log("result parsing error: %s", err.Error())
		return nil, err
	}

	return virgoRes, nil
}

func (s *searchContext) handlePingRequest() error {
	var err error

	if s.solrReq, err = solrRecordRequest(s.virgoReq); err != nil {
		s.log("query creation error: %s", err.Error())
		return err
	}

	if s.solrRes, err = solrQuery(s.solrReq, s.client); err != nil {
		s.log("query execution error: %s", err.Error())
		return err
	}

	// we don't care if there are no results, this is just a connectivity test

	return nil
}
