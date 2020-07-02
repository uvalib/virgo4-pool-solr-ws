package main

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/igorsobreira/titlecase"
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

type poolRecord struct {
	record   v4api.Record
	ris      bool
	risCodes []string
}

func (r *poolRecord) addField(field v4api.RecordField) {
	// non-RIS mode; add field as-is
	if r.ris == false {
		r.record.Fields = append(r.record.Fields, field)
		return
	}

	// RIS mode; for each RIS code, output a minimal field with that code and the field value
	for _, code := range r.risCodes {
		risField := v4api.RecordField{
			Name:    field.Name,
			Value:   field.Value,
			RISCode: code,
		}

		r.record.Fields = append(r.record.Fields, risField)
	}
}

func (s *searchContext) getRISType(formats []string) string {
	// use configured RIS type for pool, if defined
	if s.pool.config.Local.Identity.RISType != "" {
		return s.pool.config.Local.Identity.RISType
	}

	// point to last entry (which by configuration should be GEN for generic)
	best := len(s.pool.config.Global.RISTypes) - 1

	// check each format to try to find better type match
	for _, format := range formats {
		for i := range s.pool.config.Global.RISTypes {
			// no need to check worse possibilities
			if i >= best {
				continue
			}

			ristype := &s.pool.config.Global.RISTypes[i]

			if ristype.re.MatchString(format) == true {
				best = i
				break
			}
		}
	}

	return s.pool.config.Global.RISTypes[best].Type
}

func (s *searchContext) getPublisherEntry(doc *solrDocument) *poolConfigPublisher {
	for i := range s.pool.config.Global.Publishers {
		publisher := &s.pool.config.Global.Publishers[i]

		fieldValues := doc.getValuesByTag(publisher.Field)

		for _, fieldValue := range fieldValues {
			if publisher.re.MatchString(fieldValue) == true {
				return publisher
			}
		}
	}

	return nil
}

func (s *searchContext) getPublishedLocation(doc *solrDocument) []string {
	if publisher := s.getPublisherEntry(doc); publisher != nil {
		return []string{publisher.Place}
	}

	return []string{}
}

func (s *searchContext) getPublisherName(doc *solrDocument) []string {
	if publisher := s.getPublisherEntry(doc); publisher != nil {
		return []string{publisher.Publisher}
	}

	return []string{}
}

func (s *searchContext) getCopyrightLabelIcon(text string, cfg poolConfigCopyrightLabels) (string, string) {
	var icon string

	var texts []string
	if cfg.Split == "" {
		texts = []string{text}
	} else {
		texts = strings.Split(text, cfg.Split)
	}

	var labels []string
	for _, txt := range texts {
		for _, val := range cfg.Labels {
			if strings.EqualFold(txt, val.Text) == true {
				icon = val.Icon
				labels = append(labels, val.Label)
				continue
			}
		}
	}

	prefix := ""
	if cfg.Prefix != "" {
		prefix = cfg.Prefix + " "
	}

	suffix := ""
	if cfg.Suffix != "" {
		suffix = " " + cfg.Suffix
	}

	label := fmt.Sprintf("%s%s%s", prefix, strings.Join(labels, cfg.Join), suffix)

	// use default icon if none specified by label map.  this accounts for literal values or
	// templatized ones ("{code}" is the code portion of the copyright url being matched)
	if icon == "" {
		icon = strings.ReplaceAll(cfg.DefaultIcon, "{code}", text)
	}

	return label, icon
}

func (s *searchContext) getCopyrightLabelURLIcon(doc *solrDocument) (string, string, string) {
	for _, cr := range s.pool.config.Global.Copyrights {
		fieldValues := doc.getValuesByTag(cr.Field)

		for _, fieldValue := range fieldValues {
			if groups := cr.re.FindStringSubmatch(fieldValue); len(groups) > 0 {
				// first, check explicit assignment
				if cr.Label != "" {
					return cr.Label, cr.URL, cr.Icon
				}

				url := groups[cr.URLGroup]

				// next, check path mappings (if specified)
				if cr.PathGroup > 0 {
					path := groups[cr.PathGroup]
					if label, icon := s.getCopyrightLabelIcon(path, cr.PathLabels); label != "" {
						icon = fmt.Sprintf("%s/%s", cr.IconPath, icon)
						return label, url, icon
					}
				}

				// finally, attempt to map code to a label
				code := groups[cr.CodeGroup]
				if label, icon := s.getCopyrightLabelIcon(code, cr.CodeLabels); label != "" {
					icon = fmt.Sprintf("%s/%s", cr.IconPath, icon)
					return label, url, icon
				}
			}
		}
	}

	// no matches found
	return "", "", ""
}

