package main

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
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

func (s *solrDocument) getFieldValueByTag(tag string) interface{} {
	rt := reflect.TypeOf(*s)

	if rt.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		v := strings.Split(f.Tag.Get("json"), ",")[0]
		if v == tag {
			return reflect.ValueOf(*s).Field(i).Interface()
		}
	}

	return nil
}

func (s *solrDocument) getStringValueByTag(tag string) string {
	v := s.getFieldValueByTag(tag)

	switch t := v.(type) {
	case string:
		return t
	}

	return ""
}

func (s *solrDocument) getStringSliceValueByTag(tag string) []string {
	v := s.getFieldValueByTag(tag)

	switch t := v.(type) {
	case []string:
		return t
	}

	return []string{}
}

func (s *searchContext) getSolrGroupFieldValue(doc *solrDocument) string {
	return doc.getStringValueByTag(s.pool.config.solrGroupField)
}

func (s *searchContext) getAuthorFieldValue(doc *solrDocument) []string {
	return doc.getStringSliceValueByTag(s.pool.config.solrAuthorField)
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
	// use solr-provided url if present

	if thumbnailURL := firstElementOf(doc.ThumbnailURL); thumbnailURL != "" {
		return thumbnailURL
	}

	// otherwise, compose a url to the cover image service

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

	authorValues := s.getAuthorFieldValue(doc)

	// remove extraneous dates from author
	author := strings.Trim(strings.Split(firstElementOf(authorValues), "[")[0], " ")
	title := firstElementOf(doc.Title)

	if sliceContainsString(doc.Pool, "music_recordings") == true {
		// music

		qp.Add("doc_type", "music")

		if len(author) > 0 {
			qp.Add("artist_name", author)
		}

		if len(doc.Title) > 0 {
			qp.Add("album_name", title)
		}
	} else {
		// books... and everything else

		qp.Add("doc_type", "non_music")

		if len(doc.Title) > 0 {
			qp.Add("title", title)
		}
	}

	// always throw these values at the cover image service

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

func (s *searchContext) virgoPopulateRecord(doc *solrDocument) *VirgoRecord {
	var r VirgoRecord

	// new style records -- order is important, primarily for generic "text" fields

	/**************************************** [ basic fields ] ****************************************/

	r.addBasicField(newField("id", s.client.localize("FieldIdentifier"), doc.ID).setType("identifier").setDisplay("optional"))

	// title / subtitle
	r.addBasicField(newField("title", s.client.localize("FieldTitle"), firstElementOf(doc.Title)).setType("title"))
	r.addBasicField(newField("subtitle", s.client.localize("FieldSubtitle"), firstElementOf(doc.Subtitle)).setType("subtitle"))

	// authors (principal and additional)
	for _, item := range s.getAuthorFieldValue(doc) {
		r.addBasicField(newField("author", s.client.localize(s.pool.config.solrAuthorLabel), item).setType("author"))
	}

	// publication date
	for _, item := range doc.PublicationDate {
		r.addBasicField(newField("publication_date", s.client.localize("FieldPublicationDate"), item))
	}

	// format
	for _, item := range doc.Format {
		r.addBasicField(newField("format", s.client.localize("FieldFormat"), item))
	}

	// availability
	availability := doc.AnonAvailability
	if s.client.isAuthenticated() == true {
		availability = doc.UVAAvailability
	}

	isAvailableOnShelf := false
	isAvailableOnline := false

	for _, item := range availability {
		if s.isExposedFacetValue(s.pool.solr.availableFacets["FacetAvailability"], item) {
			r.addBasicField(newField("availability", s.client.localize("FieldAvailability"), item).setType("availability"))

			switch {
			case strings.EqualFold(item, "On shelf") == true:
				isAvailableOnShelf = true
			case strings.EqualFold(item, "Online") == true:
				isAvailableOnline = true
			}
		}
	}

	// access info:

	if isAvailableOnShelf == true {
		// locations
		for _, item := range doc.Library {
			r.addBasicField(newField("library", s.client.localize("FieldLibrary"), item))
		}

		// sublocations
		for _, item := range doc.Location {
			r.addBasicField(newField("location", s.client.localize("FieldLocation"), item))
		}

		// NOTE: this might require client to determine availability... always returning it for now
		// call numbers (if physical and available)
		for _, item := range doc.CallNumber {
			r.addBasicField(newField("call_number", s.client.localize("FieldCallNumber"), item))
		}
	}

	if isAvailableOnline == true {
		// urls
		for _, item := range doc.URL {
			pieces := strings.Split(item, "||")
			r.addBasicField(newField("access_url", s.client.localize("FieldAccessURL"), pieces[0]).setType("url"))
		}
	}

	/**************************************** [ detailed fields ] ****************************************/

	// languages
	for _, item := range doc.Language {
		r.addDetailedField(newField("language", s.client.localize("FieldLanguage"), item))
	}

	// FIXME
	// editions

	// identifier(s)
	r.addDetailedField(newField("id", s.client.localize("FieldIdentifier"), doc.ID))

	// publication info
	for _, item := range doc.Published {
		r.addDetailedField(newField("published", s.client.localize("FieldPublished"), item))
	}

	// subject
	for _, item := range doc.Subject {
		r.addDetailedField(newField("subject", s.client.localize("FieldSubject"), item).setType("subject"))
	}

	// pool-specific detailed fields follow

	// series
	for _, item := range doc.Series {
		r.addDetailedField(newField("series", s.client.localize("FieldSeries"), item))
	}

	// genres
	for _, item := range doc.VideoGenre {
		r.addDetailedField(newField("genre", s.client.localize("FieldGenre"), item))
	}

	/*
		for _, item := range doc.CallNumberBroad {
			r.addDetailedField(newField("call_number_broad", s.client.localize("FieldCallNumberBroad"), item))
		}

		for _, item := range doc.CallNumberNarrow {
			r.addDetailedField(newField("call_number_narrow", s.client.localize("FieldCallNumberNarrow"), item))
		}

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

	/**************************************** [ special fields ] ****************************************/

	// virgo classic url

	if strings.HasPrefix(doc.ID, "u") {
		r.addDetailedField(newField("sirsi_url", s.client.localize("FieldDetailsURL"), s.getSirsiURL(doc.ID[1:])).setType("url"))
	}

	// cover image url

	if coverImageURL := s.getCoverImageURL(doc); coverImageURL != "" {
		r.addBasicField(newField("cover_image", "", coverImageURL).setType("image-url").setDisplay("optional"))
	}

	// add exact designator if applicable

	if s.itemIsExactMatch(doc) {
		r.Exact = true
	}

	// add internal info

	r.groupValue = s.getSolrGroupFieldValue(doc)

	// add debug info?
	if s.client.opts.debug == true {
		r.Debug = s.virgoPopulateRecordDebug(doc)
	}

	return &r
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

func (s *searchContext) virgoPopulateRecordList(solrDocuments *solrResponseDocuments) *VirgoRecords {
	var recordList VirgoRecords

	for _, doc := range solrDocuments.Docs {
		record := s.virgoPopulateRecord(&doc)

		recordList = append(recordList, *record)
	}

	return &recordList
}

func (s *searchContext) virgoPopulateFacetBucket(name string, value solrBucket) *VirgoFacetBucket {
	var bucket VirgoFacetBucket

	bucket.Value = value.Val
	bucket.Count = value.Count
	bucket.Selected = s.solrReq.meta.selectionMap[name][value.Val]

	return &bucket
}

func (s *searchContext) virgoPopulateFacet(facetDef poolFacetDefinition, value solrResponseFacet) *VirgoFacet {
	var facet VirgoFacet

	facet.ID = facetDef.Name
	facet.Name = s.client.localize(facet.ID)

	var buckets VirgoFacetBuckets

	for _, b := range value.Buckets {
		bucket := s.virgoPopulateFacetBucket(facetDef.Name, b)

		if s.isExposedFacetValue(facetDef, bucket.Value) {
			buckets = append(buckets, *bucket)
		}
	}

	// sort facet values alphabetically (they are queried by count, but we want to present a-z)

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Value < buckets[j].Value
	})

	facet.Buckets = buckets

	return &facet
}

