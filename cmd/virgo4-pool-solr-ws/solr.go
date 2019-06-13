package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

var solrClient *http.Client

func solrQuery(solrReq *solrRequest, c clientOptions) (*solrResponse, error) {
	solrUrl := fmt.Sprintf("%s/%s/%s", config.solrHost.value, config.solrCore.value, config.solrHandler.value)

	req, reqErr := http.NewRequest("GET", solrUrl, nil)
	if reqErr != nil {
		c.log("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("Failed to create Solr request")
	}

	q := req.URL.Query()

	for key, val := range solrReq.params {
		if val == "" {
			continue
		}

		if key == "q" || key == "start" || key == "rows" {
			c.log("[solr] adding field: [%s] = [%s]", key, val)
		}

		q.Add(key, val)
	}

	req.URL.RawQuery = q.Encode()

	c.log("[solr] req: [%s]", req.URL.String())

	start := time.Now()

	res, resErr := solrClient.Do(req)
	if resErr != nil {
		c.log("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("Failed to receive Solr response")
	}

	elapsed := time.Since(start).Seconds()

	defer res.Body.Close()

	// parse json from body

	var solrRes solrResponse

	decoder := json.NewDecoder(res.Body)

	if decErr := decoder.Decode(&solrRes); decErr != nil {
		c.log("Decode() failed: %s", decErr.Error())
		return nil, errors.New("Failed to decode Solr response")
	}

	logHeader := fmt.Sprintf("[solr] res: header: { status = %d, QTime = %d (elapsed: %0.3fs) }", solrRes.ResponseHeader.Status, solrRes.ResponseHeader.QTime, elapsed)

	// quick validation
	if solrRes.ResponseHeader.Status != 0 {
		c.log("%s, error: { code = %d, msg = %s }", logHeader, solrRes.Error.Code, solrRes.Error.Msg)
		return nil, errors.New(fmt.Sprintf("%d - %s", solrRes.Error.Code, solrRes.Error.Msg))
	}

	c.log("%s, body: { numFound = %d, start = %d, maxScore = %0.2f, len(docs) = %d }", logHeader, solrRes.Response.NumFound, solrRes.Response.Start, solrRes.Response.MaxScore, len(solrRes.Response.Docs))

	solrRes.parserInfo = solrReq.parserInfo

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
