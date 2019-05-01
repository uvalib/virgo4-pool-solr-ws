package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
//	"net/url"
	"strconv"
//	"strings"
	"time"
)

var solrClient *http.Client

type solrQueryParams struct {
	q string `json:"q,omitempty"` // query
	fq []string `json:"fq,omitempty` // filter quer{y,ies}
	sort string `json:"sort,omitempty` // sort field or function with asc|desc
	start string `json:"start,omitempty` // number of leading documents to skip
	rows string `json:"rows,omitempty` // number of documents to return after 'start'
	fl string `json:"fl,omitempty` // field list, comma separated
	df string `json:"df,omitempty` // default search field
	wt string `json:"wt,omitempty` // writer type (response format)
	defType string `json:"defType,omitempty` // query parser (lucene, dismax, ...)
	debugQuery string `json:"debugQuery,omitempty` // timing & results ("on" or omit)
	debug string `json:"debug,omitempty`
	explainOther string `json:"explainOther,omitempty`
	timeAllowed string `json:"timeAllowed,omitempty`
	segmentTerminatedEarly string `json:"segmentTerminatedEarly,omitempty`
	omitHeader string `json:"omitHeader,omitempty`
}

type solrParamsMap map[string]string

type solrRequest struct {
	params solrParamsMap
}

type solrResponse struct {
	json map[string]interface{}
}

func solrNewRequest() *solrRequest {
	var solrReq solrRequest

	solrReq.params = make(solrParamsMap)

	return &solrReq
}

func solrPoolResultsRequest(req VirgoPoolResultsRequest) (*solrRequest, error) {
	solrReq := solrNewRequest()

	solrReq.params["q"] = req.Query

	return solrReq, nil
}

func solrPoolResultsRecordRequest(req VirgoPoolResultsRecordRequest) (*solrRequest, error) {
	solrReq := solrNewRequest()

	solrReq.params["q"] = fmt.Sprintf("id:%s", req.Id)

	return solrReq, nil
}

func solrPoolSummaryRequest(req VirgoPoolSummaryRequest) (*solrRequest, error) {
	solrReq := solrNewRequest()

	solrReq.params["q"] = req.Query

	return solrReq, nil
}

func solrQuery(solrReq *solrRequest) (*solrResponse, error) {
	solrUrl := fmt.Sprintf("%s/%s/%s", config.solrHost.value, config.solrCore.value, config.solrHandler.value)

	log.Printf("solr query url: [%s]", solrUrl)

	req, reqErr := http.NewRequest("GET", solrUrl, nil)
	if reqErr != nil {
		log.Printf("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("Failed to create Solr request")
	}

	q := req.URL.Query()

	for key, val := range solrReq.params {
		if val == "" {
			continue
		}

		log.Printf("adding field: [%s] = [%s]", key, val)
		q.Add(key, val)
	}

	req.URL.RawQuery = q.Encode()

	log.Printf("solr query raw: [%s]", req.URL.String())

	res, resErr := solrClient.Do(req)
	if resErr != nil {
		log.Printf("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("Failed to receive Solr response")
	}

	defer res.Body.Close()

	// parse json from body

	var solrRes solrResponse

	decoder := json.NewDecoder(res.Body)
	decErr := decoder.Decode(&solrRes.json)
	if decErr != nil {
		log.Printf("Decode() failed: %s", decErr.Error())
		return nil, errors.New("Failed to decode Solr response")
	}

	//log.Printf("Solr response json: %#v", solrRes)

	return &solrRes, nil
}

func init() {
	timeout, err := strconv.Atoi(config.solrTimeout.value)

	// fallback for invalid or nonsensical timeout values

	if err != nil || timeout < 1 {
		timeout = 30
	}

	log.Printf("solr client timeout: %d", timeout)

	solrClient = &http.Client{Timeout: time.Duration(timeout) * time.Second}
}