func (s *searchContext) getLabelledURLs(f v4api.RecordField, doc *solrDocument, cfg *poolConfigFieldTypeAccessURL) []v4api.RecordField {
	var values []v4api.RecordField

	urlValues := doc.getValuesByTag(cfg.URLField)
	labelValues := doc.getValuesByTag(cfg.LabelField)

	useLabels := false
	if len(labelValues) == len(urlValues) {
		useLabels = true
	}

	for i, item := range urlValues {
		item = strings.TrimSpace(item)

		if isValidURL(item) == false {
			continue
		}

		f.Value = item

		itemLabel := ""
		if useLabels == true {
			itemLabel = strings.TrimSpace(labelValues[i])
			//itemLabel = titleizeIfUppercase(itemLabel)
		}

		// if not using labels, or this label is not defined, fall back to generic item label
		if itemLabel == "" {
			itemLabel = fmt.Sprintf("%s %d", s.client.localize(cfg.DefaultItemXID), i+1)
		}

		f.Item = strings.TrimSpace(itemLabel)

		values = append(values, f)
	}

	return values
}

type recordContext struct {
	anonOnline         bool
	authOnline         bool
	availabilityValues []string
	isAvailableOnShelf bool
	anonRequest        bool
	hasDigitalContent  bool
	isWSLS             bool
	relations          categorizedRelations
}

