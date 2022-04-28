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
	doc                 *solrDocument
	resourceTypeCtx     *poolConfigResourceTypeContext
	anonOnline          bool
	authOnline          bool
	availabilityValues  []string
	isAvailableOnShelf  bool
	anonRequest         bool
	hasDigitalContent   bool
	hasVernacularTitle  bool
	hasVernacularAuthor bool
	isSirsi             bool
	isWSLS              bool
	titleize            bool
	relations           categorizedRelations
	fieldCtx            fieldContext
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

func (s *searchContext) getSolrIdentifierFieldValue(doc *solrDocument) string {
	return doc.getFirstString(s.pool.config.Local.Solr.IdentifierField)
}

func (s *searchContext) getSolrGroupFieldValue(doc *solrDocument) string {
	return doc.getFirstString(s.pool.config.Local.Solr.GroupField)
}

func (s *searchContext) getFieldValues(rc *recordContext) []v4api.RecordField {
	var fields []v4api.RecordField

	// non-custom fields just return the explicit or raw solr values

	if rc.fieldCtx.config.CustomConfig == nil {
		if rc.fieldCtx.config.Value != "" {
			// explicitly configured value
			rc.fieldCtx.field.Value = rc.fieldCtx.config.Value
			fields = append(fields, rc.fieldCtx.field)
		} else {
			// solr-based value(s)
			for _, value := range rc.doc.getStrings(rc.fieldCtx.config.Field) {
				rc.fieldCtx.field.Value = value
				fields = append(fields, rc.fieldCtx.field)
			}
		}

		return fields
	}

	// custom fields have dedicated handlers

	return rc.fieldCtx.config.CustomConfig.handler(s, rc)
}

func (s *searchContext) initializeRecordContext(doc *solrDocument) (*recordContext, error) {
	var rc recordContext

	rc.doc = doc

	pool := rc.doc.getFirstString(s.pool.config.Global.ResourceTypes.Field)
	if pool == "" {
		pool = s.pool.config.Global.ResourceTypes.DefaultContext
	}

	rc.resourceTypeCtx = s.pool.maps.resourceTypeContexts[pool]
	if rc.resourceTypeCtx == nil {
		return nil, fmt.Errorf("unable to map pool [%s] to a resource type context", pool)
	}

	// availability setup

	anonValues := doc.getStrings(s.pool.config.Global.Availability.FieldConfig.FieldAnon)
	anonOnShelf := sliceContainsAnyValueFromSlice(anonValues, s.pool.config.Global.Availability.FieldConfig.ExposedValues.OnShelf, true)
	rc.anonOnline = sliceContainsAnyValueFromSlice(anonValues, s.pool.config.Global.Availability.FieldConfig.ExposedValues.Online, true)

	authValues := doc.getStrings(s.pool.config.Global.Availability.FieldConfig.FieldAuth)
	authOnShelf := sliceContainsAnyValueFromSlice(authValues, s.pool.config.Global.Availability.FieldConfig.ExposedValues.OnShelf, true)
	rc.authOnline = sliceContainsAnyValueFromSlice(authValues, s.pool.config.Global.Availability.FieldConfig.ExposedValues.Online, true)

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
	for _, field := range rc.resourceTypeCtx.AuthorFields.Preferred {
		preferredAuthorValues = append(preferredAuthorValues, doc.getStrings(field)...)
	}

	var fallbackAuthorValues []string
	for _, field := range rc.resourceTypeCtx.AuthorFields.Fallback {
		fallbackAuthorValues = append(fallbackAuthorValues, doc.getStrings(field)...)
	}

	rawAuthorValues := preferredAuthorValues
	if len(rawAuthorValues) == 0 {
		rawAuthorValues = fallbackAuthorValues
	}

	rc.relations = s.parseRelations(rawAuthorValues)

	rc.hasVernacularTitle = doc.getFirstString(s.pool.maps.definedFields[rc.resourceTypeCtx.FieldNames.TitleVernacular.Name].Field) != ""
	rc.hasVernacularAuthor = doc.getFirstString(s.pool.maps.definedFields[rc.resourceTypeCtx.FieldNames.AuthorVernacular.Name].Field) != ""

	if s.compareFieldsOr(rc.doc, s.pool.config.Global.Titleization.Exclusions) == true {
		rc.titleize = false
	} else {
		rc.titleize = true
	}

	return &rc, nil
}

