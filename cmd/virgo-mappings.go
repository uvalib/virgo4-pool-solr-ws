package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/uvalib/virgo4-api/v4api"
)

type fieldContext struct {
	config poolConfigField
	field  v4api.RecordField
}

type recordContext struct {
	doc                *solrDocument
	anonOnline         bool
	authOnline         bool
	availabilityValues []string
	isAvailableOnShelf bool
	anonRequest        bool
	hasDigitalContent  bool
	isSirsi            bool
	isWSLS             bool
	relations          categorizedRelations
	fieldCtx           fieldContext
}

// functions that map solr data into virgo data

func (s *solrDocument) getRawValue(field string) interface{} {
	return (*s)[field]
}

func (s *solrDocument) getStrings(field string) []string {
	// turn all potential values into string slices

	v := s.getRawValue(field)

	switch t := v.(type) {
	case []interface{}:
		vals := make([]string, len(t))
		for i, val := range t {
			vals[i] = val.(string)
		}
		return vals

	case []string:
		return t

	case string:
		return []string{t}

	case float32:
		return []string{fmt.Sprintf("%0.8f", t)}

	default:
		return []string{}
	}
}

func (s *solrDocument) getFirstString(field string) string {
	// shortcut to get first value for multi-value fields that really only ever contain one value
	return firstElementOf(s.getStrings(field))
}

func (s *solrDocument) getFloat(field string) float32 {
	v := s.getRawValue(field)

	switch t := v.(type) {
	case float32:
		return t

	default:
		return 0.0
	}
}

func (s *searchContext) getSolrGroupFieldValue(doc *solrDocument) string {
	return doc.getFirstString(s.pool.config.Local.Solr.GroupField)
}

func (s *searchContext) getFieldValues(rc *recordContext) []v4api.RecordField {
	var fields []v4api.RecordField

	// non-custom fields just return the raw solr values

	if rc.fieldCtx.config.Custom == false {
		for _, value := range rc.doc.getStrings(rc.fieldCtx.config.Field) {
			rc.fieldCtx.field.Value = value
			fields = append(fields, rc.fieldCtx.field)
		}

		return fields
	}

	// custom fields have per-field handling

	switch rc.fieldCtx.config.Name {
	case "abstract":
		return s.getCustomFieldAbstract(rc)

	case "access_url":
		return s.getCustomFieldAccessURL(rc)

	case "authenticate":
		return s.getCustomFieldAuthenticate(rc)

	case "author":
		return s.getCustomFieldAuthor(rc)

	case "author_list":
		return s.getCustomFieldAuthorList(rc)

	case "availability":
		return s.getCustomFieldAvailability(rc)

	case "citation_advisor":
		return s.getCustomFieldCitationAdvisor(rc)

	case "citation_author":
		return s.getCustomFieldCitationAuthor(rc)

	case "citation_compiler":
		return s.getCustomFieldCitationCompiler(rc)

	case "citation_editor":
		return s.getCustomFieldCitationEditor(rc)

	case "citation_format":
		return s.getCustomFieldCitationFormat(rc)

	case "citation_is_online_only":
		return s.getCustomFieldCitationIsOnlineOnly(rc)

	case "citation_is_virgo_url":
		return s.getCustomFieldCitationIsVirgoURL(rc)

	case "citation_subtitle":
		return s.getCustomFieldCitationSubtitle(rc)

	case "citation_title":
		return s.getCustomFieldCitationTitle(rc)

	case "citation_translator":
		return s.getCustomFieldCitationTranslator(rc)

	case "composer_performer":
		return s.getCustomFieldComposerPerformer(rc)

	case "copyright_and_permissions":
		return s.getCustomFieldCopyrightAndPermissions(rc)

	case "cover_image_url":
		return s.getCustomFieldCoverImageURL(rc)

	case "creator":
		return s.getCustomFieldCreator(rc)

	case "digital_content_url":
		return s.getCustomFieldDigitalContentURL(rc)

	case "language":
		return s.getCustomFieldLanguage(rc)

	case "online_related":
		return s.getCustomFieldOnlineRelated(rc)

	case "published_location":
		return s.getCustomFieldPublishedLocation(rc)

	case "publisher_name":
		return s.getCustomFieldPublisherName(rc)

	case "related_resources":
		return s.getCustomFieldRelatedResources(rc)

	case "sirsi_url":
		return s.getCustomFieldSirsiURL(rc)

	case "summary_holdings":
		return s.getCustomFieldSummaryHoldings(rc)

	case "title_subtitle_edition":
		return s.getCustomFieldTitleSubtitleEdition(rc)

	case "vernacularized_author":
		return s.getCustomFieldVernacularizedAuthor(rc)

	case "vernacularized_composer_performer":
		return s.getCustomFieldVernacularizedComposerPerformer(rc)

	case "vernacularized_creator":
		return s.getCustomFieldVernacularizedCreator(rc)

	case "vernacularized_title":
		return s.getCustomFieldVernacularizedTitle(rc)

	case "vernacularized_title_subtitle_edition":
		return s.getCustomFieldVernacularizedTitleSubtitleEdition(rc)

	case "wsls_collection_description":
		return s.getCustomFieldWSLSCollectionDescription(rc)

	default:
		s.warn("unhandled custom field: [%s]", rc.fieldCtx.config.Name)
	}

	return fields
}