func (s *searchContext) virgoPopulateFacetList(facetDefs map[string]poolFacetDefinition, solrFacets solrResponseFacets) *VirgoFacets {
	var facetList VirgoFacets
	gotFacet := false

	for key, val := range solrFacets {
		if len(val.Buckets) > 0 {
			gotFacet = true

			facet := s.virgoPopulateFacet(facetDefs[key], val)

			facetList = append(facetList, *facet)
		}
	}

	if gotFacet == false {
		return nil
	}

	// sort facet names alphabetically (Solr returns them randomly)

	sort.Slice(facetList, func(i, j int) bool {
		return facetList[i].Name < facetList[j].Name
	})

	return &facetList
}

func (s *searchContext) itemIsExactMatch(doc *solrDocument) bool {
	// encapsulates document-level exact-match logic for a given search

	// resource requests are not exact matches
	// FIXME: should find or create a better way to check for this
	if s.solrRes.meta.parserInfo == nil {
		return false
	}

	// case 1: a single title search query matches the first title in this document
	if s.solrRes.meta.parserInfo.isSingleTitleSearch == true {
		firstTitleResult := firstElementOf(doc.Title)

		titleQueried := firstElementOf(s.solrRes.meta.parserInfo.parser.Titles)

		if titlesAreEqual(titleQueried, firstTitleResult) {
			return true
		}
	}

	return false
}

func (s *searchContext) searchIsExactMatch() bool {
	// encapsulates search-level exact-match logic for a given search

	// cannot determine exactness if this is not the first page of results
	if s.solrRes.meta.start != 0 {
		return false
	}

	// cannot be exact if the first result does not satisfy exactness check
	if s.itemIsExactMatch(s.solrRes.meta.firstDoc) == false {
		return false
	}

	// first document is an exact match, but we need more checks

	// case 1: title searches must have multiple words, otherwise exactness determination is too aggressive
	if s.solrRes.meta.parserInfo.isSingleTitleSearch == true {
		titleQueried := firstElementOf(s.solrRes.meta.parserInfo.parser.Titles)

		if strings.Contains(titleQueried, " ") == false {
			return false
		}
	}

	return true
}

func (s *searchContext) virgoPopulatePoolResult() {
	var poolResult VirgoPoolResult

	poolResult.Identity = s.client.localizedPoolIdentity(s.pool)

	poolResult.Pagination = s.virgoPopulatePagination(s.solrRes.meta.start, s.solrRes.meta.numRows, s.solrRes.meta.totalRows)

	poolResult.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	// default confidence, when there are no results
	poolResult.Confidence = "low"

	if s.solrRes.meta.numRows > 0 {
		poolResult.RecordList = s.virgoPopulateRecordList(&s.solrRes.Response)

		// create h/m/l confidence levels from the query score

		// individual items can have exact match status, but overall confidence
		// level might be more restrictive, e.g. title searches need multiple words
		switch {
		case s.searchIsExactMatch():
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
		v = s.virgoPopulateRecord(s.solrRes.meta.firstDoc)

	default:
		return fmt.Errorf("Multiple items found")
	}

	s.virgoRecordRes = v

	return nil
}