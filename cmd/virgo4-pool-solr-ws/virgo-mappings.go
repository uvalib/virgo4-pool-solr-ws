package main

import (
	"fmt"
	"strings"
	"time"
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

func virgoPopulateRecordDebug(doc *solrDocument) *VirgoRecordDebug {
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
	r.addField(f.setVisibility("")) // empty implies "basic"
}

func (r *VirgoRecord) addDetailedField(f *VirgoNuancedField) {
	r.addField(f.setVisibility("detailed"))
}

func (g *VirgoGroup) addField(f *VirgoNuancedField) {
	if f.Name == "" {
		return
	}

	g.Fields = append(g.Fields, *f)
}

func (g *VirgoGroup) addBasicField(f *VirgoNuancedField) {
	g.addField(f.setVisibility("")) // empty implies "basic"
}

func (g *VirgoGroup) addDetailedField(f *VirgoNuancedField) {
	g.addField(f.setVisibility("detailed"))
}

func newField(name, label, value string) *VirgoNuancedField {
	field := VirgoNuancedField{
		Name:       name,
		Type:       "", // implies "text"
		Label:      label,
		Value:      value,
		Visibility: "", // implies "basic"
		Display:    "", // implies not optional
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

func (f *VirgoNuancedField) setDisplay(s string) *VirgoNuancedField {
	f.Display = s
	return f
}

func virgoPopulateRecord(doc *solrDocument, client *clientContext, isSingleTitleSearch bool, titleQueried string) *VirgoRecord {
	var r VirgoRecord

	// new style records -- order is important!

	r.addBasicField(newField("id", client.localize("FieldIdentifier"), doc.ID).setType("identifier").setDisplay("optional"))
	r.addBasicField(newField("title", client.localize("FieldTitle"), firstElementOf(doc.Title)))
	r.addBasicField(newField("subtitle", client.localize("FieldSubtitle"), firstElementOf(doc.Subtitle)))

	for _, item := range doc.Author {
		r.addBasicField(newField("author", client.localize("FieldAuthor"), item))
	}

	for _, item := range doc.Subject {
		r.addDetailedField(newField("subject", client.localize("FieldSubject"), item))
	}

	for _, item := range doc.Language {
		r.addDetailedField(newField("language", client.localize("FieldLanguage"), item))
	}

	for _, item := range doc.Format {
		r.addDetailedField(newField("format", client.localize("FieldFormat"), item))
	}

	for _, item := range doc.Library {
		r.addDetailedField(newField("library", client.localize("FieldLibrary"), item))
	}

	for _, item := range doc.CallNumber {
		r.addDetailedField(newField("call_number", client.localize("FieldCallNumber"), item))
	}

	for _, item := range doc.CallNumberBroad {
		r.addDetailedField(newField("call_number_broad", client.localize("FieldCallNumberBroad"), item))
	}

	for _, item := range doc.CallNumberNarrow {
		r.addDetailedField(newField("call_number_narrow", client.localize("FieldCallNumberNarrow"), item))
	}

	// mocked up fields that we do not actually pass yet
	previewURL := "https://www.library.virginia.edu/images/icon-32.png"
	r.addDetailedField(newField("preview_url", "", previewURL).setType("url"))

	if doc.ID[0] == 'u' {
		classicURL := fmt.Sprintf("https://ils.lib.virginia.edu/uhtbin/cgisirsi/uva/0/0/5?searchdata1=%s{CKEY}", doc.ID[1:])
		r.addDetailedField(newField("classic_url", client.localize("FieldMore"), classicURL).setType("url"))
	}

	// add exact designator if applicable

	itemTitle := firstElementOf(doc.Title)

	if isSingleTitleSearch == true && titlesAreEqual(itemTitle, titleQueried) {
		r.Exact = true
	}

	// add internal info

	r.workTitle2KeySort = doc.WorkTitle2KeySort

	// add debug info?
	if client.opts.debug == true {
		r.Debug = virgoPopulateRecordDebug(doc)
	}

	return &r
}

func virgoPopulateFacetBucket(value solrBucket, client *clientContext) *VirgoFacetBucket {
	var bucket VirgoFacetBucket

	bucket.Value = value.Val
	bucket.Count = value.Count

	return &bucket
}

func virgoPopulateFacet(facetDef poolFacetDefinition, value solrResponseFacet, client *clientContext) *VirgoFacet {
	var facet VirgoFacet

	facet.ID = facetDef.Name
	facet.Name = client.localize(facet.ID)

	var buckets []VirgoFacetBucket

	for _, b := range value.Buckets {
		bucket := virgoPopulateFacetBucket(b, client)

		if len(facetDef.ExposedValues) > 0 {
			for _, val := range facetDef.ExposedValues {
				if bucket.Value == val {
					buckets = append(buckets, *bucket)
					break
				}
			}
		} else {
			buckets = append(buckets, *bucket)
		}
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

func virgoPopulatePoolResultDebug(solrRes *solrResponse, client *clientContext) *VirgoPoolResultDebug {
	var debug VirgoPoolResultDebug

	debug.RequestID = client.reqID
	debug.MaxScore = solrRes.meta.maxScore

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

func virgoPopulateRecordList(solrDocuments *solrResponseDocuments, client *clientContext, isSingleTitleSearch bool, titleQueried string) *[]VirgoRecord {
	var recordList []VirgoRecord

	for _, doc := range solrDocuments.Docs {
		record := virgoPopulateRecord(&doc, client, isSingleTitleSearch, titleQueried)

		recordList = append(recordList, *record)
	}

	return &recordList
}

func virgoPopulateFacetList(facetDefs map[string]poolFacetDefinition, solrFacets solrResponseFacets, client *clientContext) *[]VirgoFacet {
	var facetList []VirgoFacet
	gotFacet := false

	for key, val := range solrFacets {
		if len(val.Buckets) > 0 {
			gotFacet = true

			facet := virgoPopulateFacet(facetDefs[key], val, client)

			facetList = append(facetList, *facet)
		}
	}

	if gotFacet == true {
		return &facetList
	}

	return nil
}

func (s *searchContext) virgoPopulatePoolResult() {
	var poolResult VirgoPoolResult

	poolResult.Identity = s.client.localizedPoolIdentity(s.pool)

	poolResult.Pagination = virgoPopulatePagination(s.solrRes.meta.start, s.solrRes.meta.numRows, s.solrRes.meta.totalRows)

	poolResult.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	firstTitleResults := ""
	titleQueried := firstElementOf(s.solrRes.meta.parserInfo.parser.Titles)

	// default confidence, when there are no results
	poolResult.Confidence = "low"

	if s.solrRes.meta.numRows > 0 {
		poolResult.RecordList = virgoPopulateRecordList(&s.solrRes.Response, s.client, s.solrRes.meta.parserInfo.isSingleTitleSearch, titleQueried)

		// FIXME: somehow create h/m/l confidence levels from the query score
		firstTitleResults = firstElementOf(s.solrRes.meta.firstDoc.Title)

		switch {
		case s.solrRes.meta.start == 0 && s.solrRes.meta.parserInfo.isSingleTitleSearch && titlesAreEqual(firstTitleResults, titleQueried):
			poolResult.Confidence = "exact"
		case s.solrRes.meta.maxScore > s.pool.solr.scoreThresholdHigh:
			poolResult.Confidence = "high"
		case s.solrRes.meta.maxScore > s.pool.solr.scoreThresholdMedium:
			poolResult.Confidence = "medium"
		}
	}

	if len(s.solrRes.Facets) > 0 {
		poolResult.FacetList = virgoPopulateFacetList(s.pool.solr.availableFacets, s.solrRes.Facets, s.client)
	}

	// advertise facets?
	if s.solrRes.meta.advertiseFacets == true {
		var localizedFacets []VirgoFacet

		for _, facet := range s.pool.solr.virgoAvailableFacets {
			localizedFacet := VirgoFacet{ID: facet, Name: s.client.localize(facet)}
			localizedFacets = append(localizedFacets, localizedFacet)
		}

		poolResult.AvailableFacets = &localizedFacets
	}

	if len(s.solrRes.meta.warnings) > 0 {
		poolResult.Warn = &s.solrRes.meta.warnings
	}

	if s.client.opts.debug == true {
		poolResult.Debug = virgoPopulatePoolResultDebug(s.solrRes, s.client)
	}

	s.virgoPoolRes = &poolResult
}

// the main response functions for each endpoint

func (s *searchContext) virgoSearchResponse() error {
	s.virgoPopulatePoolResult()

	return nil
}

func (s *searchContext) virgoRecordResponse() error {
	var v *VirgoRecord

	switch {
	case s.solrRes.meta.numRows == 0:
		return fmt.Errorf("Item not found")

	case s.client.opts.grouped == true && s.solrRes.meta.numGroups == 1 && s.solrRes.meta.numRecords == 1:
		v = virgoPopulateRecord(s.solrRes.meta.firstDoc, s.client, false, "")

	case s.client.opts.grouped == false && s.solrRes.meta.numRecords == 1:
		v = virgoPopulateRecord(s.solrRes.meta.firstDoc, s.client, false, "")

	default:
		return fmt.Errorf("Multiple items found")
	}

	s.virgoRecordRes = v

	return nil
}