func (s *searchContext) initializeRecordContext(doc *solrDocument) recordContext {
	var rc recordContext

	rc.doc = doc

	// availability setup

	anonValues := doc.getStrings(s.pool.config.Global.Availability.Anon.Field)
	anonOnShelf := sliceContainsAnyValueFromSlice(anonValues, s.pool.config.Global.Availability.Values.OnShelf, true)
	rc.anonOnline = sliceContainsAnyValueFromSlice(anonValues, s.pool.config.Global.Availability.Values.Online, true)

	authValues := doc.getStrings(s.pool.config.Global.Availability.Auth.Field)
	authOnShelf := sliceContainsAnyValueFromSlice(authValues, s.pool.config.Global.Availability.Values.OnShelf, true)
	rc.authOnline = sliceContainsAnyValueFromSlice(authValues, s.pool.config.Global.Availability.Values.Online, true)

	// determine which availability field to use

	rc.availabilityValues = anonValues
	rc.isAvailableOnShelf = anonOnShelf
	rc.anonRequest = true

	if s.client.isAuthenticated() == true {
		rc.availabilityValues = authValues
		rc.isAvailableOnShelf = authOnShelf
		rc.anonRequest = false
	}

	featureValues := doc.getStrings(s.pool.config.Global.RecordAttributes.DigitalContent.Field)
	rc.hasDigitalContent = sliceContainsAnyValueFromSlice(featureValues, s.pool.config.Global.RecordAttributes.DigitalContent.Contains, true)

	dataSourceValues := doc.getStrings(s.pool.config.Global.RecordAttributes.WSLS.Field)
	rc.isSirsi = sliceContainsAnyValueFromSlice(dataSourceValues, s.pool.config.Global.RecordAttributes.Sirsi.Contains, true)
	rc.isWSLS = sliceContainsAnyValueFromSlice(dataSourceValues, s.pool.config.Global.RecordAttributes.WSLS.Contains, true)

	// build parsed author lists from configured fields

	var preferredAuthorValues []string
	for _, field := range s.pool.config.Local.Solr.AuthorFields.Preferred {
		preferredAuthorValues = append(preferredAuthorValues, doc.getStrings(field)...)
	}

	var fallbackAuthorValues []string
	for _, field := range s.pool.config.Local.Solr.AuthorFields.Fallback {
		fallbackAuthorValues = append(fallbackAuthorValues, doc.getStrings(field)...)
	}

	rawAuthorValues := preferredAuthorValues
	if len(rawAuthorValues) == 0 {
		rawAuthorValues = fallbackAuthorValues
	}

	rc.relations = s.parseRelations(rawAuthorValues)

	return rc
}

