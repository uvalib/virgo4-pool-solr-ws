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

type solrQueryParams struct {
	q                      string   // query
	fq                     []string // filter quer{y,ies}
	sort                   string   // sort field or function with asc|desc
	start                  string   // number of leading documents to skip
	rows                   string   // number of documents to return after 'start'
	fl                     string   // field list, comma separated
	df                     string   // default search field
	wt                     string   // writer type (response format)
	defType                string   // query parser (lucene, dismax, ...)
	debugQuery             string   // timing & results ("on" or omit)
	debug                  string
	explainOther           string
	timeAllowed            string
	segmentTerminatedEarly string
	omitHeader             string
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

func solrPoolResultsRequest(virgoReq VirgoPoolResultsRequest) (*solrRequest, error) {
	solrReq := solrNewRequest()

	solrReq.params["q"] = virgoReq.Query

	return solrReq, nil
}

func solrPoolResultsResponse(solrRes *solrResponse) (*VirgoPoolResultsResponse, error) {
	var virgoRes VirgoPoolResultsResponse

	virgoRes.ResultCount = 1 //solrRes.json.response.numFound

	return &virgoRes, nil
}

func solrPoolResultsHandler(virgoReq VirgoPoolResultsRequest) (*VirgoPoolResultsResponse, error) {
	solrReq, solrReqErr := solrPoolResultsRequest(virgoReq)

	if solrReqErr != nil {
		log.Printf("query build error: %s", solrReqErr.Error())
		return nil, solrReqErr
	}

	solrRes, solrResErr := solrQuery(solrReq)

	if solrResErr != nil {
		log.Printf("query execute error: %s", solrResErr.Error())
		return nil, solrResErr
	}

	res, resErr := solrPoolResultsResponse(solrRes)

	if resErr != nil {
		log.Printf("result parsing error: %s", resErr.Error())
		return nil, resErr
	}

	return res, nil
}

func solrPoolResultsRecordRequest(virgoReq VirgoPoolResultsRecordRequest) (*solrRequest, error) {
	solrReq := solrNewRequest()

	solrReq.params["q"] = fmt.Sprintf("id:%s", virgoReq.Id)

	return solrReq, nil
}

func solrPoolResultsRecordResponse(solrRes *solrResponse) (*VirgoPoolResultsRecordResponse, error) {
	var virgoRes VirgoPoolResultsRecordResponse

	virgoRes.ResultCount = 1 //solrRes.json.response.numFound

	return &virgoRes, nil
}

func solrPoolResultsRecordHandler(virgoReq VirgoPoolResultsRecordRequest) (*VirgoPoolResultsRecordResponse, error) {
	solrReq, solrReqErr := solrPoolResultsRecordRequest(virgoReq)

	if solrReqErr != nil {
		log.Printf("query build error: %s", solrReqErr.Error())
		return nil, solrReqErr
	}

	solrRes, solrResErr := solrQuery(solrReq)

	if solrResErr != nil {
		log.Printf("query execute error: %s", solrResErr.Error())
		return nil, solrResErr
	}

	res, resErr := solrPoolResultsRecordResponse(solrRes)

	if resErr != nil {
		log.Printf("result parsing error: %s", resErr.Error())
		return nil, resErr
	}

	return res, nil
}

func solrPoolSummaryRequest(virgoReq VirgoPoolSummaryRequest) (*solrRequest, error) {
	solrReq := solrNewRequest()

	solrReq.params["q"] = virgoReq.Query

	return solrReq, nil
}

func solrPoolSummaryResponse(solrRes *solrResponse) (*VirgoPoolSummaryResponse, error) {
	var virgoRes VirgoPoolSummaryResponse

	virgoRes.Name = "name"
	virgoRes.Link = "http://blah"
	virgoRes.Summary = "1 result found"

	return &virgoRes, nil
}

func solrPoolSummaryHandler(virgoReq VirgoPoolSummaryRequest) (*VirgoPoolSummaryResponse, error) {
	solrReq, solrReqErr := solrPoolSummaryRequest(virgoReq)

	if solrReqErr != nil {
		log.Printf("query build error: %s", solrReqErr.Error())
		return nil, solrReqErr
	}

	solrRes, solrResErr := solrQuery(solrReq)

	if solrResErr != nil {
		log.Printf("query execute error: %s", solrResErr.Error())
		return nil, solrResErr
	}

	res, resErr := solrPoolSummaryResponse(solrRes)

	if resErr != nil {
		log.Printf("result parsing error: %s", resErr.Error())
		return nil, resErr
	}

	return res, nil
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