func (s *searchContext) getFieldValues(rc recordContext, field poolConfigField, f v4api.RecordField, doc *solrDocument) []v4api.RecordField {
	var values []v4api.RecordField

	fieldValues := doc.getValuesByTag(field.Field)

	if field.Custom == false {
		for _, fieldValue := range fieldValues {
			f.Value = fieldValue
			values = append(values, f)
		}

		return values
	}

	switch field.Name {
	case "abstract":
		abstractValues := fieldValues

		if len(abstractValues) == 0 {
			abstractValues = doc.getValuesByTag(field.CustomInfo.Abstract.AlternateField)
		}

		for _, abstractValue := range abstractValues {
			f.Value = abstractValue
			values = append(values, f)
		}

		return values

	case "access_url":
		if rc.anonOnline == false && rc.authOnline == false {
			return values
		}

		providerValues := doc.getValuesByTag(field.CustomInfo.AccessURL.ProviderField)

		f.Provider = firstElementOf(providerValues)

		values = s.getLabelledURLs(f, doc, field.CustomInfo.AccessURL)

		return values

	case "authenticate":
		if rc.anonRequest == true && rc.anonOnline == false && rc.authOnline == true {
			values = append(values, f)
		}

		return values

	case "author":
		authorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.PreferredHeaderField)

		if len(authorValues) == 0 {
			authorValues = rc.relations.authors.name
		}

		for _, authorValue := range authorValues {
			f.Value = authorValue
			values = append(values, f)
		}

		return values

	case "author_list":
		for _, authorValue := range rc.relations.authors.name {
			f.Value = authorValue
			values = append(values, f)
		}

		return values

	case "availability":
		for _, availabilityValue := range rc.availabilityValues {
			if sliceContainsString(s.pool.config.Global.Availability.ExposedValues, availabilityValue) {
				f.Value = availabilityValue
				values = append(values, f)
			}
		}

		return values

	case "composer_performer":
		authorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.PreferredHeaderField)

		if len(authorValues) == 0 {
			authorValues = rc.relations.authors.name
		}

		for _, authorValue := range authorValues {
			f.Value = authorValue
			values = append(values, f)
		}

		return values

	case "copyright_and_permissions":
		if label, url, icon := s.getCopyrightLabelURLIcon(doc); label != "" {
			f.Value = url
			f.Item = label
			f.Icon = icon
			values = append(values, f)
		}

		return values

	case "cover_image_url":
		if s.pool.maps.attributes["cover_images"].Supported == true {
			if url := s.getCoverImageURL(field.CustomInfo.CoverImageURL, doc, rc.relations.authors.name); url != "" {
				f.Value = url
				values = append(values, f)
			}
		}

		return values

	case "creator":
		authorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.PreferredHeaderField)

		if len(authorValues) == 0 {
			authorValues = rc.relations.authors.name
		}

		for _, authorValue := range authorValues {
			f.Value = authorValue
			values = append(values, f)
		}

		return values

	case "digital_content_url":
		if url := s.getDigitalContentURL(doc, field.CustomInfo.DigitalContentURL.IDField); url != "" {
			f.Value = url
			values = append(values, f)
		}

		return values

	case "online_related":
		values = s.getLabelledURLs(f, doc, field.CustomInfo.AccessURL)

		return values

	case "pdf_download_url":
		pidValues := doc.getValuesByTag(field.CustomInfo.PdfDownloadURL.PIDField)

		if len(pidValues) <= field.CustomInfo.PdfDownloadURL.MaxSupported {
			pdfURL := firstElementOf(doc.getValuesByTag(field.CustomInfo.PdfDownloadURL.URLField))

			if pdfURL == "" {
				return values
			}

			for _, pid := range pidValues {
				if pid == "" {
					return values
				}

				statusURL := fmt.Sprintf("%s/%s%s", pdfURL, pid, s.pool.config.Global.Service.Pdf.Endpoints.Status)

				pdfStatus, pdfErr := s.getPdfStatus(statusURL)

				if pdfErr != nil {
					return values
				}

				if sliceContainsString(s.pool.config.Global.Service.Pdf.ReadyValues, pdfStatus) == true {
					downloadURL := fmt.Sprintf("%s/%s%s", pdfURL, pid, s.pool.config.Global.Service.Pdf.Endpoints.Download)
					f.Value = downloadURL
					values = append(values, f)
				}
			}
		}

		return values

	case "published_location":
		pubValues := fieldValues

		if len(pubValues) == 0 {
			pubValues = s.getPublishedLocation(doc)
		}

		for _, pubValue := range pubValues {
			f.Value = pubValue
			values = append(values, f)
		}

		return values

	case "publisher_name":
		pubValues := fieldValues

		if len(pubValues) == 0 {
			pubValues = doc.getValuesByTag(field.CustomInfo.PublisherName.AlternateField)
		}

		if len(pubValues) == 0 {
			pubValues = s.getPublisherName(doc)
		}

		for _, pubValue := range pubValues {
			f.Value = pubValue
			values = append(values, f)
		}

		return values

	case "related_resources":
		values = s.getLabelledURLs(f, doc, field.CustomInfo.AccessURL)

		return values

	case "ris_additional_author":
		for i, authorValue := range rc.relations.authors.name {
			if i == 0 {
				continue
			}

			f.Value = authorValue
			values = append(values, f)
		}

		return values

	case "ris_author":
		if len(rc.relations.authors.name) > 0 {
			f.Value = rc.relations.authors.name[0]
			values = append(values, f)
		}

		return values

	case "ris_type":
		formatValues := doc.getValuesByTag(field.CustomInfo.RISType.FormatField)
		f.Value = s.getRISType(formatValues)
		values = append(values, f)

		return values

	case "sirsi_url":
		idValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.SirsiURL.IDField))
		idPrefix := field.CustomInfo.SirsiURL.IDPrefix

		if strings.HasPrefix(idValue, idPrefix) {
			sirsiID := idValue[len(idPrefix):]
			if url := s.getSirsiURL(sirsiID); url != "" {
				f.Value = url
				values = append(values, f)
			}
		}

		return values

	case "thumbnail_url":
		urlValues := doc.getValuesByTag(field.CustomInfo.ThumbnailURL.URLField)

		if len(urlValues) <= field.CustomInfo.ThumbnailURL.MaxSupported {
			for _, url := range urlValues {
				if url != "" {
					f.Value = url
					values = append(values, f)
				}
			}
		}

		return values

	case "title_subtitle_edition":
		titleValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.TitleSubtitleEdition.TitleField))
		subtitleValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.TitleSubtitleEdition.SubtitleField))
		editionValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.TitleSubtitleEdition.EditionField))

		fullTitle := titlecase.Title(titleValue)

		if subtitleValue != "" {
			fullTitle = fmt.Sprintf("%s: %s", fullTitle, titlecase.Title(subtitleValue))
		}

		if editionValue != "" {
			if strings.HasPrefix(editionValue, "(") && strings.HasSuffix(editionValue, ")") {
				fullTitle = fmt.Sprintf("%s %s", fullTitle, editionValue)
			} else {
				fullTitle = fmt.Sprintf("%s (%s)", fullTitle, editionValue)
			}
		}

		f.Value = fullTitle
		values = append(values, f)

		return values

	case "vernacularized_author":
		vernacularValue := firstElementOf(fieldValues)
		authorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.PreferredHeaderField)

		if len(authorValues) == 0 {
			authorValues = rc.relations.authors.name
		}

		for _, authorValue := range authorValues {
			f.Value = authorValue
			if vernacularValue != "" {
				f.Value += "<p>" + vernacularValue
			}
			values = append(values, f)
		}

		return values

	case "vernacularized_composer_performer":
		vernacularValue := firstElementOf(fieldValues)
		authorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.PreferredHeaderField)

		if len(authorValues) == 0 {
			authorValues = rc.relations.authors.name
		}

		for _, authorValue := range authorValues {
			f.Value = authorValue
			if vernacularValue != "" {
				f.Value += "<p>" + vernacularValue
			}
			values = append(values, f)
		}

		return values

	case "vernacularized_creator":
		vernacularValue := firstElementOf(fieldValues)
		authorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.PreferredHeaderField)

		if len(authorValues) == 0 {
			authorValues = rc.relations.authors.name
		}

		for _, authorValue := range authorValues {
			f.Value = authorValue
			if vernacularValue != "" {
				f.Value += "<p>" + vernacularValue
			}
			values = append(values, f)
		}

		return values

	case "vernacularized_title":
		vernacularValue := firstElementOf(fieldValues)
		titleValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.TitleSubtitleEdition.TitleField))

		fullTitle := titlecase.Title(titleValue)

		if vernacularValue != "" {
			fullTitle += "<p>" + vernacularValue
		}

		f.Value = fullTitle
		values = append(values, f)

		return values

	case "vernacularized_title_subtitle_edition":
		vernacularValue := firstElementOf(fieldValues)
		titleValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.TitleSubtitleEdition.TitleField))
		subtitleValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.TitleSubtitleEdition.SubtitleField))
		editionValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.TitleSubtitleEdition.EditionField))

		fullTitle := titlecase.Title(titleValue)

		if subtitleValue != "" {
			fullTitle = fmt.Sprintf("%s: %s", fullTitle, titlecase.Title(subtitleValue))
		}

		if editionValue != "" {
			if strings.HasPrefix(editionValue, "(") && strings.HasSuffix(editionValue, ")") {
				fullTitle = fmt.Sprintf("%s %s", fullTitle, editionValue)
			} else {
				fullTitle = fmt.Sprintf("%s (%s)", fullTitle, editionValue)
			}
		}

		if vernacularValue != "" {
			fullTitle += "<p>" + vernacularValue
		}

		f.Value = fullTitle
		values = append(values, f)

		return values

	case "wsls_collection_description":
		if rc.isWSLS == true {
			f.Value = s.client.localize(field.CustomInfo.WSLSCollectionDescription.ValueXID)
			values = append(values, f)
		}

		return values
	}

	return values
}