func (s *searchContext) populateRecord(doc *solrDocument) v4api.Record {
	var record v4api.Record

	rc := s.initializeRecordContext(doc)

	// determine what fields we are extracting from the document

	var fieldCfgs []poolConfigField

	switch {
	case s.itemDetails == true:
		fieldCfgs = s.pool.fields.detailed

	default:
		fieldCfgs = s.pool.fields.basic
	}

	// field loop

	for _, fieldCfg := range fieldCfgs {
		if fieldCfg.OnShelfOnly == true && rc.isAvailableOnShelf == false {
			continue
		}

		if fieldCfg.DigitalContentOnly == true && rc.hasDigitalContent == false {
			continue
		}

		f := v4api.RecordField{
			Name:         fieldCfg.Name,
			Type:         fieldCfg.Properties.Type,
			Separator:    fieldCfg.Properties.Separator,
			Visibility:   fieldCfg.Properties.Visibility,
			Display:      fieldCfg.Properties.Display,
			Provider:     fieldCfg.Properties.Provider,
			CitationPart: fieldCfg.Properties.CitationPart,
		}

		if s.itemDetails == true {
			f.Visibility = "detailed"
		}

		if fieldCfg.XID != "" {
			if fieldCfg.WSLSXID != "" && rc.isWSLS == true {
				f.Label = s.client.localize(fieldCfg.WSLSXID)
			} else {
				f.Label = s.client.localize(fieldCfg.XID)
			}
		}

		rc.fieldCtx = fieldContext{config: fieldCfg, field: f}

		fieldValues := s.getFieldValues(&rc)

		if len(fieldValues) == 0 {
			continue
		}

		// split single field if configured
		if len(fieldValues) == 1 && fieldCfg.SplitOn != "" {
			origField := fieldValues[0]
			splitValues := strings.Split(origField.Value, fieldCfg.SplitOn)
			if len(splitValues) > 1 {
				// successful (?) split; go with it
				fieldValues = []v4api.RecordField{}
				for _, piece := range splitValues {
					newField := origField
					newField.Value = piece
					fieldValues = append(fieldValues, newField)
				}
			}
		}

		i := 0
		for _, fieldValue := range fieldValues {
			if fieldCfg.Custom == false && fieldValue.Value == "" {
				continue
			}

			if s.client.opts.citation == true {
				if fieldValue.CitationPart != "" {
					rf := v4api.RecordField{
						Name:         fieldValue.Name,
						Value:        fieldValue.Value,
						CitationPart: fieldValue.CitationPart,
					}

					record.Fields = append(record.Fields, rf)
				}
			} else {
				if fieldCfg.CitationOnly == false {
					rf := fieldValue
					rf.CitationPart = ""

					record.Fields = append(record.Fields, rf)
				}
			}

			if fieldCfg.Limit > 0 && i+1 >= fieldCfg.Limit {
				break
			}

			i++
		}
	}

	// add internal info

	record.GroupValue = s.getSolrGroupFieldValue(doc)

	if s.client.opts.debug == true {
		record.Debug = make(map[string]interface{})
		record.Debug["score"] = doc.getFloat("score")
	}

	return record
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

	facet.Sort = facetDef.BucketSort

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
			if len(facetDef.ExposedValues) == 0 || sliceContainsString(facetDef.ExposedValues, b.Val, false) {
				mappedValue := b.Val

				// if this is a mapped value facet, retrieved the translation for its mapping
				if len(facetDef.ValueXIDs) > 0 {
					xid := facetDef.valueToXIDMap[b.Val]
					if xid == "" {
						s.warn("FACET: %s: ignoring unmapped solr value: [%s]", facetDef.XID, b.Val)
						continue
					}
					mappedValue = s.client.localize(xid)
				}

				selected := false
				if s.solr.req.meta.selectionMap[facetDef.XID][b.Val] != "" {
					selected = true
				}

				buckets = append(buckets, v4api.FacetBucket{Value: mappedValue, Count: b.Count, Selected: selected})
			}
		}

		// sort facet bucket values per configuration

		switch facetDef.BucketSort {
		case "alpha":
			sort.Slice(buckets, func(i, j int) bool {
				// bucket values are unique so this is the only test we need
				return buckets[i].Value < buckets[j].Value
			})

		case "count":
			sort.Slice(buckets, func(i, j int) bool {
				if buckets[i].Count > buckets[j].Count {
					return true
				}

				if buckets[i].Count < buckets[j].Count {
					return false
				}

				// items with the same count get sorted alphabetically for consistency
				return buckets[i].Value < buckets[j].Value
			})

		default:
		}
	}

	facet.Buckets = buckets

	return facet
}

