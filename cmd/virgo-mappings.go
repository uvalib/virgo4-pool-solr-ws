package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/uvalib/virgo4-api/v4api"
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
	return firstElementOf(doc.getValuesByTag(s.pool.config.Local.Solr.GroupField))
}

func (s *searchContext) populateRecord(doc *solrDocument) v4api.Record {
	var r v4api.Record

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

		f := v4api.RecordField{
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

						r.Fields = append(r.Fields, f)
					}
				}

			case "authenticate":
				if anonRequest == true && anonOnline == false && authOnline == true {
					r.Fields = append(r.Fields, f)
				}

			case "availability":
				for _, availabilityValue := range availabilityValues {
					if sliceContainsString(s.pool.config.Global.Availability.ExposedValues, availabilityValue) {
						f.Value = availabilityValue
						r.Fields = append(r.Fields, f)
					}
				}

			case "cover_image":
				if s.pool.maps.attributes["cover_images"].Supported == true {
					if url := s.getCoverImageURL(field.CustomInfo.CoverImageURL, doc, authorValues); url != "" {
						f.Value = url
						r.Fields = append(r.Fields, f)
					}
				}

			case "digital_content_url":
				if url := s.getDigitalContentURL(doc, field.CustomInfo.DigitalContentURL.IDField); url != "" {
					f.Value = url
					r.Fields = append(r.Fields, f)
				}

			case "iiif_base_url":
				if url := s.getIIIFBaseURL(doc, field.CustomInfo.IIIFBaseURL.ImageField); url != "" {
					f.Value = url
					r.Fields = append(r.Fields, f)
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
							r.Fields = append(r.Fields, f)
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
						r.Fields = append(r.Fields, f)
					}
				}

			case "thumbnail_url":
				urlValues := doc.getValuesByTag(field.CustomInfo.ThumbnailURL.URLField)

				if len(urlValues) <= field.CustomInfo.ThumbnailURL.MaxSupported {
					for _, url := range urlValues {
						if url != "" {
							f.Value = url
							r.Fields = append(r.Fields, f)
						}
					}
				}

			}
		} else {
			fieldValues := doc.getValuesByTag(field.Field)

			for i, fieldValue := range fieldValues {
				f.Value = fieldValue
				r.Fields = append(r.Fields, f)

				if field.Limit > 0 && i+1 >= field.Limit {
					break
				}
			}
		}
	}

	// add internal info

	r.GroupValue = s.getSolrGroupFieldValue(doc)

	if s.client.opts.debug == true {
		r.Debug = make(map[string]interface{})
		r.Debug["score"] = doc.Score
	}

	return r
}

func (s *searchContext) populateRecords(solrDocuments *solrResponseDocuments) []v4api.Record {
	var records []v4api.Record

	for _, doc := range solrDocuments.Docs {
		record := s.populateRecord(&doc)

		records = append(records, record)
	}

	return records
}

func (s *searchContext) populateFacet(facetDef poolConfigFacet, value solrResponseFacet) v4api.Facet {
	var facet v4api.Facet

	facet.ID = facetDef.XID
	facet.Name = s.client.localize(facet.ID)
	facet.Type = facetDef.Type

	var buckets []v4api.FacetBucket

	switch facetDef.Type {
	case "boolean":
		selected := false
		if s.solr.req.meta.selectionMap[facetDef.XID][facetDef.Solr.Value] != "" {
			selected = true
		}

		buckets = append(buckets, v4api.FacetBucket{Selected: selected})

	default:
		for _, b := range value.Buckets {
			if len(facetDef.ExposedValues) == 0 || sliceContainsString(facetDef.ExposedValues, b.Val) {
				selected := false
				if s.solr.req.meta.selectionMap[facetDef.XID][b.Val] != "" {
					selected = true
				}

				buckets = append(buckets, v4api.FacetBucket{Value: b.Val, Count: b.Count, Selected: selected})
			}
		}

		// sort facet values alphabetically (they are queried by count, but we want to present a-z)

		sort.Slice(buckets, func(i, j int) bool {
			return buckets[i].Value < buckets[j].Value
		})
	}

	facet.Buckets = buckets

	return facet
}

