package main

import (
	"errors"
	"fmt"
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

func (r *VirgoRecord) addField(f *VirgoNuancedField) {
	if f.Name == "" {
		return
	}

	r.Fields = append(r.Fields, *f)
}

func (r *VirgoRecord) addBasicField(f *VirgoNuancedField) {
	r.addField(f.setVisibility("basic"))
}

func (r *VirgoRecord) addDetailedField(f *VirgoNuancedField) {
	r.addField(f.setVisibility("detailed"))
}

func newField(name, label, value string) *VirgoNuancedField {
	field := VirgoNuancedField{
		Name: name,
		Type: "string",
		Label: label,
		Value: value,
		Visibility: "basic",
	}

	return &field
}

func (f *VirgoNuancedField) setName(s string) *VirgoNuancedField {
	f.Name = s
	return f
}

func (f *VirgoNuancedField) setType(s string) *VirgoNuancedField {
	f.Type = s
	return f
}

func (f *VirgoNuancedField) setLabel(s string) *VirgoNuancedField {
	f.Label = s
	return f
}

func (f *VirgoNuancedField) setValue(s string) *VirgoNuancedField {
	f.Value = s
	return f
}

func (f *VirgoNuancedField) setVisibility(s string) *VirgoNuancedField {
	f.Visibility = s
	return f
}

func virgoPopulateRecord(doc solrDocument, client clientOptions) *VirgoRecord {
	var r VirgoRecord

	// old style records
	r.ID = doc.ID
	r.Title = firstElementOf(doc.Title)
	r.Subtitle = firstElementOf(doc.Subtitle)
	r.Author = firstElementOf(doc.Author)

	// new style records -- order is important!

	r.addBasicField(newField("id", "Identifier", doc.ID))
	r.addBasicField(newField("title", "Title", firstElementOf(doc.Title)))
	r.addBasicField(newField("subtitle", "Subtitle", firstElementOf(doc.Subtitle)))

	for _, s := range doc.Author {
		r.addBasicField(newField("author", "Author", s))
	}

	for _, s := range doc.Subject {
		r.addDetailedField(newField("subject", "Subject", s))
	}

	for _, s := range doc.Language {
		r.addDetailedField(newField("language", "Language", s))
	}

	for _, s := range doc.Format {
		r.addDetailedField(newField("format", "Format", s))
	}

	for _, s := range doc.Library {
		r.addDetailedField(newField("library", "Library", s))
	}

	for _, s := range doc.CallNumber {
		r.addDetailedField(newField("call_number", "Call Number", s))
	}

	for _, s := range doc.CallNumberBroad {
		r.addDetailedField(newField("call_number_broad", "Call Number (Broad)", s))
	}

	for _, s := range doc.CallNumberNarrow {
		r.addDetailedField(newField("call_number_narrow", "Call Number (Narrow)", s))
	}


	// mocked up fields that we do not actually pass yet
	previewURL := "https://www.library.virginia.edu/images/icon-32.png"
	r.addDetailedField(newField("preview_url", "Preview Image", previewURL).setType("url"))

	if doc.ID[0] == 'u' {
		classicURL := fmt.Sprintf("https://ils.lib.virginia.edu/uhtbin/cgisirsi/uva/0/0/5?searchdata1=%s{CKEY}", doc.ID[1:])
		r.addDetailedField(newField("classic_url", "Access in Virgo Classic", classicURL).setType("url"))
	}

	// add debug info?
	if client.debug == true {
		r.Debug = virgoPopulateRecordDebug(doc)
	}

	return &r
}

func virgoPopulateFacetBucket(value solrBucket, client clientOptions) *VirgoFacetBucket {
	var bucket VirgoFacetBucket

	bucket.Value = value.Val
	bucket.Count = value.Count

	return &bucket
}

func virgoPopulateFacet(name string, value solrResponseFacet, client clientOptions) *VirgoFacet {
	var facet VirgoFacet

	facet.Name = name

	var buckets []VirgoFacetBucket

	for _, b := range value.Buckets {
		bucket := virgoPopulateFacetBucket(b, client)

		buckets = append(buckets, *bucket)
	}

	facet.Buckets = buckets

	return &facet
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
	firstTitleQueried := firstElementOf(solrRes.solrReq.meta.parserInfo.parser.Titles)

	if len(solrRes.Response.Docs) > 0 {
		firstTitleResults = firstElementOf(solrRes.Response.Docs[0].Title)

		var recordList []VirgoRecord

		for _, doc := range solrRes.Response.Docs {
			record := virgoPopulateRecord(doc, client)

			recordList = append(recordList, *record)
		}

		poolResult.RecordList = &recordList
	}

	if len(solrRes.Facets) > 0 {
		var facetList []VirgoFacet
		gotFacet := false

		for key, val := range solrRes.Facets {
			if len(val.Buckets) > 0 {
				gotFacet = true

				facet := virgoPopulateFacet(key, val, client)

				facetList = append(facetList, *facet)
			}
		}

		if gotFacet {
			poolResult.FacetList = &facetList
		}
	}

	// advertise facets?
	if solrRes.solrReq.meta.advertiseFacets == true {
		poolResult.AvailableFacets = &virgoAvailableFacets
	}

	// FIXME: somehow create h/m/l confidence levels from the query score
	switch {
	case solrRes.Response.Start == 0 && solrRes.solrReq.meta.parserInfo.isTitleSearch && titlesAreEqual(firstTitleResults, firstTitleQueried):
		poolResult.Confidence = "exact"
	case solrRes.Response.MaxScore > 200.0:
		poolResult.Confidence = "high"
	case solrRes.Response.MaxScore > 100.0:
		poolResult.Confidence = "medium"
	default:
		poolResult.Confidence = "low"
	}

	if len(solrRes.solrReq.meta.warnings) > 0 {
		poolResult.Warn = &solrRes.solrReq.meta.warnings
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