func (s *searchContext) populateFacetList(solrFacets map[string]solrResponseFacet) []v4api.Facet {
	type indexedFacet struct {
		index int
		facet v4api.Facet
	}

	// first, convert component query facets back to internal facets by
	// creating buckets for each component with the translated value

	mergedFacets := make(map[string]solrResponseFacet)
	componentQueries := make(map[string]map[string]*solrResponseFacet)

	// add normal facets; track component facets
	for key := range solrFacets {
		val := solrFacets[key]

		switch s.solr.req.meta.requestFacets[key].config.Type {
		case "component":
			xid := s.solr.req.meta.requestFacets[key].config.XID
			if componentQueries[xid] == nil {
				componentQueries[xid] = make(map[string]*solrResponseFacet)
			}
			componentQueries[xid][key] = &val

		default:
			mergedFacets[key] = val
		}
	}

	// add component query facets, in the order they were defined
	for key, val := range componentQueries {
		var facet solrResponseFacet

		for _, q := range s.solr.req.meta.internalFacets[key].config.ComponentQueries {
			qval := val[q.XID]
			if qval == nil || qval.Count == 0 {
				continue
			}

			bucket := solrBucket{
				Val:        s.client.localize(q.XID),
				Count:      qval.Count,
				GroupCount: qval.GroupCount,
			}

			facet.Buckets = append(facet.Buckets, bucket)
		}

		mergedFacets[key] = facet
	}

	// now, convert these to external facets
	var orderedFacets []indexedFacet

	gotFacet := false

	for key, val := range mergedFacets {
		if len(val.Buckets) > 0 {
			var facetDef poolConfigFacet
			if s.virgo.flags.preSearchFilters == true {
				facetDef = s.pool.maps.filters[key]
			} else {
				facetDef = s.pool.maps.facets[key]
			}

			// add this facet to the response as long as one of its dependent facets is selected

			if len(facetDef.DependentFacetXIDs) > 0 {
				numSelected := 0

				for _, facet := range facetDef.DependentFacetXIDs {
					n := len(s.solr.req.meta.selectionMap[facet])
					numSelected += n
				}

				if numSelected == 0 {
					s.log("FACET: omitting facet [%s] due to lack of selected dependent filters", facetDef.XID)
					continue
				}

				s.log("FACET: including facet [%s] due to %d selected dependent filters", facetDef.XID, numSelected)
			}

			gotFacet = true

			facet := s.populateFacet(facetDef, val)

			orderedFacets = append(orderedFacets, indexedFacet{index: facetDef.Index, facet: facet})
		}
	}

	if gotFacet == false {
		return nil
	}

	// sort facet names in the same order the pool config lists them (Solr returns them randomly)

	sort.Slice(orderedFacets, func(i, j int) bool {
		return orderedFacets[i].index < orderedFacets[j].index
	})

	var facetList []v4api.Facet
	for _, f := range orderedFacets {
		facetList = append(facetList, f.facet)
	}

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
		firstTitleResult := doc.getFirstString(s.pool.config.Local.Solr.ExactMatchTitleField)

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

func (s *searchContext) buildPoolSearchResponse() searchResponse {
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
		//pr.Debug["solr"] = s.solr.res.Debug
	}

	s.virgo.poolRes = &pr

	return searchResponse{status: http.StatusOK}
}

func (s *searchContext) buildPoolRecordResponse() searchResponse {
	r := s.populateRecord(s.solr.res.meta.firstDoc)

	s.virgo.recordRes = &r

	return searchResponse{status: http.StatusOK}
}
