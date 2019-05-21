package main

import (
	"errors"
	"fmt"
)

// functions that map solr data into virgo data

func virgoPopulatePoolResultSummary(numFound int, maxScore float32) *VirgoPoolResult {
	// populates just the minimal amount of info needed for a summary
	var poolResult VirgoPoolResult

	s := "s"
	if numFound == 1 {
		s = ""
	}

	poolResult.PoolId = "Catalog"
	poolResult.ServiceUrl = "https://pool-solr-ws-dev.internal.lib.virginia.edu"
	poolResult.Summary = fmt.Sprintf("%d item%s found", numFound, s)

	// FIXME: somehow create a confidence level from the query score

	switch {
	case maxScore > 100.0:
		poolResult.Confidence = "exact"
	case maxScore > 10.0:
		poolResult.Confidence = "high"
	case maxScore > 1.0:
		poolResult.Confidence = "medium"
	default:
		poolResult.Confidence = "low"
	}

	return &poolResult
}

func virgoPopulatePoolResult(solrRes *solrResponse) *VirgoPoolResult {
	// populates additional information for a normal pool result
	poolResult := virgoPopulatePoolResultSummary(solrRes.Response.NumFound, solrRes.Response.MaxScore)

	poolResult.Pagination = virgoPopulatePagination(solrRes.Response.Start, len(solrRes.Response.Docs), solrRes.Response.NumFound)

	for _, doc := range solrRes.Response.Docs {
		record := virgoPopulateRecord(doc)

		poolResult.RecordList = append(poolResult.RecordList, *record)
	}

	return poolResult
}

func virgoPopulateSearchResponse(solrRes *solrResponse) *VirgoSearchResponse {
	var searchResponse VirgoSearchResponse

	poolResult := virgoPopulatePoolResult(solrRes)

	searchResponse.ResultsPools = append(searchResponse.ResultsPools, *poolResult)

	return &searchResponse
}

func virgoPopulatePagination(start, rows, total int) *VirgoPagination {
	var pagination VirgoPagination

	pagination.Start = start
	pagination.Rows = rows
	pagination.Total = total

	return &pagination
}

func virgoPopulateRecord(doc solrDocument) *VirgoRecord {
	var record VirgoRecord

	record.Id = doc.Id

	// just grab the first entry in each array

	if len(doc.Title) > 0 {
		record.Title = doc.Title[0]
	}

	if len(doc.Author) > 0 {
		record.Author = doc.Author[0]
	}

	return &record
}

// the main response functions for each endpoint

func virgoSearchResponse(solrRes *solrResponse) (*VirgoSearchResponse, error) {
	virgoRes := virgoPopulateSearchResponse(solrRes)

	return virgoRes, nil
}

func virgoRecordResponse(solrRes *solrResponse) (*VirgoRecord, error) {
	var virgoRes *VirgoRecord

	switch solrRes.Response.NumFound {
	case 0:
		return nil, errors.New("Item not found")

	case 1:
		virgoRes = virgoPopulateRecord(solrRes.Response.Docs[0])

	default:
		return nil, errors.New("Multiple items found")
	}

	return virgoRes, nil
}

func virgoPoolSummaryResponse(solrRes *solrResponse) (*VirgoPoolResult, error) {
	virgoRes := virgoPopulatePoolResultSummary(solrRes.Response.NumFound, solrRes.Response.MaxScore)

	return virgoRes, nil
}
