package main

import (
	"errors"
)

// functions that map solr data into virgo data

func virgoPopulateRecordDebug(doc solrDocument) *VirgoRecordDebug {
	var debug VirgoRecordDebug

	debug.Score = doc.Score

	return &debug
}

func virgoPopulateRecord(doc solrDocument, client clientOptions) *VirgoRecord {
	var record VirgoRecord

	record.Id = doc.Id

	// just grab the first entry in each array

	if len(doc.Title) > 0 {
		record.Title = doc.Title[0]
	}

	if len(doc.Subtitle) > 0 {
		record.Subtitle = doc.Subtitle[0]
	}

	if len(doc.Author) > 0 {
		record.Author = doc.Author[0]
	}

	if client.debug == true {
		record.Debug = virgoPopulateRecordDebug(doc)
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

func virgoPopulatePoolResultDebug(solrRes *solrResponse) *VirgoPoolResultDebug {
	var debug VirgoPoolResultDebug

	debug.MaxScore = solrRes.Response.MaxScore

	return &debug
}

func virgoPopulatePoolResult(solrRes *solrResponse, client clientOptions) *VirgoPoolResult {
	var poolResult VirgoPoolResult

	poolResult.ServiceUrl = config.poolServiceUrl.value

	poolResult.Pagination = virgoPopulatePagination(solrRes.Response.Start, len(solrRes.Response.Docs), solrRes.Response.NumFound)

	firstTitle := ""

	if len(solrRes.Response.Docs) > 0 {
		firstTitle = solrRes.Response.Docs[0].Title[0]

		var recordList VirgoRecordList

		for _, doc := range solrRes.Response.Docs {
			record := virgoPopulateRecord(doc, client)

			recordList = append(recordList, *record)
		}

		poolResult.RecordList = &recordList
	}

	// FIXME: somehow create a confidence level from the query score
	// (exact would mean first result equals the query and/or has a high enough score?)

	switch {
	case firstTitle == "FIXME: matches (one of?) searched title(s?)":
		poolResult.Confidence = "exact"
	case solrRes.Response.MaxScore > 100.0:
		poolResult.Confidence = "high"
	case solrRes.Response.MaxScore > 10.0:
		poolResult.Confidence = "medium"
	default:
		poolResult.Confidence = "low"
	}

	if client.debug == true {
		poolResult.Debug = virgoPopulatePoolResultDebug(solrRes)
	}

	return &poolResult
}

// the main response functions for each endpoint

func virgoSearchResponse(solrRes *solrResponse, client clientOptions) (*VirgoPoolResult, error) {
	virgoRes := virgoPopulatePoolResult(solrRes, client)

	return virgoRes, nil
}

func virgoRecordResponse(solrRes *solrResponse, client clientOptions) (*VirgoRecord, error) {
	var virgoRes *VirgoRecord

	switch solrRes.Response.NumFound {
	case 0:
		return nil, errors.New("Item not found")

	case 1:
		virgoRes = virgoPopulateRecord(solrRes.Response.Docs[0], client)

	default:
		return nil, errors.New("Multiple items found")
	}

	return virgoRes, nil
}