func (s *searchContext) populateFacetList(solrFacets solrResponseFacets) []v4api.Facet {
	var facetList []v4api.Facet

	gotFacet := false

	for key, val := range solrFacets {
		if len(val.Buckets) > 0 {
			facetDef := s.pool.maps.availableFacets[key]

			// add this facet to the response as long as one of its dependent facets is selected

			if len(facetDef.DependentFacetXIDs) > 0 {
				numSelected := 0

				for _, facet := range facetDef.DependentFacetXIDs {
					n := len(s.solr.req.meta.selectionMap[facet])
					numSelected += n
				}

				if numSelected == 0 {
					s.log("[FACET] omitting facet [%s] due to lack of selected dependent filters", facetDef.XID)
					continue
				}

				s.log("[FACET] including facet [%s] due to %d selected dependent filters", facetDef.XID, numSelected)
			}

			gotFacet = true

			facet := s.populateFacet(facetDef, val)

			facetList = append(facetList, facet)
		}
	}

	if gotFacet == false {
		return nil
	}

	// sort facet names alphabetically (Solr returns them randomly)

	sort.Slice(facetList, func(i, j int) bool {
		return facetList[i].Name < facetList[j].Name
	})

	return facetList
}

func (s *searchContext) itemIsExactMatch(doc *solrDocument) bool {
	// encapsulates document-level exact-match logic for a given search

	// resource requests are not exact matches
	if s.itemDetails == true {
		return false
	}

	// this should be defined, but check just in case
	if s.solr.res.meta.parserInfo == nil {
		return false
	}

	// case 1: a single title search query matches the first title in this document
	if s.solr.res.meta.parserInfo.isSingleTitleSearch == true {
		firstTitleResult := firstElementOf(doc.getValuesByTag(s.pool.config.Local.Solr.ExactMatchTitleField))

		titleQueried := firstElementOf(s.solr.res.meta.parserInfo.titles)

		if titlesAreEqual(titleQueried, firstTitleResult) {
			return true
		}
	}

	return false
}

func (s *searchContext) searchIsExactMatch() bool {
	// encapsulates search-level exact-match logic for a given search

	// cannot determine exactness if this is not the first page of results
	if s.solr.res.meta.start != 0 {
		return false
	}

	// cannot be exact if the first result does not satisfy exactness check
	if s.itemIsExactMatch(s.solr.res.meta.firstDoc) == false {
		return false
	}

	// first document is an exact match, but we need more checks

	// case 1: title searches must have multiple words, otherwise exactness determination is too aggressive
	if s.solr.res.meta.parserInfo.isSingleTitleSearch == true {
		titleQueried := firstElementOf(s.solr.res.meta.parserInfo.titles)

		if strings.Contains(titleQueried, " ") == false {
			return false
		}
	}

	return true
}

// the main response functions for each endpoint

func (s *searchContext) buildPoolSearchResponse() error {
	var pr v4api.PoolResult

	//pr.Identity = s.client.localizedPoolIdentity(s.pool)

	pr.Pagination = v4api.Pagination{
		Start: s.solr.res.meta.start,
		Rows:  s.solr.res.meta.numRows,
		Total: s.solr.res.meta.totalRows,
	}

	pr.ElapsedMS = int64(time.Since(s.client.start) / time.Millisecond)

	// default confidence, when there are no results
	pr.Confidence = "low"

	if s.solr.res.meta.numRows > 0 {
		records := s.populateRecords(&s.solr.res.Response)

		group := v4api.Group{
			Records: records,
			Count:   len(records),
		}

		pr.Groups = append(pr.Groups, group)

		// create h/m/l confidence levels from the query score

		// individual items can have exact match status, but overall confidence
		// level might be more restrictive, e.g. title searches need multiple words
		switch {
		case s.searchIsExactMatch():
			pr.Confidence = "exact"
		case s.solr.res.meta.maxScore > s.pool.solr.scoreThresholdHigh:
			pr.Confidence = "high"
		case s.solr.res.meta.maxScore > s.pool.solr.scoreThresholdMedium:
			pr.Confidence = "medium"
		}
	}

	pr.FacetList = s.populateFacetList(s.solr.res.Facets)

	pr.Warnings = s.solr.res.meta.warnings

	if s.client.opts.debug == true {
		pr.Debug = make(map[string]interface{})
		pr.Debug["request_id"] = s.client.reqID
		pr.Debug["max_score"] = s.solr.res.meta.maxScore
	}

	s.virgo.poolRes = &pr

	return nil
}

func (s *searchContext) buildPoolRecordResponse() error {
	var r v4api.Record

	switch s.solr.res.meta.numRecords {
	case 0:
		return fmt.Errorf("record not found")

	case 1:
		r = s.populateRecord(s.solr.res.meta.firstDoc)

	default:
		return fmt.Errorf("multiple records found")
	}

	s.virgo.recordRes = &r

	return nil
}
