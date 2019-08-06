package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	//log "github.com/sirupsen/logrus"
)

const defaultScoreThresholdMedium = 100.0
const defaultScoreThresholdHigh = 200.0

// functions that map solr data into virgo data

func scoreWithFallback(s string, fallback float32) float32 {
	score := fallback

	if f, err := strconv.ParseFloat(s, 32); err == nil {
		score = float32(f)
	} else {
		log.Printf("could not convert [%s] to float; falling back to %0.1f", fallback)
	}

	return score
}

func getScoreThresholds() (float32, float32) {
	medium := scoreWithFallback(pool.config.scoreThresholdMedium, defaultScoreThresholdMedium)
	high := scoreWithFallback(pool.config.scoreThresholdHigh, defaultScoreThresholdHigh)

	if medium >= high {
		medium, high = high, medium
	}

	log.Printf("score thresholds: medium = %0.1f, high = %0.1f", medium, high)

	return medium, high
}

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
	r.addField(f.setVisibility("basic"))
}

func (r *VirgoRecord) addDetailedField(f *VirgoNuancedField) {
	r.addField(f.setVisibility("detailed"))
}

func newField(name, label, value string) *VirgoNuancedField {
	field := VirgoNuancedField{
		Name:       name,
		Type:       "text",
		Label:      label,
		Value:      value,
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

func virgoPopulateRecord(doc *solrDocument, client *clientOptions) *VirgoRecord {
	var r VirgoRecord

	// new style records -- order is important!

	r.addBasicField(newField("id", "Identifier", doc.ID))
	r.addBasicField(newField("title", "Title", firstElementOf(doc.Title)))
	r.addBasicField(newField("subtitle", "Subtitle", firstElementOf(doc.Subtitle)))

	for _, item := range doc.Author {
		r.addBasicField(newField("author", "Author", item))
	}

	for _, item := range doc.Subject {
		r.addDetailedField(newField("subject", "Subject", item))
	}

	for _, item := range doc.Language {
		r.addDetailedField(newField("language", "Language", item))
	}

	for _, item := range doc.Format {
		r.addDetailedField(newField("format", "Format", item))
	}

	for _, item := range doc.Library {
		r.addDetailedField(newField("library", "Library", item))
	}

	for _, item := range doc.CallNumber {
		r.addDetailedField(newField("call_number", "Call Number", item))
	}

	for _, item := range doc.CallNumberBroad {
		r.addDetailedField(newField("call_number_broad", "Call Number (Broad)", item))
	}

	for _, item := range doc.CallNumberNarrow {
		r.addDetailedField(newField("call_number_narrow", "Call Number (Narrow)", item))
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

func virgoPopulateFacetBucket(value solrBucket, client *clientOptions) *VirgoFacetBucket {
	var bucket VirgoFacetBucket

	bucket.Value = value.Val
	bucket.Count = value.Count

	return &bucket
}

func virgoPopulateFacet(name string, value solrResponseFacet, client *clientOptions) *VirgoFacet {
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

func virgoPopulatePoolResultDebug(solrRes *solrResponse, client *clientOptions) *VirgoPoolResultDebug {
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

func virgoPopulateGroup(solrGroup *solrResponseGroup, client *clientOptions) *VirgoGroup {
	var group VirgoGroup

	group.Value = solrGroup.GroupValue
	group.Count = len(solrGroup.DocList.Docs)

	for _, doc := range solrGroup.DocList.Docs {
		record := virgoPopulateRecord(&doc, client)

		group.RecordList = append(group.RecordList, *record)
	}

	return &group
}

func virgoPopulateGroupList(solrGrouping *solrResponseGrouping, client *clientOptions) *[]VirgoGroup {
	var groupList []VirgoGroup

	for _, g := range solrGrouping.Groups {
		group := virgoPopulateGroup(&g, client)

		groupList = append(groupList, *group)
	}

	return &groupList
}

func virgoPopulateRecordList(solrDocuments *solrResponseDocuments, client *clientOptions) *[]VirgoRecord {
	var recordList []VirgoRecord

	for _, doc := range solrDocuments.Docs {
		record := virgoPopulateRecord(&doc, client)

		recordList = append(recordList, *record)
	}

	return &recordList
}

func virgoPopulateFacetList(solrFacets solrResponseFacets, client *clientOptions) *[]VirgoFacet {
	var facetList []VirgoFacet
	gotFacet := false

	for key, val := range solrFacets {
		if len(val.Buckets) > 0 {
			gotFacet = true

			facet := virgoPopulateFacet(key, val, client)

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

	poolResult.ServiceURL = s.pool.config.poolServiceURL

	poolResult.Pagination = virgoPopulatePagination(s.solrRes.meta.start, s.solrRes.meta.numRows, s.solrRes.meta.totalRows)

	poolResult.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	firstTitleResults := ""
	firstTitleQueried := firstElementOf(s.solrRes.meta.parserInfo.parser.Titles)

	// default confidence, when there are no results
	poolResult.Confidence = "low"

	if s.solrRes.meta.numRows > 0 {
		if s.client.grouped == true {
			poolResult.GroupList = virgoPopulateGroupList(&s.solrRes.Grouped.WorkTitle2KeySort, s.client)
		} else {
			poolResult.RecordList = virgoPopulateRecordList(&s.solrRes.Response, s.client)
		}

		// FIXME: somehow create h/m/l confidence levels from the query score
		firstTitleResults = firstElementOf(s.solrRes.meta.firstDoc.Title)

		scoreThresholdMedium, scoreThresholdHigh := getScoreThresholds()

		switch {
		case s.solrRes.meta.start == 0 && s.solrRes.meta.parserInfo.isTitleSearch && titlesAreEqual(firstTitleResults, firstTitleQueried):
			poolResult.Confidence = "exact"
		case s.solrRes.meta.maxScore > scoreThresholdHigh:
			poolResult.Confidence = "high"
		case s.solrRes.meta.maxScore > scoreThresholdMedium:
			poolResult.Confidence = "medium"
		}
	}

	if len(s.solrRes.Facets) > 0 {
		poolResult.FacetList = virgoPopulateFacetList(s.solrRes.Facets, s.client)
	}

	// advertise facets?
	if s.solrRes.meta.advertiseFacets == true {
		poolResult.AvailableFacets = &s.pool.solr.virgoAvailableFacets
	}

	if len(s.solrRes.meta.warnings) > 0 {
		poolResult.Warn = &s.solrRes.meta.warnings
	}

	if s.client.debug == true {
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
		return errors.New("Item not found")

	case s.client.grouped == true && s.solrRes.meta.numGroups == 1 && s.solrRes.meta.numRecords == 1:
		v = virgoPopulateRecord(s.solrRes.meta.firstDoc, s.client)

	case s.client.grouped == false && s.solrRes.meta.numRecords == 1:
		v = virgoPopulateRecord(s.solrRes.meta.firstDoc, s.client)

	default:
		return errors.New("Multiple items found")
	}

	s.virgoRecordRes = v

	return nil
}
