package main

import (
	"fmt"
	"strings"

	"github.com/uvalib/virgo4-api/v4api"
)

type customFieldHandler func(*searchContext, *recordContext) []v4api.RecordField

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

func (s *searchContext) compareField(doc *solrDocument, field poolConfigFieldComparison) bool {
	// set return value based on negate flag (default is false):
	// op == true --> contains/matches checks return false; otherwise true
	// op == false --> contains/matches checks return true; otherwise false
	op := field.Negate

	fieldValues := doc.getStrings(field.Field)

	for _, values := range field.Contains {
		if sliceContainsAllValuesFromSlice(fieldValues, values, true) == true {
			return !op
		}
	}

	for _, values := range field.Matches {
		if slicesAreEqual(fieldValues, values, true) == true {
			return !op
		}
	}

	return op
}

func (s *searchContext) evaluateConditions(doc *solrDocument, conditions poolConfigFieldConditions) bool {
	// determine how to compare fields based on operator (default is AND unless OR is specified):
	// op == true --> this function acts as an OR operation when comparing fields
	// op == false --> this function acts as an AND operation when comparing fields
	op := strings.EqualFold(conditions.Operator, "or")

	for _, field := range conditions.Comparisons {
		if s.compareField(doc, field) == op {
			return op
		}
	}

	return !op
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

		// override any provider supplied by solr, if provider definition has a pattern that matches this url
		for _, provider := range s.pool.config.Global.Providers {
			if provider.re != nil && provider.re.MatchString(item) {
				f.Provider = provider.Name
				break
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

		// if not using supplied labels, or this supplied label is blank, fall back to generic item label
		// (but only if there are multiple urls that would be distinguished by such labels)
		if itemLabel == "" && len(urlValues) > 1 {
			itemLabel = fmt.Sprintf("%s %d", cfg.DefaultLabel, i+1)
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

	lastCallNumber := "---"

	for _, fieldValue := range fieldValues {
		parts := strings.Split(fieldValue, "|")
		if len(parts) != 6 {
			s.log("unexpected summary holding entry: [%s]", fieldValue)
			continue
		}

		// to be included in the summary holdings data sent to the client:
		library := parts[0]  // this must exist;
		location := parts[1] // this can be empty (otherwise some text/notes get omitted);
		text := parts[2]     // either this or note must exist;
		note := parts[3]     // either this or text must exist;
		//label := parts[4]      // this can be empty (if not, it will be one of "Library has", "Index text holdings", or "Suppl text holdings");
		callNumber := parts[5] // must have previously existed (we track the last call number seen).

		if callNumber != "" && callNumber != lastCallNumber {
			lastCallNumber = callNumber
		}

		if library != "" {
			if libraries[library] == nil {
				libraries[library] = make(map[string]map[string][]summaryTextNote)
			}

			if libraries[library][location] == nil {
				libraries[library][location] = make(map[string][]summaryTextNote)
			}

			if (text != "" || note != "") && lastCallNumber != "" {
				textNote := summaryTextNote{Text: text, Note: note}
				libraries[library][location][lastCallNumber] = append(libraries[library][location][lastCallNumber], textNote)
			}
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

func getCustomFieldAccessNote(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, override := range rc.fieldCtx.config.CustomConfig.Overrides {
		if s.evaluateConditions(rc.doc, override.Conditions) == true {
			rc.fieldCtx.field.Value = override.Value
			fv = append(fv, rc.fieldCtx.field)
			return fv
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

func getCustomFieldAuthenticate(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.anonRequest == true && rc.anonOnline == false && rc.authOnline == true {
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldAuthor(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.hasVernacularAuthor == true {
		rc.fieldCtx.field.Type = rc.fieldCtx.config.CustomConfig.AlternateType
		rc.fieldCtx.config.Label = rc.fieldCtx.config.CustomConfig.AlternateLabel
	}

	for _, n := range rc.relations.authors.values {
		rc.fieldCtx.field.Value = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	for _, n := range rc.relations.editors.values {
		rc.fieldCtx.field.Value = n.NameRelation
		rc.fieldCtx.field.AlternateValue = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	for _, n := range rc.relations.advisors.values {
		rc.fieldCtx.field.Value = n.NameRelation
		rc.fieldCtx.field.AlternateValue = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldAuthorVernacular(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.hasVernacularAuthor == true {
		rc.fieldCtx.field.Type = rc.fieldCtx.config.CustomConfig.AlternateType
		rc.fieldCtx.config.Label = rc.fieldCtx.config.CustomConfig.AlternateLabel
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

	// only want to include advisors.list for records NOT in the pre-defined list
	if s.evaluateConditions(rc.doc, rc.fieldCtx.config.CustomConfig.Conditions) == false {
		for _, n := range rc.relations.advisors.values {
			rc.fieldCtx.field.Value = n.Name
			fv = append(fv, rc.fieldCtx.field)
		}
	}

	return fv
}

func getCustomFieldCitationAuthor(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, n := range rc.relations.authors.values {
		rc.fieldCtx.field.Value = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCitationCompiler(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, n := range rc.relations.compilers.values {
		rc.fieldCtx.field.Value = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCitationEditor(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, n := range rc.relations.editors.values {
		rc.fieldCtx.field.Value = n.Name
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

	if s.evaluateConditions(rc.doc, rc.fieldCtx.config.CustomConfig.Conditions) == true {
		rc.fieldCtx.field.Value = "true"
	} else {
		rc.fieldCtx.field.Value = "false"
	}

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func getCustomFieldCitationIsVirgoURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if s.evaluateConditions(rc.doc, rc.fieldCtx.config.CustomConfig.Conditions) == true {
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

	for _, n := range rc.relations.translators.values {
		rc.fieldCtx.field.Value = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCollectionContext(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := append(rc.doc.getStrings(rc.fieldCtx.config.Field), rc.doc.getStrings(rc.fieldCtx.config.CustomConfig.AlternateField)...)

	if len(values) == 0 {
		return fv
	}

	// select an arbitrary entry

	value := values[0]

	rc.fieldCtx.field.Value = value
	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func getCustomFieldComposerPerformer(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, n := range rc.relations.authors.values {
		rc.fieldCtx.field.Value = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	for _, n := range rc.relations.editors.values {
		rc.fieldCtx.field.Value = n.NameRelation
		rc.fieldCtx.field.AlternateValue = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	for _, n := range rc.relations.advisors.values {
		rc.fieldCtx.field.Value = n.NameRelation
		rc.fieldCtx.field.AlternateValue = n.Name
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
		var values []string

		for _, n := range rc.relations.authors.values {
			values = append(values, n.Name)
		}

		coverImageURL = s.getCoverImageURL(rc.fieldCtx.config.CustomConfig, rc.doc, values)
	}

	if coverImageURL != "" {
		rc.fieldCtx.field.Value = coverImageURL
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldCreator(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, n := range rc.relations.authors.values {
		rc.fieldCtx.field.Value = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	for _, n := range rc.relations.editors.values {
		rc.fieldCtx.field.Value = n.NameRelation
		rc.fieldCtx.field.AlternateValue = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	for _, n := range rc.relations.advisors.values {
		rc.fieldCtx.field.Value = n.NameRelation
		rc.fieldCtx.field.AlternateValue = n.Name
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldDigitalContentURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if url := s.getDigitalContentURL(rc.doc, s.pool.config.Local.Solr.IdentifierField); url != "" {
		rc.fieldCtx.field.Value = url
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldExtentOfDigitization(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	// flag archival pool archival/manuscript records with digital content as partially digitized.
	// some hardcoding acceptable here since this should eventually go into tracksys-enrich

	if len(values) == 0 && rc.hasDigitalContent == true {
		// has digital content, and not already marked as partially digitized...

		pools := rc.doc.getStrings(rc.fieldCtx.config.CustomConfig.PoolField)
		if sliceContainsString(pools, "archival", true) == true {
			// pool is archival...

			callNumbers := rc.doc.getStrings(rc.fieldCtx.config.CustomConfig.CallNumberField)
			for _, callNumber := range callNumbers {
				cn := strings.ToUpper(callNumber)
				if strings.HasPrefix(cn, "MSS") || strings.HasPrefix(cn, "RG-") {
					// call number has manuscript (MSS*) or archival (RG-*) prefix...
					values = []string{"partial"}
				}
			}
		}
	}

	for _, value := range values {
		rc.fieldCtx.field.Value = value
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

func getCustomFieldLibraryAvailabilityNote(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, override := range rc.fieldCtx.config.CustomConfig.Overrides {
		if s.evaluateConditions(rc.doc, override.Conditions) == true {
			rc.fieldCtx.field.Value = override.Value
			fv = append(fv, rc.fieldCtx.field)
			return fv
		}
	}

	return fv
}

func getCustomFieldOnlineRelated(s *searchContext, rc *recordContext) []v4api.RecordField {
	return s.getLabelledURLs(rc.fieldCtx.field, rc.doc, rc.fieldCtx.config.CustomConfig)
}

func getCustomFieldPublishedDate(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isWSLS == true {
		rc.fieldCtx.config.Label = rc.fieldCtx.config.CustomConfig.AlternateLabel
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

func getCustomFieldShelfBrowseURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	// we are checking for the absence of solr values by abusing limitations of the comparison field structure,
	// so a "match" here means this item is NOT part of the shelf browse list
	if s.evaluateConditions(rc.doc, rc.fieldCtx.config.CustomConfig.Conditions) == true {
		return fv
	}

	if url := s.getShelfBrowseURL(rc.doc, s.pool.config.Local.Solr.IdentifierField); url != "" {
		rc.fieldCtx.field.Value = url
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func getCustomFieldSirsiURL(s *searchContext, rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isSirsi == true {
		idValue := s.getSolrIdentifierFieldValue(rc.doc)
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
		rc.fieldCtx.config.Label = rc.fieldCtx.config.CustomConfig.AlternateLabel
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
		rc.fieldCtx.config.Label = rc.fieldCtx.config.CustomConfig.AlternateLabel
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
		rc.fieldCtx.config.Label = rc.fieldCtx.config.CustomConfig.AlternateLabel
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
		rc.fieldCtx.config.Label = rc.fieldCtx.config.CustomConfig.AlternateLabel
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
		rc.fieldCtx.field.Value = rc.fieldCtx.config.CustomConfig.AlternateValue
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}
