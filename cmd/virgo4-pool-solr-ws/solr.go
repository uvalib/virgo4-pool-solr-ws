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

func solrQuery(solrReq *solrRequest) (*solrResponse, error) {
	solrUrl := fmt.Sprintf("%s/%s/%s", config.solrHost.value, config.solrCore.value, config.solrHandler.value)

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

	log.Printf("solr query: [%s]", req.URL.String())

	res, resErr := solrClient.Do(req)
	if resErr != nil {
		log.Printf("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("Failed to receive Solr response")
	}

	defer res.Body.Close()

	// parse json from body

	var solrRes solrResponse

	decoder := json.NewDecoder(res.Body)
	decErr := decoder.Decode(&solrRes)
	if decErr != nil {
		log.Printf("Decode() failed: %s", decErr.Error())
		return nil, errors.New("Failed to decode Solr response")
	}

	// quick validation
	if solrRes.ResponseHeader.Status != 0 {
		log.Printf("Solr error: %d (%d - %s)", solrRes.ResponseHeader.Status, solrRes.Error.Code, solrRes.Error.Msg)
		return nil, errors.New(fmt.Sprintf("%d - %s", solrRes.Error.Code, solrRes.Error.Msg))
	}

	return &solrRes, nil
}

func solrPoolResultsRequest(virgoReq VirgoPoolResultsRequest) (*solrRequest, error) {
	solrReq := solrRequestNew()

	// defaults

	start := 0
	rows := 10

	// use passed values if they make sense

	if virgoReq.Start >= 0 {
		start = virgoReq.Start
	}

	if virgoReq.Rows > 0 {
		rows = virgoReq.Rows
	}

	// build parameter map

	solrReq.params["q"] = virgoReq.Query
	solrReq.params["start"] = fmt.Sprintf("%d", start)
	solrReq.params["rows"] = fmt.Sprintf("%d", rows)

	return solrReq, nil
}

func solrPoolResultsResponse(solrRes *solrResponse) (*VirgoPoolResultsResponse, error) {
	var virgoRes VirgoPoolResultsResponse

	virgoRes.ResultCount = solrRes.Response.NumFound
	virgoRes.Pagination.Start = solrRes.Response.Start
	virgoRes.Pagination.Rows = len(solrRes.Response.Docs)
	virgoRes.Pagination.Total = solrRes.Response.NumFound

	for _, doc := range solrRes.Response.Docs {
		var record VirgoRecord

		record.Id = doc.Id

		if len(doc.Title) > 0 {
			record.Title = doc.Title[0]
		}

		virgoRes.RecordSet = append(virgoRes.RecordSet, record)
	}

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

	virgoRes, virgoResErr := solrPoolResultsResponse(solrRes)

	if virgoResErr != nil {
		log.Printf("result parsing error: %s", virgoResErr.Error())
		return nil, virgoResErr
	}

	return virgoRes, nil
}

func solrPoolResultsRecordRequest(virgoReq VirgoPoolResultsRecordRequest) (*solrRequest, error) {
	solrReq := solrRequestNew()

	solrReq.params["q"] = fmt.Sprintf("id:%s", virgoReq.Id)

	return solrReq, nil
}

func solrPoolResultsRecordResponse(solrRes *solrResponse) (*VirgoPoolResultsRecordResponse, error) {
	var virgoRes VirgoPoolResultsRecordResponse

	virgoRes.ResultCount = solrRes.Response.NumFound
	virgoRes.Pagination.Start = solrRes.Response.Start
	virgoRes.Pagination.Rows = len(solrRes.Response.Docs)
	virgoRes.Pagination.Total = solrRes.Response.NumFound

	for _, doc := range solrRes.Response.Docs {
		var record VirgoRecord

		if len(doc.Title) > 0 {
			record.Title = doc.Title[0]
		}

		virgoRes.RecordSet = append(virgoRes.RecordSet, record)
	}

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

	virgoRes, virgoResErr := solrPoolResultsRecordResponse(solrRes)

	if virgoResErr != nil {
		log.Printf("result parsing error: %s", virgoResErr.Error())
		return nil, virgoResErr
	}

	return virgoRes, nil
}

func solrPoolSummaryRequest(virgoReq VirgoPoolSummaryRequest) (*solrRequest, error) {
	solrReq := solrRequestNew()

	solrReq.params["q"] = virgoReq.Query

	return solrReq, nil
}

func solrPoolSummaryResponse(solrRes *solrResponse) (*VirgoPoolSummaryResponse, error) {
	var virgoRes VirgoPoolSummaryResponse

	s := "s"
	if solrRes.Response.NumFound == 1 {
		s = ""
	}

	virgoRes.Name = "Catalog"
	virgoRes.Link = "https://fixme"
	virgoRes.Summary = fmt.Sprintf("%d item%s found", solrRes.Response.NumFound, s)

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

	virgoRes, virgoResErr := solrPoolSummaryResponse(solrRes)

	if virgoResErr != nil {
		log.Printf("result parsing error: %s", virgoResErr.Error())
		return nil, virgoResErr
	}

	return virgoRes, nil
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
