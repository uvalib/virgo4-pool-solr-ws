package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
)

func (s *searchContext) convertFacets() error {
	// convert Solr "facets" block to internal structures.
	// due to its structure block, we cannot read it directly into arbitrary structs
	// (it contains both named facet blocks along with a "count" field that is not such a block).
	//
	// e.g. '{ "count": 23, "facet1": { ... }, "facet2": { ... }, ..., "facetN": { ... } }'
	//
	// so we read it in as map[string]interface{}, strip out the keys that are not this type
	// (e.g. "count", which will be float64), and then decode the resulting map into
	// a map[string]solrResponseFacet type.

	facetsRaw := make(map[string]interface{})
	var facets map[string]solrResponseFacet

	for key, val := range s.solr.res.FacetsRaw {
		switch val.(type) {
		case map[string]interface{}:
			facetsRaw[key] = val
		}
	}

	cfg := &mapstructure.DecoderConfig{
		Metadata:   nil,
		Result:     &facets,
		TagName:    "json",
		ZeroFields: true,
	}

	dec, _ := mapstructure.NewDecoder(cfg)

	if mapDecErr := dec.Decode(facetsRaw); mapDecErr != nil {
		s.log("SOLR: mapstructure.Decode() failed: %s", mapDecErr.Error())
		return fmt.Errorf("failed to decode Solr facet map")
	}

	s.solr.res.Facets = facets

	return nil
}

func (s *searchContext) populateMetaFields() {
	// fill out meta fields for easier use later

	s.solr.res.meta = &s.solr.req.meta

	s.solr.res.meta.start = s.solr.req.json.Params.Start

	if s.virgo.flags.groupResults == true {
		// calculate number of groups in this response, and total available
		// (grouping, take 2: each record is the top entry of a group, so effectively records == groups)

		s.solr.res.meta.numGroups = len(s.solr.res.Response.Docs)
		s.solr.res.meta.totalGroups = s.solr.res.Response.NumFound

		// find max score and first document
		if s.solr.res.meta.numGroups > 0 {
			s.solr.res.meta.maxScore = s.solr.res.Response.MaxScore
			s.solr.res.meta.firstDoc = &s.solr.res.Response.Docs[0]
		}

		// calculate number of records in this response
		// (grouping, take 2: this happens later, after all groups are queried to fill out their records)
		s.solr.res.meta.numRecords = 0
		s.solr.res.meta.totalRecords = -1

		// set generic "rows" fields for client pagination
		s.solr.res.meta.numRows = s.solr.res.meta.numGroups
		s.solr.res.meta.totalRows = s.solr.res.meta.totalGroups
	} else {
		// calculate number of records in this response, and total available
		s.solr.res.meta.numRecords = len(s.solr.res.Response.Docs)
		s.solr.res.meta.totalRecords = s.solr.res.Response.NumFound

		// find max score and first document
		if s.solr.res.meta.numRecords > 0 {
			s.solr.res.meta.maxScore = s.solr.res.Response.MaxScore
			s.solr.res.meta.firstDoc = &s.solr.res.Response.Docs[0]
		}

		// set generic "rows" fields for client pagination
		s.solr.res.meta.numRows = s.solr.res.meta.numRecords
		s.solr.res.meta.totalRows = s.solr.res.meta.totalRecords
	}
}