func (s *searchContext) populateRecord(doc *solrDocument) v4api.Record {
	var record v4api.Record

	rc, err := s.initializeRecordContext(doc)
	if err != nil {
		s.err(err.Error())
		return record
	}

	// determine what fields we are extracting from the document

	var fieldCfgs *[]poolConfigField

	switch {
	case s.itemDetails == true:
		fieldCfgs = &rc.resourceTypeCtx.fields.detailed

	default:
		fieldCfgs = &rc.resourceTypeCtx.fields.basic
	}

	// field loop

	for _, fieldCfg := range *fieldCfgs {
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

		rc.fieldCtx = fieldContext{config: fieldCfg, field: f}

		fieldValues := s.getFieldValues(rc)

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
					newField.Value = strings.TrimSpace(piece)
					fieldValues = append(fieldValues, newField)
				}
			}
		}

		i := 0
		for _, fieldValue := range fieldValues {
			if fieldCfg.CustomConfig == nil && fieldValue.Value == "" {
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

					if rc.fieldCtx.config.XID != "" {
						rf.Label = s.client.localize(rc.fieldCtx.config.XID)
					}

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

	groupsSeen := make(map[string]bool)

	start := time.Now()

	for i := range solrDocuments.Docs {
		doc := &solrDocuments.Docs[i]

		group := s.getSolrGroupFieldValue(doc)

		// default to an empty record with an empty set of fields
		record := v4api.Record{Fields: []v4api.RecordField{}}

		if s.virgo.flags.firstRecordOnly == false || groupsSeen[group] == false {
			record = s.populateRecord(doc)
		}

		records = append(records, record)

		groupsSeen[group] = true
	}

	elapsed := int64(time.Since(start) / time.Millisecond)

	s.verbose("populateRecords(): processed %d records in %d ms (%0.2f records/sec)", len(solrDocuments.Docs), elapsed, 1000.0*(float32(len(solrDocuments.Docs))/float32(elapsed)))

	return records
}

func (s *searchContext) newFacetFromDefinition(facetDef *poolConfigFilter) v4api.Facet {
	xid := s.resourceTypeCtx.FilterOverrides[facetDef.XID].XID
	if xid == "" {
		xid = facetDef.XID
	}

	facet := v4api.Facet{
		ID:     facetDef.XID,
		Name:   s.client.localize(xid),
		Type:   facetDef.Type,
		Sort:   facetDef.BucketSort,
		Hidden: facetDef.Hidden,
	}

	return facet
}

func (s *searchContext) populateFacet(facetDef *poolConfigFilter, value solrResponseFacet) v4api.Facet {

	facet := s.newFacetFromDefinition(facetDef)

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

				mappedValue, err := s.getExternalSolrValue(facetDef.Solr.Field, b.Val)
				if err != nil {
					s.warn(err.Error())
					continue
				}

				selected := false
				if s.solr.req.meta.selectionMap[facetDef.XID][b.Val] != "" {
					selected = true
				}

				buckets = append(buckets, v4api.FacetBucket{Value: mappedValue, Count: b.Count, Selected: selected})
			}
		}

		// sort facet bucket values per configuration.
		// this overrides any initial sort order returned by solr.  for instance,
		// we can re-sort pool_f bucket values based on the mapped displayed value,
		// or sort the most populous entries alphabetically.

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

		if s.solr.req.meta.requestFacets[key] == nil {
			continue
		}

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
			var facetDef *poolConfigFilter
			if s.virgo.flags.facetCache == true {
				if s.virgo.flags.globalFacetCache == true {
					facetDef = s.pool.maps.preSearchFilters[key]
				} else {
					facetDef = s.pool.maps.supportedFilters[key]
				}
			} else {
				facetDef = s.resourceTypeCtx.filterMap[key]
			}

			// if this is not the facet cache requesting all facets, then
			// add this facet to the response as long as one of its dependent facets is selected
			dependentFilterXIDs := s.resourceTypeCtx.FilterOverrides[key].DependentFilterXIDs

			if s.virgo.flags.facetCache == false && len(dependentFilterXIDs) > 0 {
				numSelected := 0

				for _, facet := range dependentFilterXIDs {
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
