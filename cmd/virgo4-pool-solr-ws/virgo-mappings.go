package main

import (
	"errors"
	"strings"
)

// functions that map solr data into virgo data

func firstElementOf(s []string) string {
	// return first element of slice, or blank string if empty
	val := ""

	if len(s) > 0 {
		val = s[0]
	}

	return val
}

func virgoPopulateRecordDebug(doc solrDocument) *VirgoRecordDebug {
	var debug VirgoRecordDebug

	debug.Score = doc.Score

	return &debug
}

func virgoPopulateRecord(doc solrDocument, client clientOptions) *VirgoRecord {
	var record VirgoRecord

	record.ID = doc.ID

	record.Title = firstElementOf(doc.Title)
	record.Subtitle = firstElementOf(doc.Subtitle)
	record.Author = firstElementOf(doc.Author)

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

func titlesAreEqual(t1, t2 string) bool {
	// case-insensitive match.  titles must be nonempty
	var s1, s2 string

	if s1 = strings.Trim(t1, " "); s1 == "" {
		return false
	}

	if s2 = strings.Trim(t2, " "); s2 == "" {
		return false
	}

	return strings.EqualFold(s1, s2)
}

func virgoPopulatePoolResult(solrRes *solrResponse, client clientOptions) *VirgoPoolResult {
	var poolResult VirgoPoolResult

	poolResult.ServiceURL = config.poolServiceURL.value

	poolResult.Pagination = virgoPopulatePagination(solrRes.Response.Start, len(solrRes.Response.Docs), solrRes.Response.NumFound)

	firstTitleResults := ""
	firstTitleQueried := firstElementOf(solrRes.parserInfo.parser.Titles)

	if len(solrRes.Response.Docs) > 0 {
		firstTitleResults = firstElementOf(solrRes.Response.Docs[0].Title)

		var recordList VirgoRecordList

		for _, doc := range solrRes.Response.Docs {
			record := virgoPopulateRecord(doc, client)

			recordList = append(recordList, *record)
		}

		poolResult.RecordList = &recordList
	}

	// FIXME: somehow create h/m/l confidence levels from the query score
	switch {
	case solrRes.Response.Start == 0 && solrRes.parserInfo.isTitleSearch && titlesAreEqual(firstTitleResults, firstTitleQueried):
		poolResult.Confidence = "exact"
	case solrRes.Response.MaxScore > 200.0:
		poolResult.Confidence = "high"
	case solrRes.Response.MaxScore > 100.0:
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
