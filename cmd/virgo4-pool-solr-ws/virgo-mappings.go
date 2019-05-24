package main

import (
	"errors"
)

// functions that map solr data into virgo data

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

func virgoPopulatePagination(start, rows, total int) *VirgoPagination {
	var pagination VirgoPagination

	pagination.Start = start
	pagination.Rows = rows
	pagination.Total = total

	return &pagination
}

func virgoPopulatePoolResult(solrRes *solrResponse) *VirgoPoolResult {
	var poolResult VirgoPoolResult

	poolResult.ServiceUrl = config.poolServiceUrl.value

	// FIXME: somehow create a confidence level from the query score

	switch {
	case solrRes.Response.MaxScore > 100.0:
		poolResult.Confidence = "exact"
	case solrRes.Response.MaxScore > 10.0:
		poolResult.Confidence = "high"
	case solrRes.Response.MaxScore > 1.0:
		poolResult.Confidence = "medium"
	default:
		poolResult.Confidence = "low"
	}

	poolResult.Pagination = virgoPopulatePagination(solrRes.Response.Start, len(solrRes.Response.Docs), solrRes.Response.NumFound)

	for _, doc := range solrRes.Response.Docs {
		record := virgoPopulateRecord(doc)

		poolResult.RecordList = append(poolResult.RecordList, *record)
	}

	return &poolResult
}

// the main response functions for each endpoint

func virgoSearchResponse(solrRes *solrResponse) (*VirgoPoolResult, error) {
	virgoRes := virgoPopulatePoolResult(solrRes)

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
