package main

import (
	"errors"
	"fmt"
)

// functions that map solr data into virgo data

func virgoPopulatePoolSummary(numFound int, maxScore float32) VirgoPoolSummary {
	var summary VirgoPoolSummary

	s := "s"
	if numFound == 1 {
		s = ""
	}

	summary.Name = "Catalog"
	summary.Link = "https://fixme"
	summary.Summary = fmt.Sprintf("%d item%s found", numFound, s)

	// FIXME: somehow create a confidence level from the query score

	switch {
	case maxScore > 100.0:
		summary.Confidence = "exact"
	case maxScore > 10.0:
		summary.Confidence = "high"
	case maxScore > 1.0:
		summary.Confidence = "medium"
	default:
		summary.Confidence = "low"
	}

	return summary
}

func virgoPopulatePagination(start, rows, total int) VirgoPagination {
	var pagination VirgoPagination

	pagination.Start = start
	pagination.Rows = rows
	pagination.Total = total

	return pagination
}

func virgoPopulateRecord(doc solrDocument) VirgoRecord {
	var record VirgoRecord

	record.Id = doc.Id

	// just grab the first entry in each array

	if len(doc.Title) > 0 {
		record.Title = doc.Title[0]
	}

	if len(doc.Author) > 0 {
		record.Author = doc.Author[0]
	}

	return record
}

func virgoPoolResultsResponse(solrRes *solrResponse) (*VirgoPoolResult, error) {
	var virgoRes VirgoPoolResult

	virgoRes.Pagination = virgoPopulatePagination(solrRes.Response.Start, len(solrRes.Response.Docs), solrRes.Response.NumFound)

	virgoRes.Summary = virgoPopulatePoolSummary(solrRes.Response.NumFound, solrRes.Response.MaxScore)

	for _, doc := range solrRes.Response.Docs {
		record := virgoPopulateRecord(doc)

		virgoRes.RecordList = append(virgoRes.RecordList, record)
	}

	return &virgoRes, nil
}

func virgoPoolResultsRecordResponse(solrRes *solrResponse) (*VirgoRecord, error) {
	var virgoRes VirgoRecord

	switch solrRes.Response.NumFound {
	case 0:
		return nil, errors.New("Item not found")

	case 1:
		virgoRes = virgoPopulateRecord(solrRes.Response.Docs[0])

	default:
		return nil, errors.New("Multiple items found")
	}

	return &virgoRes, nil
}

func virgoPoolSummaryResponse(solrRes *solrResponse) (*VirgoPoolSummary, error) {
	virgoRes := virgoPopulatePoolSummary(solrRes.Response.NumFound, solrRes.Response.MaxScore)

	return &virgoRes, nil
}