func (s *searchContext) populateRecord(doc *solrDocument) v4api.Record {
	r := poolRecord{ris: s.client.opts.ris}

	var rc recordContext

	// availability setup

	anonValues := doc.getValuesByTag(s.pool.config.Global.Availability.Anon.Field)
	anonOnShelf := sliceContainsValueFromSlice(anonValues, s.pool.config.Global.Availability.Values.OnShelf)
	rc.anonOnline = sliceContainsValueFromSlice(anonValues, s.pool.config.Global.Availability.Values.Online)

	authValues := doc.getValuesByTag(s.pool.config.Global.Availability.Auth.Field)
	authOnShelf := sliceContainsValueFromSlice(authValues, s.pool.config.Global.Availability.Values.OnShelf)
	rc.authOnline = sliceContainsValueFromSlice(authValues, s.pool.config.Global.Availability.Values.Online)

	// determine which availability field to use

	rc.availabilityValues = anonValues
	rc.isAvailableOnShelf = anonOnShelf
	rc.anonRequest = true

	if s.client.isAuthenticated() == true {
		rc.availabilityValues = authValues
		rc.isAvailableOnShelf = authOnShelf
		rc.anonRequest = false
	}

	featureValues := doc.getValuesByTag(s.pool.config.Global.RecordAttributes.DigitalContent.Field)
	rc.hasDigitalContent = sliceContainsValueFromSlice(featureValues, s.pool.config.Global.RecordAttributes.DigitalContent.Contains)

	dataSourceValues := doc.getValuesByTag(s.pool.config.Global.RecordAttributes.WSLS.Field)
	rc.isWSLS = sliceContainsValueFromSlice(dataSourceValues, s.pool.config.Global.RecordAttributes.WSLS.Contains)

	// build parsed author lists from configured fields

	initialAuthorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.InitialAuthorField)
	preferredAuthorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.PreferredAuthorField)
	fallbackAuthorValues := doc.getValuesByTag(s.pool.config.Local.Solr.AuthorFields.FallbackAuthorField)

	rawAuthorValues := initialAuthorValues
	if len(preferredAuthorValues) > 0 {
		rawAuthorValues = append(rawAuthorValues, preferredAuthorValues...)
	} else {
		rawAuthorValues = append(rawAuthorValues, fallbackAuthorValues...)
	}

	rc.relations = s.parseRelations(rawAuthorValues)

	// field loop

	fields := s.pool.fields.basic
	if s.itemDetails == true {
		fields = s.pool.fields.detailed
	}

	for _, field := range fields {
		if field.OnShelfOnly == true && rc.isAvailableOnShelf == false {
			continue
		}

		if field.DigitalContentOnly == true && rc.hasDigitalContent == false {
			continue
		}

		r.risCodes = field.RISCodes

		f := v4api.RecordField{
			Name:       field.Name,
			Type:       field.Properties.Type,
			Separator:  field.Properties.Separator,
			Visibility: field.Properties.Visibility,
			Display:    field.Properties.Display,
			Provider:   field.Properties.Provider,
		}

		if field.XID != "" {
			if field.WSLSXID != "" && rc.isWSLS == true {
				f.Label = s.client.localize(field.WSLSXID)
			} else {
				f.Label = s.client.localize(field.XID)
			}
		}

		fieldValues := s.getFieldValues(rc, field, f, doc)

		if len(fieldValues) == 0 {
			continue
		}

		// split single field if configured
		if len(fieldValues) == 1 && field.SplitOn != "" {
			origField := fieldValues[0]
			splitValues := strings.Split(origField.Value, field.SplitOn)
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
			if fieldValue.Value == "" {
				continue
			}

			r.addField(fieldValue)

			if field.Limit > 0 && i+1 >= field.Limit {
				break
			}

			i++
		}
	}

	// add internal info

	r.record.GroupValue = s.getSolrGroupFieldValue(doc)

	if s.client.opts.debug == true {
		r.record.Debug = make(map[string]interface{})
		r.record.Debug["score"] = doc.Score
	}

	return r.record
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

				buckets = append(buckets, v4api.FacetBucket{Value: b.Val, Count: b.GroupCount, Selected: selected})
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

func (s *searchContext) populateFacetList(solrFacets solrResponseFacets) []v4api.Facet {
	type indexedFacet struct {
		index int
		facet v4api.Facet
	}

	var orderedFacets []indexedFacet

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
	var r v4api.Record

	switch s.solr.res.meta.numRecords {
	case 0:
		return searchResponse{status: http.StatusNotFound, err: fmt.Errorf("record not found")}

	case 1:
		r = s.populateRecord(s.solr.res.meta.firstDoc)

	default:
		return searchResponse{status: http.StatusInternalServerError, err: fmt.Errorf("multiple records found")}
	}

	s.virgo.recordRes = &r

	return searchResponse{status: http.StatusOK}
}
