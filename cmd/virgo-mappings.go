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
	citation bool
}

func (r *poolRecord) addField(field v4api.RecordField) {
	switch {
	case r.citation == true:
		// citation parts mode; if the field has a citation part, output a minimal field
		if field.CitationPart != "" {
			citationField := v4api.RecordField{
				Name:         field.Name,
				Value:        field.Value,
				CitationPart: field.CitationPart,
			}

			r.record.Fields = append(r.record.Fields, citationField)
		}

	default:
		// normal mode; add field as-is
		r.record.Fields = append(r.record.Fields, field)
	}
}

func (s *searchContext) getCitationFormat(formats []string) string {
	// use configured citation format for pool, if defined
	if s.pool.config.Local.Identity.CitationFormat != "" {
		return s.pool.config.Local.Identity.CitationFormat
	}

	// point to last entry (which by configuration is the fallback value)
	best := len(s.pool.config.Global.CitationFormats) - 1

	// check each format to try to find better type match
	for _, format := range formats {
		for i := range s.pool.config.Global.CitationFormats {
			// no need to check worse possibilities
			if i >= best {
				continue
			}

			citationFormat := &s.pool.config.Global.CitationFormats[i]

			if citationFormat.re.MatchString(format) == true {
				best = i
				break
			}
		}
	}

	return s.pool.config.Global.CitationFormats[best].Format
}

