package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/mitchellh/mapstructure"
)

var solrClient *http.Client

func solrQuery(solrReq *solrRequest, c clientOptions) (*solrResponse, error) {
	solrURL := fmt.Sprintf("%s/%s/%s", config.solrHost.value, config.solrCore.value, config.solrHandler.value)

	jsonBytes, jsonErr := json.Marshal(solrReq.json)
	if jsonErr != nil {
		c.log("Marshal() failed: %s", jsonErr.Error())
		return nil, errors.New("Failed to marshal Solr JSON")
	}

	req, reqErr := http.NewRequest("GET", solrURL, bytes.NewBuffer(jsonBytes))
	if reqErr != nil {
		c.log("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("Failed to create Solr request")
	}

	req.Header.Set("Content-Type", "application/json")

	c.log("[solr] req: [%s]", string(jsonBytes))

	start := time.Now()

	res, resErr := solrClient.Do(req)

	elapsedNanoSec := time.Since(start)
	elapsedMS := int64(elapsedNanoSec / time.Millisecond)

	if resErr != nil {
		c.log("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("Failed to receive Solr response")
	}

	log.Printf("Successful Solr response from %s. Elapsed Time: %dms", solrURL, elapsedMS)

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

	c.log("raw json: [%s]", buf)

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

	c.log("%s, body: { numFound = %d, start = %d, maxScore = %0.2f, len(docs) = %d }", logHeader, solrRes.Response.NumFound, solrRes.Response.Start, solrRes.Response.MaxScore, len(solrRes.Response.Docs))

	solrRes.solrReq = solrReq

	return &solrRes, nil
}

func timeoutWithMinimum(str string, min int) int {
	val, err := strconv.Atoi(str)

	// fallback for invalid or nonsensical timeout values
	if err != nil || val < min {
		val = min
	}

	log.Printf("converted timeout: (%s, min: %d) => %d", str, min, val)

	return val
}

func initSolrClient() {
	connTimeout := timeoutWithMinimum(config.solrConnTimeout.value, 5)
	readTimeout := timeoutWithMinimum(config.solrReadTimeout.value, 5)

	log.Printf("Solr: conn timeout: %ds, read timeout: %ds", connTimeout, readTimeout)

	solrTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(connTimeout) * time.Second,
		}).Dial,
		TLSHandshakeTimeout: time.Duration(connTimeout) * time.Second,
	}

	solrClient = &http.Client{
		Timeout:   time.Duration(readTimeout) * time.Second,
		Transport: solrTransport,
	}
}
