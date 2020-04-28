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
	return firstElementOf(doc.getValuesByTag(s.pool.config.Local.Solr.Grouping.Field))
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
	return getGenericURL(s.pool.config.Global.Service.URLTemplates.Sirsi, id)
}

func (s *searchContext) getCoverImageURL(cfg *poolConfigFieldTypeCoverImageURL, doc *solrDocument, authorValues []string) string {
	// use solr-provided url if present

	thumbnailValues := doc.getValuesByTag(cfg.ThumbnailField)

	if thumbnailURL := firstElementOf(thumbnailValues); thumbnailURL != "" {
		return thumbnailURL
	}

	// otherwise, compose a url to the cover image service

	idValues := doc.getValuesByTag(cfg.IDField)

	url := getGenericURL(s.pool.config.Global.Service.URLTemplates.CoverImages, firstElementOf(idValues))

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

	titleValues := doc.getValuesByTag(cfg.TitleField)
	poolValues := doc.getValuesByTag(cfg.PoolField)

	// remove extraneous dates from author
	author := strings.Trim(strings.Split(firstElementOf(authorValues), "[")[0], " ")
	title := firstElementOf(titleValues)

	if sliceContainsString(poolValues, cfg.MusicPool) == true {
		// music

		qp.Add("doc_type", "music")

		if len(author) > 0 {
			qp.Add("artist_name", author)
		}

		if len(title) > 0 {
			qp.Add("album_name", title)
		}
	} else {
		// books... and everything else

		qp.Add("doc_type", "non_music")

		if len(title) > 0 {
			qp.Add("title", title)
		}
	}

	// always throw these optional values at the cover image service

	isbnValues := doc.getValuesByTag(cfg.ISBNField)
	if len(isbnValues) > 0 {
		qp.Add("isbn", strings.Join(isbnValues, ","))
	}

	oclcValues := doc.getValuesByTag(cfg.OCLCField)
	if len(oclcValues) > 0 {
		qp.Add("oclc", strings.Join(oclcValues, ","))
	}

	lccnValues := doc.getValuesByTag(cfg.LCCNField)
	if len(lccnValues) > 0 {
		qp.Add("lccn", strings.Join(lccnValues, ","))
	}

	upcValues := doc.getValuesByTag(cfg.UPCField)
	if len(upcValues) > 0 {
		qp.Add("upc", strings.Join(upcValues, ","))
	}

	req.URL.RawQuery = qp.Encode()

	return req.URL.String()
}

func (s *searchContext) getIIIFBaseURL(doc *solrDocument, idField string) string {
	// FIXME: update after iiif_image_url is correct

	// construct iiif image base url from known image identifier prefixes.
	// this fallback url conveniently points to an "orginial image missing" image

	pid := s.pool.config.Global.Service.URLTemplates.IIIF.Fallback

	idValues := doc.getValuesByTag(idField)

	for _, id := range idValues {
		for _, prefix := range s.pool.config.Global.Service.URLTemplates.IIIF.Prefixes {
			if strings.HasPrefix(id, prefix) {
				pid = id
				break
			}
		}
	}

	return getGenericURL(s.pool.config.Global.Service.URLTemplates.IIIF, pid)
}

func (s *searchContext) getDigitalContentURL(doc *solrDocument, idField string) string {
	idValues := doc.getValuesByTag(idField)

	id := firstElementOf(idValues)

	return getGenericURL(s.pool.config.Global.Service.URLTemplates.DigitalContent, id)
}

