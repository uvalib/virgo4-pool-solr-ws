package main

import (
	"errors"
	"fmt"
)

// functions that map solr data into virgo data

func virgoPopulatePoolSummary(numFound int) (VirgoPoolSummary) {
	var summary VirgoPoolSummary

	s := "s"
	if numFound == 1 {
		s = ""
	}

	summary.Name = "Catalog"
	summary.Link = "https://fixme"
	summary.Summary = fmt.Sprintf("%d item%s found", numFound, s)

	return summary
}

func virgoPopulatePagination(start, rows, total int) (VirgoPagination) {
	var pagination VirgoPagination

	pagination.Start = start
	pagination.Rows = rows
	pagination.Total = total

	return pagination
}

func virgoPopulateRecord(doc solrDocument) (VirgoRecord) {
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

	virgoRes.ResultCount = solrRes.Response.NumFound

	virgoRes.Pagination = virgoPopulatePagination(solrRes.Response.Start, len(solrRes.Response.Docs), solrRes.Response.NumFound)

	virgoRes.Summary = virgoPopulatePoolSummary(solrRes.Response.NumFound)

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
		return nil, errors.New("No results found")

	case 1:
		virgoRes = virgoPopulateRecord(solrRes.Response.Docs[0])

	default:
		return nil, errors.New("Too many results found")
	}

	return &virgoRes, nil
}

func virgoPoolSummaryResponse(solrRes *solrResponse) (*VirgoPoolSummary, error) {
	virgoRes := virgoPopulatePoolSummary(solrRes.Response.NumFound)

	return &virgoRes, nil
}
