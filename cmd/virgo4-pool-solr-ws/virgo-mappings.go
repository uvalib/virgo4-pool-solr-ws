package main

import (
	"fmt"
	"net/http"
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

func sliceContainsString(list []string, val string) bool {
	for _, i := range list {
		if i == val {
			return true
		}
	}

	return false
}

func (s *searchContext) virgoPopulateRecordDebug(doc *solrDocument) *VirgoRecordDebug {
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

func (s *searchContext) getSirsiURL(id string) string {
	url := strings.Replace(s.pool.config.sirsiURLTemplate, "__identifier__", id, -1)

	return url
}

func (s *searchContext) getCoverImageURL(doc *solrDocument) string {
	url := strings.Replace(s.pool.config.coverImageURLTemplate, "__identifier__", doc.ID, -1)

	// also add query parameters:
	// doc_type: music or non_music
	// books require at least one of: isbn, oclc, lccn, upc
	// music requires: artist_name, album_name
	// all else is optional

	// build query parameters using http package to properly quote values
	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		return ""
	}

	qp := req.URL.Query()

	switch {
	// music
	case sliceContainsString(doc.Pool, "music_recordings"):
		qp.Add("doc_type", "music")

		if len(doc.Author) > 0 {
			qp.Add("artist_name", firstElementOf(doc.Author))
		}

		if len(doc.Title) > 0 {
			qp.Add("album_name", firstElementOf(doc.Title))
		}

	// books
	case sliceContainsString(doc.Pool, "catalog"):
		qp.Add("doc_type", "non_music")

		if len(doc.ISBN) > 0 {
			qp.Add("isbn", strings.Join(doc.ISBN, ","))
		}

		if len(doc.OCLC) > 0 {
			qp.Add("oclc", strings.Join(doc.OCLC, ","))
		}

		if len(doc.LCCN) > 0 {
			qp.Add("lccn", strings.Join(doc.LCCN, ","))
		}

		if len(doc.UPC) > 0 {
			qp.Add("upc", strings.Join(doc.UPC, ","))
		}

	// everything else
	default:
		qp.Add("doc_type", "non_music")
	}

	req.URL.RawQuery = qp.Encode()

	return req.URL.String()
}

func (s *searchContext) isExposedFacetValue(facetDef poolFacetDefinition, value string) bool {
	if len(facetDef.ExposedValues) == 0 {
		return true
	}

	for _, exposedValue := range facetDef.ExposedValues {
		if strings.EqualFold(value, exposedValue) == true {
			return true
		}
	}

	return false
}

func (s *searchContext) virgoPopulateRecord(doc *solrDocument, isSingleTitleSearch bool, titleQueried string) *VirgoRecord {
	var r VirgoRecord

	// new style records -- order is important, primarily for generic "text" fields

	r.addBasicField(newField("id", s.client.localize("FieldIdentifier"), doc.ID).setType("identifier").setDisplay("optional"))
	r.addBasicField(newField("title", s.client.localize("FieldTitle"), firstElementOf(doc.Title)).setType("title"))
	r.addBasicField(newField("subtitle", s.client.localize("FieldSubtitle"), firstElementOf(doc.Subtitle)).setType("subtitle"))

	for _, item := range doc.Author {
		r.addBasicField(newField("author", s.client.localize("FieldAuthor"), item).setType("author"))
	}

	availability := doc.AnonAvailability
	if s.client.isAuthenticated() == true {
		availability = doc.UVAAvailability
	}

	for _, item := range availability {
		if s.isExposedFacetValue(s.pool.solr.availableFacets["FacetAvailability"], item) {
			r.addBasicField(newField("availability", s.client.localize("FieldAvailability"), item).setType("availability"))
		}
	}

	for _, item := range doc.Subject {
		r.addDetailedField(newField("subject", s.client.localize("FieldSubject"), item))
	}

	for _, item := range doc.Language {
		r.addDetailedField(newField("language", s.client.localize("FieldLanguage"), item))
	}

	for _, item := range doc.Format {
		r.addDetailedField(newField("format", s.client.localize("FieldFormat"), item))
	}

	for _, item := range doc.Library {
		r.addDetailedField(newField("library", s.client.localize("FieldLibrary"), item))
	}

	for _, item := range doc.CallNumber {
		r.addDetailedField(newField("call_number", s.client.localize("FieldCallNumber"), item))
	}

	for _, item := range doc.CallNumberBroad {
		r.addDetailedField(newField("call_number_broad", s.client.localize("FieldCallNumberBroad"), item))
	}

	for _, item := range doc.CallNumberNarrow {
		r.addDetailedField(newField("call_number_narrow", s.client.localize("FieldCallNumberNarrow"), item))
	}

	/*
		for _, item := range doc.ISBN {
			r.addDetailedField(newField("isbn", "ISBN", item).setDisplay("optional"))
		}

		for _, item := range doc.ISSN {
			r.addDetailedField(newField("issn", "ISSN", item).setDisplay("optional"))
		}

		for _, item := range doc.OCLC {
			r.addDetailedField(newField("oclc", "OCLC", item).setDisplay("optional"))
		}

		for _, item := range doc.LCCN {
			r.addDetailedField(newField("lccn", "LCCN", item).setDisplay("optional"))
		}

		for _, item := range doc.UPC {
			r.addDetailedField(newField("upc", "UPC", item).setDisplay("optional"))
		}
	*/

	// virgo classic url

	if strings.HasPrefix(doc.ID, "u") {
		r.addDetailedField(newField("sirsi_url", s.client.localize("FieldMore"), s.getSirsiURL(doc.ID[1:])).setType("url"))
	}

	// cover image url

	/*
		if coverImageURL := s.getCoverImageURL(doc); coverImageURL != "" {
			r.addBasicField(newField("cover_image", "", coverImageURL).setType("image-json-url").setDisplay("optional"))
		}
	*/

	// add exact designator if applicable

	itemTitle := firstElementOf(doc.Title)

	if isSingleTitleSearch == true && titlesAreEqual(itemTitle, titleQueried) {
		r.Exact = true
	}

	// add internal info

	r.workTitle2KeySort = doc.WorkTitle2KeySort

	// add debug info?
	if s.client.opts.debug == true {
		r.Debug = s.virgoPopulateRecordDebug(doc)
	}

	return &r
}

func (s *searchContext) virgoPopulateFacetBucket(value solrBucket) *VirgoFacetBucket {
	var bucket VirgoFacetBucket

	bucket.Value = value.Val
	bucket.Count = value.Count

	return &bucket
}

func (s *searchContext) virgoPopulateFacet(facetDef poolFacetDefinition, value solrResponseFacet) *VirgoFacet {
	var facet VirgoFacet

	facet.ID = facetDef.Name
	facet.Name = s.client.localize(facet.ID)

	var buckets []VirgoFacetBucket

	for _, b := range value.Buckets {
		bucket := s.virgoPopulateFacetBucket(b)

		if s.isExposedFacetValue(facetDef, bucket.Value) {
			buckets = append(buckets, *bucket)
		}
	}

	facet.Buckets = buckets

	return &facet
}

func (s *searchContext) virgoPopulatePagination(start, rows, total int) *VirgoPagination {
	var pagination VirgoPagination

	pagination.Start = start
	pagination.Rows = rows
	pagination.Total = total

	return &pagination
}

func (s *searchContext) virgoPopulatePoolResultDebug(solrRes *solrResponse) *VirgoPoolResultDebug {
	var debug VirgoPoolResultDebug

	debug.RequestID = s.client.reqID
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

func (s *searchContext) virgoPopulateRecordList(solrDocuments *solrResponseDocuments, isSingleTitleSearch bool, titleQueried string) *[]VirgoRecord {
	var recordList []VirgoRecord

	for _, doc := range solrDocuments.Docs {
		record := s.virgoPopulateRecord(&doc, isSingleTitleSearch, titleQueried)

		recordList = append(recordList, *record)
	}

	return &recordList
}

func (s *searchContext) virgoPopulateFacetList(facetDefs map[string]poolFacetDefinition, solrFacets solrResponseFacets) *[]VirgoFacet {
	var facetList []VirgoFacet
	gotFacet := false

	for key, val := range solrFacets {
		if len(val.Buckets) > 0 {
			gotFacet = true

			facet := s.virgoPopulateFacet(facetDefs[key], val)

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

	poolResult.Pagination = s.virgoPopulatePagination(s.solrRes.meta.start, s.solrRes.meta.numRows, s.solrRes.meta.totalRows)

	poolResult.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	firstTitleResults := ""
	titleQueried := firstElementOf(s.solrRes.meta.parserInfo.parser.Titles)

	// default confidence, when there are no results
	poolResult.Confidence = "low"

	if s.solrRes.meta.numRows > 0 {
		poolResult.RecordList = s.virgoPopulateRecordList(&s.solrRes.Response, s.solrRes.meta.parserInfo.isSingleTitleSearch, titleQueried)

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
		poolResult.FacetList = s.virgoPopulateFacetList(s.pool.solr.availableFacets, s.solrRes.Facets)
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
		poolResult.Debug = s.virgoPopulatePoolResultDebug(s.solrRes)
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

	switch s.solrRes.meta.numRecords {
	case 0:
		return fmt.Errorf("Item not found")

	case 1:
		v = s.virgoPopulateRecord(s.solrRes.meta.firstDoc, false, "")

	default:
		return fmt.Errorf("Multiple items found")
	}

	s.virgoRecordRes = v

	return nil
}