func (s *searchContext) virgoPopulateRecord(doc *solrDocument) *VirgoRecord {
	var r VirgoRecord

	var authorValues []string

	// availability setup

	anonValues := doc.getValuesByTag(s.pool.config.Global.Availability.Anon.Field)
	anonOnShelf := sliceContainsValueFromSlice(anonValues, s.pool.config.Global.Availability.Values.OnShelf)
	anonOnline := sliceContainsValueFromSlice(anonValues, s.pool.config.Global.Availability.Values.Online)

	authValues := doc.getValuesByTag(s.pool.config.Global.Availability.Auth.Field)
	authOnShelf := sliceContainsValueFromSlice(authValues, s.pool.config.Global.Availability.Values.OnShelf)
	authOnline := sliceContainsValueFromSlice(authValues, s.pool.config.Global.Availability.Values.Online)

	// determine which availability field to use

	availabilityValues := anonValues
	isAvailableOnShelf := anonOnShelf
	anonRequest := true

	if s.client.isAuthenticated() == true {
		availabilityValues = authValues
		isAvailableOnShelf = authOnShelf
		anonRequest = false
	}

	featureValues := doc.getValuesByTag(s.pool.config.Global.Service.DigitalContent.FeatureField)
	hasDigitalContent := sliceContainsValueFromSlice(featureValues, s.pool.config.Global.Service.DigitalContent.Features)

	// field loop (preprocessing)

	for _, field := range s.pool.config.Mappings.Fields {
		if field.Field != "" && field.Properties.Type == "author" {
			authorValues = doc.getValuesByTag(field.Field)
		}
	}

	// field loop

	for _, field := range s.pool.config.Mappings.Fields {
		if field.DetailsOnly == true && s.itemDetails == false {
			continue
		}

		if field.OnShelfOnly == true && isAvailableOnShelf == false {
			continue
		}

		if field.DigitalContentOnly == true && hasDigitalContent == false {
			continue
		}

		f := &VirgoNuancedField{
			Name:       field.Name,
			Type:       field.Properties.Type,
			Visibility: field.Properties.Visibility,
			Display:    field.Properties.Display,
			Provider:   field.Properties.Provider,
			RISCode:    field.Properties.RISCode,
		}

		if field.XID != "" {
			f.Label = s.client.localize(field.XID)
		}

		if field.Properties.RISCode == "" && field.Field != "" {
			f.RISCode = s.pool.maps.risCodes[field.Field]
		}

		if field.Custom == true {
			switch field.Name {
			case "access_url":
				if anonOnline == true || authOnline == true {
					urlValues := doc.getValuesByTag(field.CustomInfo.AccessURL.URLField)
					labelValues := doc.getValuesByTag(field.CustomInfo.AccessURL.LabelField)
					providerValues := doc.getValuesByTag(field.CustomInfo.AccessURL.ProviderField)

					f.Provider = firstElementOf(providerValues)

					useLabels := false
					if len(labelValues) == len(urlValues) {
						useLabels = true
					}

					for i, item := range urlValues {
						f.Value = item

						itemLabel := ""

						if useLabels == true {
							itemLabel = labelValues[i]
						}

						// if not using labels, or this label is not defined, fall back to generic item label
						if itemLabel == "" {
							itemLabel = fmt.Sprintf("%s %d", s.client.localize(field.CustomInfo.AccessURL.DefaultItemXID), i+1)
						}

						f.Item = itemLabel

						r.addField(f)
					}
				}

			case "authenticate":
				if anonRequest == true && anonOnline == false && authOnline == true {
					r.addField(f)
				}

			case "availability":
				for _, availabilityValue := range availabilityValues {
					if sliceContainsString(s.pool.config.Global.Availability.ExposedValues, availabilityValue) {
						f.Value = availabilityValue
						r.addField(f)
					}
				}

			case "cover_image":
				if s.pool.maps.attributes["cover_images"].Supported == true {
					if url := s.getCoverImageURL(field.CustomInfo.CoverImageURL, doc, authorValues); url != "" {
						f.Value = url
						r.addField(f)
					}
				}

			case "digital_content_url":
				if url := s.getDigitalContentURL(doc, field.CustomInfo.DigitalContentURL.IDField); url != "" {
					f.Value = url
					r.addField(f)
				}

			case "iiif_base_url":
				if url := s.getIIIFBaseURL(doc, field.CustomInfo.IIIFBaseURL.IdentifierField); url != "" {
					f.Value = url
					r.addField(f)
				}

			case "pdf_download_url":
				pidValues := doc.getValuesByTag(field.CustomInfo.PdfDownloadURL.PIDField)

				if len(pidValues) <= field.CustomInfo.PdfDownloadURL.MaxSupported {
					pdfURL := firstElementOf(doc.getValuesByTag(field.CustomInfo.PdfDownloadURL.URLField))

					if pdfURL == "" {
						continue
					}

					for _, pid := range pidValues {
						if pid == "" {
							continue
						}

						statusURL := fmt.Sprintf("%s/%s%s", pdfURL, pid, s.pool.config.Global.Service.Pdf.Endpoints.Status)

						pdfStatus, pdfErr := s.getPdfStatus(statusURL)

						if pdfErr != nil {
							continue
						}

						if sliceContainsString(s.pool.config.Global.Service.Pdf.ReadyValues, pdfStatus) == true {
							downloadURL := fmt.Sprintf("%s/%s%s", pdfURL, pid, s.pool.config.Global.Service.Pdf.Endpoints.Download)
							f.Value = downloadURL
							r.addField(f)
						}
					}
				}

			case "sirsi_url":
				idValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.SirsiURL.IDField))
				idPrefix := field.CustomInfo.SirsiURL.IDPrefix

				if strings.HasPrefix(idValue, idPrefix) {
					sirsiID := idValue[len(idPrefix):]
					if url := s.getSirsiURL(sirsiID); url != "" {
						f.Value = url
						r.addField(f)
					}
				}

			case "thumbnail_url":
				urlValues := doc.getValuesByTag(field.CustomInfo.ThumbnailURL.URLField)

				if len(urlValues) <= field.CustomInfo.ThumbnailURL.MaxSupported {
					for _, url := range urlValues {
						if url != "" {
							f.Value = url
							r.addField(f)
						}
					}
				}

			}
		} else {
			fieldValues := doc.getValuesByTag(field.Field)

			for i, fieldValue := range fieldValues {
				f.Value = fieldValue
				r.addField(f)

				if field.Limit > 0 && i+1 >= field.Limit {
					break
				}
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

	selected := false
	if s.solrReq.meta.selectionMap[name][value.Val] != "" {
		selected = true
	}

	bucket.Value = value.Val
	bucket.Count = value.Count
	bucket.Selected = selected

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
		selected := false
		if s.solrReq.meta.selectionMap[facetDef.XID][facetDef.Solr.Value] != "" {
			selected = true
		}

		bucket := VirgoFacetBucket{
			Selected: selected,
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
					numSelected += n
				}

				if numSelected == 0 {
					s.log("[FACET] omitting facet [%s] due to lack of selected dependent filters", facetDef.XID)
					continue
				}

				s.log("[FACET] including facet [%s] due to %d selected dependent filters", facetDef.XID, numSelected)
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
	if s.itemDetails == true {
		return false
	}

	// this should be defined, but check just in case
	if s.solrRes.meta.parserInfo == nil {
		return false
	}

	// case 1: a single title search query matches the first title in this document
	if s.solrRes.meta.parserInfo.isSingleTitleSearch == true {
		firstTitleResult := firstElementOf(doc.getValuesByTag(s.pool.config.Local.Solr.ExactMatchTitleField))

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
		return fmt.Errorf("record not found")

	case 1:
		v = s.virgoPopulateRecord(s.solrRes.meta.firstDoc)

	default:
		return fmt.Errorf("multiple records found")
	}

	s.virgoRecordRes = v

	return nil
}