func (s *searchContext) solrQuery() error {
	ctx := s.pool.solr.service

	jsonBytes, jsonErr := json.Marshal(s.solr.req.json)
	if jsonErr != nil {
		s.log("SOLR: Marshal() failed: %s", jsonErr.Error())
		return fmt.Errorf("failed to marshal Solr JSON")
	}

	// we cannot use query parameters for the request due to the
	// possibility of triggering a 414 response (URI Too Long).

	// instead, write the json to the body of the request.
	// NOTE: Solr is lenient; GET or POST works fine for this.

	req, reqErr := http.NewRequest("POST", ctx.url, bytes.NewBuffer(jsonBytes))
	if reqErr != nil {
		s.log("SOLR: NewRequest() failed: %s", reqErr.Error())
		return fmt.Errorf("failed to create Solr request")
	}

	req.Header.Set("Content-Type", "application/json")

	if s.client.opts.verbose == true {
		s.log("SOLR: req: [%s]", string(jsonBytes))
	} else {
		// prettify logged query
		pieces := strings.SplitAfter(s.solr.req.json.Params.Q, fmt.Sprintf(" AND (%s:", s.pool.config.Local.Solr.GroupField))
		q := pieces[0]
		if len(pieces) > 1 {
			q = q + " ... )"
		}
		s.log("SOLR: req: [%s]", q)
	}

	start := time.Now()
	res, resErr := ctx.client.Do(req)
	elapsedMS := int64(time.Since(start) / time.Millisecond)

	// external service failure logging (scenario 1)

	if resErr != nil {
		status := http.StatusBadRequest
		errMsg := resErr.Error()
		if strings.Contains(errMsg, "Timeout") {
			status = http.StatusRequestTimeout
			errMsg = fmt.Sprintf("%s timed out", ctx.url)
		} else if strings.Contains(errMsg, "connection refused") {
			status = http.StatusServiceUnavailable
			errMsg = fmt.Sprintf("%s refused connection", ctx.url)
		}

		s.log("SOLR: client.Do() failed: %s", resErr.Error())
		s.err("Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, ctx.url, status, errMsg, elapsedMS)
		return fmt.Errorf("failed to receive Solr response")
	}

	defer res.Body.Close()

	var solrRes solrResponse

	decoder := json.NewDecoder(res.Body)

	// external service failure logging (scenario 2)

	if decErr := decoder.Decode(&solrRes); decErr != nil {
		s.log("SOLR: Decode() failed: %s", decErr.Error())
		s.err("Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, ctx.url, http.StatusInternalServerError, decErr.Error(), elapsedMS)
		return fmt.Errorf("failed to decode Solr response")
	}

	// external service success logging

	s.log("Successful Solr response from %s %s. Elapsed Time: %d (ms)", req.Method, ctx.url, elapsedMS)

	s.log("SOLR: endpoint: %-8s  qtime: %5d  elapsed: %5d  overhead: %5d", s.virgo.endpoint, solrRes.ResponseHeader.QTime, elapsedMS, elapsedMS-int64(solrRes.ResponseHeader.QTime))

	s.solr.res = &solrRes

	s.convertFacets()

	// log abbreviated results

	logHeader := fmt.Sprintf("SOLR: res: header: { status = %d, QTime = %d }", solrRes.ResponseHeader.Status, solrRes.ResponseHeader.QTime)

	// quick validation
	if solrRes.ResponseHeader.Status != 0 {
		s.log("%s, error: { code = %d, msg = %s }", logHeader, solrRes.Error.Code, solrRes.Error.Msg)
		return fmt.Errorf("%d - %s", solrRes.Error.Code, solrRes.Error.Msg)
	}

	s.populateMetaFields()

	s.log("%s, meta: { groups = %d, records = %d }, body: { start = %d, rows = %d, total = %d, maxScore = %0.2f }", logHeader, solrRes.meta.numGroups, solrRes.meta.numRecords, solrRes.meta.start, solrRes.meta.numRows, solrRes.meta.totalRows, solrRes.meta.maxScore)

	return nil
}

func (s *searchContext) solrPing() error {
	ctx := s.pool.solr.healthCheck

	req, reqErr := http.NewRequest("GET", ctx.url, nil)
	if reqErr != nil {
		s.log("SOLR: NewRequest() failed: %s", reqErr.Error())
		return fmt.Errorf("failed to create Solr request")
	}

	start := time.Now()
	res, resErr := ctx.client.Do(req)
	elapsedMS := int64(time.Since(start) / time.Millisecond)

	// external service failure logging (scenario 1)

	if resErr != nil {
		status := http.StatusBadRequest
		errMsg := resErr.Error()
		if strings.Contains(errMsg, "Timeout") {
			status = http.StatusRequestTimeout
			errMsg = fmt.Sprintf("%s timed out", ctx.url)
		} else if strings.Contains(errMsg, "connection refused") {
			status = http.StatusServiceUnavailable
			errMsg = fmt.Sprintf("%s refused connection", ctx.url)
		}

		s.log("SOLR: client.Do() failed: %s", resErr.Error())
		s.err("Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, ctx.url, status, errMsg, elapsedMS)
		return fmt.Errorf("failed to receive Solr response")
	}

	defer res.Body.Close()

	var solrRes solrResponse

	decoder := json.NewDecoder(res.Body)

	// external service failure logging (scenario 2)

	if decErr := decoder.Decode(&solrRes); decErr != nil {
		s.log("SOLR: Decode() failed: %s", decErr.Error())
		s.err("Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, ctx.url, http.StatusInternalServerError, decErr.Error(), elapsedMS)
		return fmt.Errorf("failed to decode Solr response")
	}

	// external service success logging

	s.log("Successful Solr response from %s %s. Elapsed Time: %d (ms)", req.Method, ctx.url, elapsedMS)

	logHeader := fmt.Sprintf("SOLR: res: header: { status = %d, QTime = %d }", solrRes.ResponseHeader.Status, solrRes.ResponseHeader.QTime)

	// quick validation
	if solrRes.ResponseHeader.Status != 0 {
		s.log("%s, error: { code = %d, msg = %s }", logHeader, solrRes.Error.Code, solrRes.Error.Msg)
		return fmt.Errorf("%d - %s", solrRes.Error.Code, solrRes.Error.Msg)
	}

	s.log("%s, ping status: %s", logHeader, solrRes.Status)

	if solrRes.Status != "OK" {
		return fmt.Errorf("ping status was not OK")
	}

	return nil
}
