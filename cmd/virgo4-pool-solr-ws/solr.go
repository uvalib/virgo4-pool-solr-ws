package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
)

func solrQuery(solrReq *solrRequest, c clientOptions) (*solrResponse, error) {
	jsonBytes, jsonErr := json.Marshal(solrReq.json)
	if jsonErr != nil {
		c.log("Marshal() failed: %s", jsonErr.Error())
		return nil, errors.New("Failed to marshal Solr JSON")
	}

	req, reqErr := http.NewRequest("GET", pool.solr.url, bytes.NewBuffer(jsonBytes))
	if reqErr != nil {
		c.log("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("Failed to create Solr request")
	}

	req.Header.Set("Content-Type", "application/json")

	if c.verbose == true {
		c.log("[solr] req: [%s]", string(jsonBytes))
	} else {
		c.log("[solr] req: [%s]", solrReq.json.Params.Q)
	}

	start := time.Now()

	res, resErr := pool.solr.client.Do(req)

	elapsedNanoSec := time.Since(start)
	elapsedMS := int64(elapsedNanoSec / time.Millisecond)

	if resErr != nil {
		c.log("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("Failed to receive Solr response")
	}

	c.log("Successful Solr response from %s. Elapsed Time: %d (ms)", pool.solr.url, elapsedMS)

	defer res.Body.Close()

	// parse json from body

	var solrRes solrResponse

	/*
		// from stream:

		decoder := json.NewDecoder(res.Body)

		if decErr := decoder.Decode(&solrRes); decErr != nil {
			c.log("Decode() failed: %s", decErr.Error())
			return nil, errors.New("Failed to decode Solr response")
		}
	*/

	// from buffer:

	buf, _ := ioutil.ReadAll(res.Body)

	if c.verbose == true {
		c.log("[solr] raw: [%s]", buf)
	}

	if jErr := json.Unmarshal(buf, &solrRes); jErr != nil {
		c.log("Unmarshal() failed: %s", jErr.Error())
		return nil, errors.New("Failed to unmarshal Solr response")
	}

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

	facets := make(map[string]interface{})

	for key, val := range solrRes.FacetsRaw {
		switch val.(type) {
		case map[string]interface{}:
			facets[key] = val
		}
	}

	cfg := &mapstructure.DecoderConfig{
		Metadata:   nil,
		Result:     &solrRes.Facets,
		TagName:    "json",
		ZeroFields: true,
	}

	dec, _ := mapstructure.NewDecoder(cfg)

	if mapDecErr := dec.Decode(facets); mapDecErr != nil {
		c.log("mapstructure.Decode() failed: %s", mapDecErr.Error())
		return nil, errors.New("Failed to decode Solr facet map")
	}

	//c.log("dec json: %#v", solrRes)

	// log abbreviated results

	logHeader := fmt.Sprintf("[solr] res: header: { status = %d, QTime = %d (elapsed: %0.3fs) }", solrRes.ResponseHeader.Status, solrRes.ResponseHeader.QTime, elapsedNanoSec.Seconds())

	// quick validation
	if solrRes.ResponseHeader.Status != 0 {
		c.log("%s, error: { code = %d, msg = %s }", logHeader, solrRes.Error.Code, solrRes.Error.Msg)
		return nil, fmt.Errorf("%d - %s", solrRes.Error.Code, solrRes.Error.Msg)
	}

	// fill out meta fields for easier use later

	solrRes.meta = &solrReq.meta

	solrRes.meta.start = solrReq.json.Params.Start

	if c.grouped == true {
		solrRes.Grouped.WorkTitle2KeySort.NGroups = -1

		// calculate number of groups in this response, and total available
		solrRes.meta.numGroups = len(solrRes.Grouped.WorkTitle2KeySort.Groups)
		solrRes.meta.totalGroups = -1

		// find max score and first document
		if solrRes.meta.numGroups > 0 {
			solrRes.meta.maxScore = solrRes.Grouped.WorkTitle2KeySort.Groups[0].DocList.MaxScore
			solrRes.meta.firstDoc = &solrRes.Grouped.WorkTitle2KeySort.Groups[0].DocList.Docs[0]
		}

		// calculate number of records in this response
		solrRes.meta.numRecords = 0
		solrRes.meta.totalRecords = -1

		for _, g := range solrRes.Grouped.WorkTitle2KeySort.Groups {
			solrRes.meta.numRecords += len(g.DocList.Docs)
		}

		// set generic "rows" fields for client pagination
		solrRes.meta.numRows = solrRes.meta.numGroups
		solrRes.meta.totalRows = solrRes.meta.totalGroups
	} else {
		// calculate number of records in this response, and total available
		solrRes.meta.numRecords = len(solrRes.Response.Docs)
		solrRes.meta.totalRecords = solrRes.Response.NumFound

		// find max score and first document
		if solrRes.meta.numRecords > 0 {
			solrRes.meta.maxScore = solrRes.Response.MaxScore
			solrRes.meta.firstDoc = &solrRes.Response.Docs[0]
		}

		// set generic "rows" fields for client pagination
		solrRes.meta.numRows = solrRes.meta.numRecords
		solrRes.meta.totalRows = solrRes.meta.totalRecords
	}

	c.log("%s, meta: { groups = %d, records = %d }, body: { start = %d, rows = %d, total = %d, maxScore = %0.2f }", logHeader, solrRes.meta.numGroups, solrRes.meta.numRecords, solrRes.meta.start, solrRes.meta.numRows, solrRes.meta.totalRows, solrRes.meta.maxScore)

	return &solrRes, nil
}