func (s *searchContext) isOnlineOnly(doc *solrDocument, fields []poolConfigOnlineField) bool {
	for _, field := range fields {
		fieldValues := doc.getValuesByTag(field.Field)

		for _, values := range field.Contains {
			s.log("[SLICE] [%s] %v contains %v ?", field.Field, fieldValues, values)
			if sliceContainsAllValuesFromSlice(fieldValues, values, true) == true {
				s.log("[SLICE] %v is a subset of %v", values, fieldValues)
				return true
			}
		}

		for _, values := range field.Matches {
			s.log("[SLICE] [%s] %v equals %v ?", field.Field, fieldValues, values)
			if slicesAreEqual(fieldValues, values, true) == true {
				s.log("[SLICE] %v is equal to %v", values, fieldValues)
				return true
			}
		}
	}

	return false
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

func (s *searchContext) getLabelledURLs(f v4api.RecordField, doc *solrDocument, cfg *poolConfigFieldTypeCustom) []v4api.RecordField {
	var values []v4api.RecordField

	urlValues := doc.getValuesByTag(cfg.URLField)
	labelValues := doc.getValuesByTag(cfg.LabelField)
	providerValues := doc.getValuesByTag(cfg.ProviderField)

	useLabels := false
	if len(labelValues) == len(urlValues) {
		useLabels = true
	}

	for i, item := range urlValues {
		item = strings.TrimSpace(item)

		if isValidURL(item) == false {
			continue
		}

		// prepend proxy URL if configured, not already present, is from a provider not specifically excluded, and matches a proxifiable domain
		proxify := false

		if cfg.ProxyPrefix != "" && strings.HasPrefix(item, cfg.ProxyPrefix) == false && sliceContainsAnyValueFromSlice(providerValues, cfg.NoProxyProviders, true) == false {
			for _, domain := range cfg.ProxyDomains {
				if strings.Contains(item, fmt.Sprintf("%s/", domain)) == true {
					proxify = true
					break
				}
			}
		}

		if proxify == true {
			f.Value = cfg.ProxyPrefix + item
		} else {
			f.Value = item
		}

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
	isSirsi            bool
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
		var authorValues []string

		authorValues = append(authorValues, rc.relations.authors.name...)
		authorValues = append(authorValues, rc.relations.editors.nameRelation...)
		authorValues = append(authorValues, rc.relations.advisors.nameRelation...)

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
			if sliceContainsString(s.pool.config.Global.Availability.ExposedValues, availabilityValue, true) {
				f.Value = availabilityValue
				values = append(values, f)
			}
		}

		return values

	case "citation_access":
		if s.isOnlineOnly(doc, field.CustomInfo.CitationAccess.OnlineFields) == true {
			f.Value = "online_only"
			values = append(values, f)
		}

		return values

	case "citation_advisor":
		for _, advisorValue := range rc.relations.advisors.name {
			f.Value = advisorValue
			values = append(values, f)
		}

		return values

	case "citation_author":
		for _, authorValue := range rc.relations.authors.name {
			f.Value = authorValue
			values = append(values, f)
		}

		return values

	case "citation_editor":
		for _, editorValue := range rc.relations.editors.name {
			f.Value = editorValue
			values = append(values, f)
		}

		return values

	case "citation_format":
		f.Value = s.getCitationFormat(fieldValues)
		values = append(values, f)

		return values

	case "citation_subtitle":
		subtitle := firstElementOf(fieldValues)
		f.Value = titlecase.Title(subtitle)
		values = append(values, f)

		return values

	case "citation_title":
		title := firstElementOf(fieldValues)
		f.Value = titlecase.Title(title)
		values = append(values, f)

		return values

	case "composer_performer":
		var authorValues []string

		authorValues = append(authorValues, rc.relations.authors.name...)
		authorValues = append(authorValues, rc.relations.editors.nameRelation...)
		authorValues = append(authorValues, rc.relations.advisors.nameRelation...)

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
		coverImageURL := ""

		if len(fieldValues) > 0 {
			coverImageURL = firstElementOf(fieldValues)
		} else {
			coverImageURL = s.getCoverImageURL(field.CustomInfo.CoverImageURL, doc, rc.relations.authors.name)
		}

		if coverImageURL != "" {
			f.Value = coverImageURL
			values = append(values, f)
		}

		return values

	case "creator":
		var authorValues []string

		authorValues = append(authorValues, rc.relations.authors.name...)
		authorValues = append(authorValues, rc.relations.editors.nameRelation...)
		authorValues = append(authorValues, rc.relations.advisors.nameRelation...)

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

	case "language":
		languageValues := fieldValues

		if len(languageValues) == 0 {
			languageValues = doc.getValuesByTag(field.CustomInfo.Language.AlternateField)
		}

		for _, languageValue := range languageValues {
			f.Value = languageValue
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

				if sliceContainsString(s.pool.config.Global.Service.Pdf.ReadyValues, pdfStatus, true) == true {
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

	case "sirsi_url":
		if rc.isSirsi == true {
			idValue := firstElementOf(doc.getValuesByTag(field.CustomInfo.SirsiURL.IDField))
			idPrefix := field.CustomInfo.SirsiURL.IDPrefix

			if strings.HasPrefix(idValue, idPrefix) {
				sirsiID := idValue[len(idPrefix):]
				if url := s.getSirsiURL(sirsiID); url != "" {
					f.Value = url
					values = append(values, f)
				}
			}
		}

		return values

	case "summary_holdings":
		for _, fieldValue := range fieldValues {
			parts := strings.Split(fieldValue, "|")
			if len(parts) != 6 {
				s.log("unexpected summary holding entry: [%s]", fieldValue)
				continue
			}
			f.SummaryLibrary = parts[0]
			f.SummaryLocation = parts[1]
			f.SummaryText = parts[2]
			f.SummaryNote = parts[3]
			f.SummaryLabel = parts[4]
			f.SummaryCallNumber = parts[5]
			values = append(values, f)
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

		authorValues := rc.relations.authors.name

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

		authorValues := rc.relations.authors.name

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

		authorValues := rc.relations.authors.name

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
	r := poolRecord{citation: s.client.opts.citation}

	var rc recordContext

	// availability setup

	anonValues := doc.getValuesByTag(s.pool.config.Global.Availability.Anon.Field)
	anonOnShelf := sliceContainsAnyValueFromSlice(anonValues, s.pool.config.Global.Availability.Values.OnShelf, true)
	rc.anonOnline = sliceContainsAnyValueFromSlice(anonValues, s.pool.config.Global.Availability.Values.Online, true)

	authValues := doc.getValuesByTag(s.pool.config.Global.Availability.Auth.Field)
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

	featureValues := doc.getValuesByTag(s.pool.config.Global.RecordAttributes.DigitalContent.Field)
	rc.hasDigitalContent = sliceContainsAnyValueFromSlice(featureValues, s.pool.config.Global.RecordAttributes.DigitalContent.Contains, true)

	dataSourceValues := doc.getValuesByTag(s.pool.config.Global.RecordAttributes.WSLS.Field)
	rc.isSirsi = sliceContainsAnyValueFromSlice(dataSourceValues, s.pool.config.Global.RecordAttributes.Sirsi.Contains, true)
	rc.isWSLS = sliceContainsAnyValueFromSlice(dataSourceValues, s.pool.config.Global.RecordAttributes.WSLS.Contains, true)

	// build parsed author lists from configured fields

	var preferredAuthorValues []string
	for _, field := range s.pool.config.Local.Solr.AuthorFields.Preferred {
		preferredAuthorValues = append(preferredAuthorValues, doc.getValuesByTag(field)...)
	}

	var fallbackAuthorValues []string
	for _, field := range s.pool.config.Local.Solr.AuthorFields.Fallback {
		fallbackAuthorValues = append(fallbackAuthorValues, doc.getValuesByTag(field)...)
	}

	rawAuthorValues := preferredAuthorValues
	if len(rawAuthorValues) == 0 {
		rawAuthorValues = fallbackAuthorValues
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

		f := v4api.RecordField{
			Name:         field.Name,
			Type:         field.Properties.Type,
			Separator:    field.Properties.Separator,
			Visibility:   field.Properties.Visibility,
			Display:      field.Properties.Display,
			Provider:     field.Properties.Provider,
			CitationPart: field.Properties.CitationPart,
		}

		if s.itemDetails == true {
			f.Visibility = "detailed"
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
			if field.Custom == false && fieldValue.Value == "" {
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
			if len(facetDef.ExposedValues) == 0 || sliceContainsString(facetDef.ExposedValues, b.Val, false) {
				selected := false
				if s.solr.req.meta.selectionMap[facetDef.XID][b.Val] != "" {
					selected = true
				}

				buckets = append(buckets, v4api.FacetBucket{Value: b.Val, Count: b.Count, Selected: selected})
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
			facetDef := s.pool.maps.externalFacets[key]

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
