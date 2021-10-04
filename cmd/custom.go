package main

import (
	"fmt"
	"strings"

	"github.com/uvalib/virgo4-api/v4api"
)

type customHandler func(*searchContext, *recordContext) []v4api.RecordField

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

func (s *searchContext) compareFields(doc *solrDocument, fields []poolConfigFieldComparison) bool {
	for _, field := range fields {
		fieldValues := doc.getStrings(field.Field)

		for _, values := range field.Contains {
			if sliceContainsAllValuesFromSlice(fieldValues, values, true) == true {
				return true
			}
		}

		for _, values := range field.Matches {
			if slicesAreEqual(fieldValues, values, true) == true {
				return true
			}
		}
	}

	return false
}

func (s *searchContext) getPublisherEntry(doc *solrDocument) *poolConfigPublisher {
	for i := range s.pool.config.Global.Publishers {
		publisher := &s.pool.config.Global.Publishers[i]

		fieldValues := doc.getStrings(publisher.Field)

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
		fieldValues := doc.getStrings(cr.Field)

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

func (s *searchContext) getLabelledURLs(f v4api.RecordField, doc *solrDocument, cfg *poolConfigFieldCustomConfig) []v4api.RecordField {
	var values []v4api.RecordField

	urlValues := doc.getStrings(cfg.URLField)
	labelValues := doc.getStrings(cfg.LabelField)
	providerValues := doc.getStrings(cfg.ProviderField)

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

func (s *searchContext) getSummaryHoldings(fieldValues []string) interface{} {
	type summaryTextNote struct {
		Text string `json:"text"`
		Note string `json:"note,omitempty"`
	}

	type summaryCallNumber struct {
		CallNumber string            `json:"call_number"`
		TextNotes  []summaryTextNote `json:"text_notes,omitempty"`
	}

	type summaryLocation struct {
		Location    string              `json:"location"`
		CallNumbers []summaryCallNumber `json:"call_numbers,omitempty"`
	}

	type summaryLibrary struct {
		Library   string            `json:"library"`
		Locations []summaryLocation `json:"locations,omitempty"`
	}

	type summaryHoldings struct {
		Libraries []summaryLibrary `json:"libraries,omitempty"`
	}

	type summaryResp struct {
		Holdings summaryHoldings `json:"holdings,omitempty"`
	}

	var resp summaryResp

	// maps, from which the final structure will be made

	libraries := make(map[string]map[string]map[string][]summaryTextNote)

	lastCallNumber := ""

	for _, fieldValue := range fieldValues {
		parts := strings.Split(fieldValue, "|")
		if len(parts) != 6 {
			s.log("unexpected summary holding entry: [%s]", fieldValue)
			continue
		}

		library := parts[0]
		location := parts[1]
		text := parts[2]
		note := parts[3]
		//label := parts[4]
		callNumber := parts[5]

		if library != "" && libraries[library] == nil {
			libraries[library] = make(map[string]map[string][]summaryTextNote)
		}

		if library != "" && location != "" && libraries[library][location] == nil {
			libraries[library][location] = make(map[string][]summaryTextNote)
		}

		if callNumber != "" && callNumber != lastCallNumber {
			lastCallNumber = callNumber
		}

		if text != "" && library != "" && location != "" && lastCallNumber != "" {
			textNote := summaryTextNote{Text: text, Note: note}
			libraries[library][location][lastCallNumber] = append(libraries[library][location][lastCallNumber], textNote)
		}
	}

	// if no data, return nil so the field can easily be omitted from the response
	if len(libraries) == 0 {
		return nil
	}

	// build holdings structure from collected maps

	for klib, vlib := range libraries {
		nlib := summaryLibrary{Library: klib}

		for kloc, vloc := range vlib {
			nloc := summaryLocation{Location: kloc}

			for knum, vnum := range vloc {
				nnum := summaryCallNumber{CallNumber: knum, TextNotes: vnum}
				nloc.CallNumbers = append(nloc.CallNumbers, nnum)
			}

			nlib.Locations = append(nlib.Locations, nloc)
		}

		resp.Holdings.Libraries = append(resp.Holdings.Libraries, nlib)
	}

	return resp
}

func (s *searchContext) buildTitle(rc *recordContext, title, subtitle, edition, vernacular string) string {
	theTitle := title
	theSubtitle := subtitle
	if rc.titleize == true {
		theTitle = s.pool.titleizer.titleize(title)
		theSubtitle = s.pool.titleizer.titleize(subtitle)
	}

	fullTitle := theTitle
	if subtitle != "" {
		fullTitle = fmt.Sprintf("%s: %s", fullTitle, theSubtitle)
	}

	if edition != "" {
		if strings.HasPrefix(edition, "(") && strings.HasSuffix(edition, ")") {
			fullTitle = fmt.Sprintf("%s %s", fullTitle, edition)
		} else {
			fullTitle = fmt.Sprintf("%s (%s)", fullTitle, edition)
		}
	}

	if vernacular != "" {
		fullTitle += "<p>" + vernacular
	}

	return fullTitle
}

/********************  individual custom field implementations  ********************/

func getCustomFieldAbstract(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if len(values) == 0 {
		values = rc.doc.getStrings(rc.fieldCtx.config.CustomConfig.AlternateField)
	}

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldAccessURLSerialsSolutions(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if s.itemDetails == false {
		return fv
	}

	if s.pool.config.Global.Service.SerialsSolutions.Enabled == false {
		s.log("skipping serials solutions API query per configuration")
		return fv
	}

	issns := rc.doc.getStrings(rc.fieldCtx.config.CustomConfig.ISSNField)
	isbns := rc.doc.getStrings(rc.fieldCtx.config.CustomConfig.ISBNField)

	genre := ""
	serialType := ""
	var serials []string

	switch {
	case len(issns) > 0:
		genre = "journal"
		serialType = "issn"
		serials = issns

	case len(isbns) > 0:
		genre = "book"
		serialType = "isbn"
		serials = isbns

	default:
		return fv
	}

	res, err := s.serialsSolutionsLookup(genre, serialType, serials)
	if err != nil {
		s.warn("serials solutions lookup failed: [%s]", err.Error())
		return fv
	}

	for _, r := range res.Results {
		for _, l := range r.LinkGroups {
			if l.Type != "holding" { //|| l.HoldingData.ProviderID != "PRVEBS" {
				continue
			}

			for _, u := range l.URLs {
				if u.Type != "journal" {
					continue
				}

				startDate := l.HoldingData.StartDate
				if startDate == "" {
					startDate = "unknown"
				}

				endDate := l.HoldingData.EndDate
				if endDate == "" {
					endDate = "present"
				}

				rc.fieldCtx.field.Provider = "exlibris"
				rc.fieldCtx.field.Value = u.URL
				rc.fieldCtx.field.Item = fmt.Sprintf("%s (%s to %s)", l.HoldingData.DatabaseName, startDate, endDate)

				fv = append(fv, rc.fieldCtx.field)
			}
		}
	}

	return fv
}

func getCustomFieldAccessURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.anonOnline == false && rc.authOnline == false {
		// this item is not available online per solr... but maybe we hold electronic subscriptions?
		return getCustomFieldAccessURLSerialsSolutions(s, rc)
	}

	rc.fieldCtx.field.Provider = rc.doc.getFirstString(rc.fieldCtx.config.CustomConfig.ProviderField)

	fv = s.getLabelledURLs(rc.fieldCtx.field, rc.doc, rc.fieldCtx.config.CustomConfig)

	return fv
}

func getCustomFieldAuthenticate(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.anonRequest == true && rc.anonOnline == false && rc.authOnline == true {
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldAuthor(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	var values []string

	if rc.hasVernacularAuthor == true {
		rc.fieldCtx.field.Type = rc.fieldCtx.config.CustomConfig.AlternateType
		rc.fieldCtx.config.XID = rc.fieldCtx.config.CustomConfig.AlternateXID
	}

	values = append(values, rc.relations.authors.name...)
	values = append(values, rc.relations.editors.nameRelation...)
	values = append(values, rc.relations.advisors.nameRelation...)

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldAuthorVernacular(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.hasVernacularAuthor == true {
		rc.fieldCtx.field.Type = rc.fieldCtx.config.CustomConfig.AlternateType
		rc.fieldCtx.config.XID = rc.fieldCtx.config.CustomConfig.AlternateXID
	}

	for _, value := range rc.doc.getStrings(rc.fieldCtx.config.Field) {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldAvailability(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.availabilityValues {
		if sliceContainsString(s.pool.config.Global.Availability.FieldConfig.ExposedValues.Combined, value, true) {
			rc.fieldCtx.field.Value = value
			fv = append(fv, rc.fieldCtx.field)
		}
	}

	return fv
}

func getCustomFieldCitationAdvisor(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.advisors.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCitationAuthor(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.authors.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCitationCompiler(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.compilers.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCitationEditor(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.editors.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCitationFormat(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	rc.fieldCtx.field.Value = s.getCitationFormat(values)

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func getCustomFieldCitationIsOnlineOnly(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if s.compareFields(rc.doc, rc.fieldCtx.config.CustomConfig.ComparisonFields) == true {
		rc.fieldCtx.field.Value = "true"
	} else {
		rc.fieldCtx.field.Value = "false"
	}

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func getCustomFieldCitationIsVirgoURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if s.compareFields(rc.doc, rc.fieldCtx.config.CustomConfig.ComparisonFields) == true {
		rc.fieldCtx.field.Value = "true"
	} else {
		rc.fieldCtx.field.Value = "false"
	}

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func getCustomFieldCitationSubtitle(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	subtitle := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	if rc.titleize == true {
		subtitle = s.pool.titleizer.titleize(subtitle)
	}

	rc.fieldCtx.field.Value = subtitle

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func getCustomFieldCitationTitle(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	title := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	if rc.titleize == true {
		title = s.pool.titleizer.titleize(title)
	}

	rc.fieldCtx.field.Value = title

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func getCustomFieldCitationTranslator(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.translators.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCollectionContext(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	return fv
}

func getCustomFieldComposerPerformer(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	var values []string

	values = append(values, rc.relations.authors.name...)
	values = append(values, rc.relations.editors.nameRelation...)
	values = append(values, rc.relations.advisors.nameRelation...)

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCopyrightAndPermissions(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if label, url, icon := s.getCopyrightLabelURLIcon(rc.doc); label != "" {
		rc.fieldCtx.field.Value = url
		rc.fieldCtx.field.Item = label
		rc.fieldCtx.field.Icon = icon
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCoverImageURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	coverImageURL := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	if coverImageURL == "" {
		coverImageURL = s.getCoverImageURL(rc.fieldCtx.config.CustomConfig, rc.doc, rc.relations.authors.name)
	}

	if coverImageURL != "" {
		rc.fieldCtx.field.Value = coverImageURL
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCreator(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	var values []string

	values = append(values, rc.relations.authors.name...)
	values = append(values, rc.relations.editors.nameRelation...)
	values = append(values, rc.relations.advisors.nameRelation...)

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldDigitalContentURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if url := s.getDigitalContentURL(rc.doc, rc.fieldCtx.config.CustomConfig.IDField); url != "" {
		rc.fieldCtx.field.Value = url
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldLanguage(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if len(values) == 0 {
		values = rc.doc.getStrings(rc.fieldCtx.config.CustomConfig.AlternateField)
	}

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldOnlineRelated(s *searchContext, rc *recordContext) []v4api.RecordField {
	return s.getLabelledURLs(rc.fieldCtx.field, rc.doc, rc.fieldCtx.config.CustomConfig)
}

func getCustomFieldPublishedDate(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isWSLS == true {
		rc.fieldCtx.config.XID = rc.fieldCtx.config.CustomConfig.AlternateXID
	}

	for _, value := range rc.doc.getStrings(rc.fieldCtx.config.Field) {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldPublishedLocation(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if len(values) == 0 {
		values = s.getPublishedLocation(rc.doc)
	}

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldPublisherName(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if len(values) == 0 {
		values = rc.doc.getStrings(rc.fieldCtx.config.CustomConfig.AlternateField)
	}

	if len(values) == 0 {
		values = s.getPublisherName(rc.doc)
	}

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldRelatedResources(s *searchContext, rc *recordContext) []v4api.RecordField {
	return s.getLabelledURLs(rc.fieldCtx.field, rc.doc, rc.fieldCtx.config.CustomConfig)
}

func getCustomFieldResponsibilityStatement(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.hasVernacularAuthor == true {
		return fv
	}

	for _, value := range rc.doc.getStrings(rc.fieldCtx.config.Field) {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldSirsiURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isSirsi == true {
		idValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomConfig.IDField)
		idPrefix := rc.fieldCtx.config.CustomConfig.IDPrefix

		if strings.HasPrefix(idValue, idPrefix) {
			sirsiID := idValue[len(idPrefix):]
			if url := s.getSirsiURL(sirsiID); url != "" {
				rc.fieldCtx.field.Value = url
				fv = append(fv, rc.fieldCtx.field)
			}
		}
	}

	return fv
}

func getCustomFieldSubjectSummary(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isWSLS == true {
		rc.fieldCtx.config.XID = rc.fieldCtx.config.CustomConfig.AlternateXID
	}

	for _, value := range rc.doc.getStrings(rc.fieldCtx.config.Field) {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldSummaryHoldings(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if summaryHoldings := s.getSummaryHoldings(values); summaryHoldings != nil {
		rc.fieldCtx.field.StructuredValue = summaryHoldings
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldTermsOfUse(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isWSLS == true {
		rc.fieldCtx.config.XID = rc.fieldCtx.config.CustomConfig.AlternateXID
	}

	for _, value := range rc.doc.getStrings(rc.fieldCtx.config.Field) {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldTitleSubtitleEdition(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.hasVernacularTitle == true {
		rc.fieldCtx.field.Type = rc.fieldCtx.config.CustomConfig.AlternateType
		rc.fieldCtx.config.XID = rc.fieldCtx.config.CustomConfig.AlternateXID
	}

	titleValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomConfig.TitleField)
	subtitleValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomConfig.SubtitleField)
	editionValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomConfig.EditionField)
	vernacularValue := ""

	rc.fieldCtx.field.Value = s.buildTitle(rc, titleValue, subtitleValue, editionValue, vernacularValue)

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func getCustomFieldTitleVernacular(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.hasVernacularTitle == true {
		rc.fieldCtx.field.Type = rc.fieldCtx.config.CustomConfig.AlternateType
		rc.fieldCtx.config.XID = rc.fieldCtx.config.CustomConfig.AlternateXID
	}

	for _, value := range rc.doc.getStrings(rc.fieldCtx.config.Field) {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldWSLSCollectionDescription(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isWSLS == true {
		rc.fieldCtx.field.Value = s.client.localize(rc.fieldCtx.config.CustomConfig.ValueXID)
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}
