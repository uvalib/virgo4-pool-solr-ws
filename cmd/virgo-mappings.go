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

func (s *solrDocument) getFieldByTag(tag string) interface{} {
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

func (s *solrDocument) getValuesByTag(tag string) []string {
	// turn all potential values into string slices

	v := s.getFieldByTag(tag)

	switch t := v.(type) {
	case []string:
		return t

	case string:
		return []string{t}

	case float32:
		// in case this is ever called for fields such as 'score'
		return []string{fmt.Sprintf("%0.8f", t)}

	default:
		return []string{}
	}
}

func (s *searchContext) getSolrGroupFieldValue(doc *solrDocument) string {
	return firstElementOf(doc.getValuesByTag(s.pool.config.Solr.Grouping.Field))
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

func (g *VirgoGroup) addField(f *VirgoNuancedField) {
	if f.Name == "" {
		return
	}

	g.Fields = append(g.Fields, *f)
}

func getGenericURL(t poolConfigURLTemplate, id string) string {
	if strings.Contains(t.Template, t.Pattern) == false {
		return ""
	}

	return strings.Replace(t.Template, t.Pattern, id, -1)
}

func (s *searchContext) getSirsiURL(id string) string {
	return getGenericURL(s.pool.config.Service.URLTemplates.Sirsi, id)
}

func (s *searchContext) getCoverImageURL(doc *solrDocument, authorValues []string) string {
	// use solr-provided url if present

	if thumbnailURL := firstElementOf(doc.ThumbnailURL); thumbnailURL != "" {
		return thumbnailURL
	}

	// otherwise, compose a url to the cover image service

	url := getGenericURL(s.pool.config.Service.URLTemplates.CoverImages, doc.ID)

	if url == "" {
		return ""
	}

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

func (s *searchContext) virgoPopulateRecord(doc *solrDocument) *VirgoRecord {
	var r VirgoRecord

	var authorField []string

	// availability setup

	anonField := doc.getValuesByTag(s.pool.config.Availability.Anon.Field)
	anonOnShelf := sliceContainsValueFromSlice(anonField, s.pool.config.Availability.Values.OnShelf)
	anonOnline := sliceContainsValueFromSlice(anonField, s.pool.config.Availability.Values.Online)

	authField := doc.getValuesByTag(s.pool.config.Availability.Auth.Field)
	authOnShelf := sliceContainsValueFromSlice(authField, s.pool.config.Availability.Values.OnShelf)
	authOnline := sliceContainsValueFromSlice(authField, s.pool.config.Availability.Values.Online)

	// determine which availability field to use

	availability := anonField
	isAvailableOnShelf := anonOnShelf
	anonRequest := true

	if s.client.isAuthenticated() == true {
		availability = authField
		isAvailableOnShelf = authOnShelf
		anonRequest = false
	}

	// flag requests that may need authentication to access this resource

	if anonRequest == true && anonOnline == false && authOnline == true {
		f := &VirgoNuancedField{
			Name:    "authenticate",
			Type:    "boolean",
			Display: "optional",
		}

		r.addField(f)
	}

	// field loop

	for _, field := range s.pool.config.Fields {
		if field.DetailsOnly && s.itemDetails == false {
			continue
		}

		if field.OnShelfOnly && isAvailableOnShelf == false {
			continue
		}

		f := &VirgoNuancedField{
			Name:       field.Name,
			Type:       field.Type,
			Visibility: field.Visibility,
			Display:    field.Display,
			Provider:   field.Provider,
		}

		if field.XID != "" {
			f.Label = s.client.localize(field.XID)
		}

		if field.Field != "" {
			value := doc.getValuesByTag(field.Field)

			if len(value) == 0 {
				continue
			}

			// save for later use (e.g. cover image url)
			if field.Name == "author" {
				authorField = value
			}

			for i, item := range value {
				f.Value = item
				r.addField(f)

				if field.Limit > 0 && i+1 >= field.Limit {
					break
				}
			}
		} else {
			switch field.Name {
			case "access_url":
				if anonOnline == true || authOnline == true {
					urlField := doc.getValuesByTag(field.URLField)
					labelField := doc.getValuesByTag(field.LabelField)
					providerField := doc.getValuesByTag(field.ProviderField)

					f.Provider = firstElementOf(providerField)

					useLabels := false
					if len(labelField) == len(urlField) {
						useLabels = true
					}

					for i, item := range urlField {
						f.Value = item

						itemLabel := ""

						if useLabels == true {
							itemLabel = labelField[i]
						}

						// if not using labels, or this label is not defined, fall back to generic item label
						if itemLabel == "" {
							itemLabel = fmt.Sprintf("%s %d", s.client.localize("FieldAccessURLDefaultItemLabelPrefix"), i+1)
						}

						f.Item = itemLabel

						r.addField(f)
					}
				}

			case "availability":
				for _, item := range availability {
					if sliceContainsString(s.pool.config.Availability.ExposedValues, item) {
						f.Value = item
						r.addField(f)
					}
				}

			case "cover_image":
				if s.pool.maps.attributes["cover_images"].Supported == true {
					if url := s.getCoverImageURL(doc, authorField); url != "" {
						f.Value = url
						r.addField(f)
					}
				}

			case "iiif_base_url":
				f.Value = getIIIFBaseURL(doc, field.IdentifierField)
				r.addField(f)

			case "sirsi_url":
				if strings.HasPrefix(doc.ID, "u") {
					if url := s.getSirsiURL(doc.ID[1:]); url != "" {
						f.Value = url
						r.addField(f)
					}
				}

			default:
				s.log("WARNING: unhandled field: %s", field.Name)
			}
		}
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

func (s *searchContext) virgoPopulateFacet(facetDef poolConfigFacet, value solrResponseFacet) *VirgoFacet {
	var facet VirgoFacet

	facet.ID = facetDef.XID
	facet.Name = s.client.localize(facet.ID)
	facet.Type = facetDef.Type

	var buckets VirgoFacetBuckets

	switch facetDef.Type {
	case "boolean":
		bucket := VirgoFacetBucket{
			Selected: s.solrReq.meta.selectionMap[facetDef.XID][facetDef.Solr.Value],
		}

		buckets = append(buckets, bucket)

	default:
		for _, b := range value.Buckets {
			bucket := s.virgoPopulateFacetBucket(facetDef.XID, b)

			if len(facetDef.ExposedValues) == 0 || sliceContainsString(facetDef.ExposedValues, bucket.Value) {
				buckets = append(buckets, *bucket)
			}
		}

		// sort facet values alphabetically (they are queried by count, but we want to present a-z)

		sort.Slice(buckets, func(i, j int) bool {
			return buckets[i].Value < buckets[j].Value
		})
	}

	facet.Buckets = buckets

	return &facet
}

func (s *searchContext) virgoPopulateFacetList(solrFacets solrResponseFacets) *VirgoFacets {
	var facetList VirgoFacets
	gotFacet := false

	for key, val := range solrFacets {
		if len(val.Buckets) > 0 {
			facetDef := s.pool.maps.availableFacets[key]

			// add this facet to the response as long as one of its dependent facets is selected

			if len(facetDef.DependentFacetXIDs) > 0 {
				numSelected := 0

				for _, facet := range facetDef.DependentFacetXIDs {
					n := len(s.solrReq.meta.selectionMap[facet])
					s.log("virgoPopulateFacetList(): [%s] %d selected filters for %s", facetDef.XID, n, facet)
					numSelected += n
				}

				if numSelected == 0 {
					s.log("virgoPopulateFacetList(): [%s] omitting facet due to lack of selected dependent filters", facetDef.XID)
					continue
				}

				s.log("virgoPopulateFacetList(): [%s] including facet due to %d selected dependent filters", facetDef.XID, numSelected)
			}

			gotFacet = true

			facet := s.virgoPopulateFacet(facetDef, val)

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

		titleQueried := firstElementOf(s.solrRes.meta.parserInfo.titles)

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
		titleQueried := firstElementOf(s.solrRes.meta.parserInfo.titles)

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
		poolResult.FacetList = s.virgoPopulateFacetList(s.solrRes.Facets)
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
		s.itemDetails = true
		v = s.virgoPopulateRecord(s.solrRes.meta.firstDoc)

	default:
		return fmt.Errorf("Multiple items found")
	}

	s.virgoRecordRes = v

	return nil
}
