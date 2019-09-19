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
	//
	// NOTE: maybe this can be avoided by setting an appropriate "json.nl" Solr value... investigating!

	facetsRaw := make(map[string]interface{})
	var facets solrResponseFacets

	for key, val := range s.solrRes.FacetsRaw {
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
		s.log("mapstructure.Decode() failed: %s", mapDecErr.Error())
		return fmt.Errorf("Failed to decode Solr facet map")
	}

	s.solrRes.Facets = facets

	return nil
}

func (s *searchContext) populateMetaFields() {
	// fill out meta fields for easier use later

	s.solrRes.meta = &s.solrReq.meta

	s.solrRes.meta.start = s.solrReq.json.Params.Start

	if s.client.opts.grouped == true {
		// calculate number of groups in this response, and total available
		// (grouping, take 2: each record is the top entry of a group, so effectively records == groups)

		s.solrRes.meta.numGroups = len(s.solrRes.Response.Docs)
		s.solrRes.meta.totalGroups = s.solrRes.Response.NumFound

		// find max score and first document
		if s.solrRes.meta.numGroups > 0 {
			s.solrRes.meta.maxScore = s.solrRes.Response.MaxScore
			s.solrRes.meta.firstDoc = &s.solrRes.Response.Docs[0]
		}

		// calculate number of records in this response
		// (grouping, take 2: this happens later, after all groups are queried to fill out their records)
		s.solrRes.meta.numRecords = 0
		s.solrRes.meta.totalRecords = -1

		// set generic "rows" fields for client pagination
		s.solrRes.meta.numRows = s.solrRes.meta.numGroups
		s.solrRes.meta.totalRows = s.solrRes.meta.totalGroups
	} else {
		// calculate number of records in this response, and total available
		s.solrRes.meta.numRecords = len(s.solrRes.Response.Docs)
		s.solrRes.meta.totalRecords = s.solrRes.Response.NumFound

		// find max score and first document
		if s.solrRes.meta.numRecords > 0 {
			s.solrRes.meta.maxScore = s.solrRes.Response.MaxScore
			s.solrRes.meta.firstDoc = &s.solrRes.Response.Docs[0]
		}

		// set generic "rows" fields for client pagination
		s.solrRes.meta.numRows = s.solrRes.meta.numRecords
		s.solrRes.meta.totalRows = s.solrRes.meta.totalRecords
	}
}

func (s *searchContext) solrQuery() error {
	jsonBytes, jsonErr := json.Marshal(s.solrReq.json)
	if jsonErr != nil {
		s.log("Marshal() failed: %s", jsonErr.Error())
		return fmt.Errorf("Failed to marshal Solr JSON")
	}

	req, reqErr := http.NewRequest("GET", s.pool.solr.url, bytes.NewBuffer(jsonBytes))
	if reqErr != nil {
		s.log("NewRequest() failed: %s", reqErr.Error())
		return fmt.Errorf("Failed to create Solr request")
	}

	req.Header.Set("Content-Type", "application/json")

	if s.client.opts.verbose == true {
		s.log("[solr] req: [%s]", string(jsonBytes))
	} else {
		// prettify logged query
		pieces := strings.SplitAfter(s.solrReq.json.Params.Q, fmt.Sprintf(" AND %s:", s.pool.config.solrGroupField))
		q := pieces[0]
		if len(pieces) > 1 {
			q = q + " ..."
		}
		s.log("[solr] req: [%s]", q)
	}

	start := time.Now()
	res, resErr := s.pool.solr.client.Do(req)
	elapsedMS := int64(time.Since(start) / time.Millisecond)

	if resErr != nil {
		s.log("client.Do() failed: %s", resErr.Error())
		return fmt.Errorf("Failed to receive Solr response")
	}

	s.log("Successful Solr response from %s. Elapsed Time: %d (ms)", s.pool.solr.url, elapsedMS)

	s.log("[SOLR] http res: %5d ms", int64(time.Since(start) / time.Millisecond))

	defer res.Body.Close()

	var solrRes solrResponse

	/*
		// read entire response, then parse it (causing issues for large responses?)

		buf, _ := ioutil.ReadAll(res.Body)

		if s.client.opts.verbose == true {
			s.log("[solr] raw: [%s]", buf)
		}

		if jErr := json.Unmarshal(buf, &solrRes); jErr != nil {
			s.log("unexpected Solr response: [%s]", buf)
			s.log("Unmarshal() failed: %s", jErr.Error())
			return fmt.Errorf("Failed to unmarshal Solr response")
		}
	*/

	// parse response from stream

	decoder := json.NewDecoder(res.Body)

	start = time.Now()
	if decErr := decoder.Decode(&solrRes); decErr != nil {
		s.log("Decode() failed: %s", decErr.Error())
		return fmt.Errorf("Failed to decode Solr response")
	}
	s.log("[SOLR] json dec: %5d ms", int64(time.Since(start) / time.Millisecond))

	s.solrRes = &solrRes

	start = time.Now()
	s.convertFacets()
	s.log("[SOLR] conv fac: %5d ms", int64(time.Since(start) / time.Millisecond))

	// log abbreviated results

	logHeader := fmt.Sprintf("[solr] res: header: { status = %d, QTime = %d }", solrRes.ResponseHeader.Status, solrRes.ResponseHeader.QTime)

	// quick validation
	if solrRes.ResponseHeader.Status != 0 {
		s.log("%s, error: { code = %d, msg = %s }", logHeader, solrRes.Error.Code, solrRes.Error.Msg)
		return fmt.Errorf("%d - %s", solrRes.Error.Code, solrRes.Error.Msg)
	}

	start = time.Now()
	s.populateMetaFields()
	s.log("[SOLR] pop meta: %5d ms", int64(time.Since(start) / time.Millisecond))

	s.log("%s, meta: { groups = %d, records = %d }, body: { start = %d, rows = %d, total = %d, maxScore = %0.2f }", logHeader, solrRes.meta.numGroups, solrRes.meta.numRecords, solrRes.meta.start, solrRes.meta.numRows, solrRes.meta.totalRows, solrRes.meta.maxScore)

	return nil
}
