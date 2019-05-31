package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

var solrClient *http.Client

func solrQuery(solrReq solrRequest, client ClientOptions) (*solrResponse, error) {
	solrUrl := fmt.Sprintf("%s/%s/%s", config.solrHost.value, config.solrCore.value, config.solrHandler.value)

	req, reqErr := http.NewRequest("GET", solrUrl, nil)
	if reqErr != nil {
		log.Printf("[%s] NewRequest() failed: %s", client.reqId, reqErr.Error())
		return nil, errors.New("Failed to create Solr request")
	}

	q := req.URL.Query()

	for key, val := range solrReq.params {
		if val == "" {
			continue
		}

		log.Printf("[%s] [solr] adding field: [%s] = [%s]", client.reqId, key, val)
		q.Add(key, val)
	}

	req.URL.RawQuery = q.Encode()

	log.Printf("[%s] [solr] req: [%s]", client.reqId, req.URL.String())

	start := time.Now()

	res, resErr := solrClient.Do(req)
	if resErr != nil {
		log.Printf("[%s] client.Do() failed: %s", client.reqId, resErr.Error())
		return nil, errors.New("Failed to receive Solr response")
	}

	elapsed := time.Since(start).Seconds()

	log.Printf("[%s] [solr] query elapsed time: %0.3fs", client.reqId, elapsed)

	defer res.Body.Close()

	// parse json from body

	var solrRes solrResponse

	decoder := json.NewDecoder(res.Body)

	if decErr := decoder.Decode(&solrRes); decErr != nil {
		log.Printf("[%s] Decode() failed: %s", client.reqId, decErr.Error())
		return nil, errors.New("Failed to decode Solr response")
	}

	logHeader := fmt.Sprintf("[%s] [solr] res: header: { status = %d, QTime = %d (elapsed: %0.3fs) }", client.reqId, solrRes.ResponseHeader.Status, solrRes.ResponseHeader.QTime, elapsed)

	// quick validation
	if solrRes.ResponseHeader.Status != 0 {
		log.Printf("%s, error: { code = %d, msg = %s }", logHeader, solrRes.Error.Code, solrRes.Error.Msg)
		return nil, errors.New(fmt.Sprintf("%d - %s", solrRes.Error.Code, solrRes.Error.Msg))
	}

	log.Printf("%s, body: { numFound = %d, start = %d, maxScore = %0.2f, len(docs) = %d }", logHeader, solrRes.Response.NumFound, solrRes.Response.Start, solrRes.Response.MaxScore, len(solrRes.Response.Docs))

	return &solrRes, nil
}

func solrSearchHandler(virgoReq VirgoSearchRequest, client ClientOptions) (*VirgoPoolResult, error) {
	solrReq := solrSearchRequest(virgoReq)

	solrRes, solrResErr := solrQuery(solrReq, client)

	if solrResErr != nil {
		log.Printf("[%s] query execute error: %s", client.reqId, solrResErr.Error())
		return nil, solrResErr
	}

	virgoRes, virgoResErr := virgoSearchResponse(solrRes, client)

	if virgoResErr != nil {
		log.Printf("[%s] result parsing error: %s", client.reqId, virgoResErr.Error())
		return nil, virgoResErr
	}

	return virgoRes, nil
}

func solrRecordHandler(id string, client ClientOptions) (*VirgoRecord, error) {
	solrReq := solrRecordRequest(id)

	solrRes, solrResErr := solrQuery(solrReq, client)

	if solrResErr != nil {
		log.Printf("[%s] query execute error: %s", client.reqId, solrResErr.Error())
		return nil, solrResErr
	}

	virgoRes, virgoResErr := virgoRecordResponse(solrRes, client)

	if virgoResErr != nil {
		log.Printf("[%s] result parsing error: %s", client.reqId, virgoResErr.Error())
		return nil, virgoResErr
	}

	return virgoRes, nil
}

func initSolrClient() {
	timeout, err := strconv.Atoi(config.solrTimeout.value)

	// fallback for invalid or nonsensical timeout values

	if err != nil || timeout < 1 {
		timeout = 30
	}

	solrClient = &http.Client{Timeout: time.Duration(timeout) * time.Second}
}
