package main

import (
	"fmt"
	"strings"

	"github.com/igorsobreira/titlecase"
	"github.com/uvalib/virgo4-api/v4api"
)

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

func (s *searchContext) getLabelledURLs(f v4api.RecordField, doc *solrDocument, cfg *poolConfigFieldTypeCustom) []v4api.RecordField {
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

func (s *searchContext) getSummaryHoldings(fieldValues []string) interface{} {
	type summaryCallNumber struct {
		CallNumber string   `json:"call_number"`
		Texts      []string `json:"text,omitempty"`
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

	libraries := make(map[string]map[string]map[string][]string)

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
		//note := parts[3]
		//label := parts[4]
		callNumber := parts[5]

		if library != "" && libraries[library] == nil {
			libraries[library] = make(map[string]map[string][]string)
		}

		if library != "" && location != "" && libraries[library][location] == nil {
			libraries[library][location] = make(map[string][]string)
		}

		if callNumber != "" && callNumber != lastCallNumber {
			lastCallNumber = callNumber
		}

		if text != "" {
			libraries[library][location][lastCallNumber] = append(libraries[library][location][lastCallNumber], text)
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
				nnum := summaryCallNumber{CallNumber: knum, Texts: vnum}
				nloc.CallNumbers = append(nloc.CallNumbers, nnum)
			}

			nlib.Locations = append(nlib.Locations, nloc)
		}

		resp.Holdings.Libraries = append(resp.Holdings.Libraries, nlib)
	}

	return resp
}

func (s *searchContext) buildTitle(title, subtitle, edition, vernacular string) string {
	fullTitle := titlecase.Title(title)

	if subtitle != "" {
		fullTitle = fmt.Sprintf("%s: %s", fullTitle, titlecase.Title(subtitle))
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

func (s *searchContext) getCustomFieldAbstract(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if len(values) == 0 {
		values = rc.doc.getStrings(rc.fieldCtx.config.CustomInfo.Abstract.AlternateField)
	}

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldAccessURL(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.anonOnline == false && rc.authOnline == false {
		return fv
	}

	rc.fieldCtx.field.Provider = rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.AccessURL.ProviderField)

	fv = s.getLabelledURLs(rc.fieldCtx.field, rc.doc, rc.fieldCtx.config.CustomInfo.AccessURL)

	return fv
}

func (s *searchContext) getCustomFieldAuthenticate(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.anonRequest == true && rc.anonOnline == false && rc.authOnline == true {
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldAuthor(rc *recordContext) []v4api.RecordField {
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

func (s *searchContext) getCustomFieldAuthorList(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.authors.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldAvailability(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.availabilityValues {
		if sliceContainsString(s.pool.config.Global.Availability.ExposedValues, value, true) {
			rc.fieldCtx.field.Value = value
			fv = append(fv, rc.fieldCtx.field)
		}
	}

	return fv
}

func (s *searchContext) getCustomFieldCitationAdvisor(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.advisors.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldCitationAuthor(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.authors.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldCitationCompiler(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.compilers.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldCitationEditor(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.editors.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldCitationFormat(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	rc.fieldCtx.field.Value = s.getCitationFormat(values)

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func (s *searchContext) getCustomFieldCitationIsOnlineOnly(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if s.compareFields(rc.doc, rc.fieldCtx.config.CustomInfo.CitationOnlineOnly.ComparisonFields) == true {
		rc.fieldCtx.field.Value = "true"
	} else {
		rc.fieldCtx.field.Value = "false"
	}

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func (s *searchContext) getCustomFieldCitationIsVirgoURL(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if s.compareFields(rc.doc, rc.fieldCtx.config.CustomInfo.CitationVirgoURL.ComparisonFields) == true {
		rc.fieldCtx.field.Value = "true"
	} else {
		rc.fieldCtx.field.Value = "false"
	}

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func (s *searchContext) getCustomFieldCitationSubtitle(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	subtitle := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	rc.fieldCtx.field.Value = titlecase.Title(subtitle)

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func (s *searchContext) getCustomFieldCitationTitle(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	title := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	rc.fieldCtx.field.Value = titlecase.Title(title)

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func (s *searchContext) getCustomFieldCitationTranslator(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	for _, value := range rc.relations.translators.name {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldComposerPerformer(rc *recordContext) []v4api.RecordField {
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

func (s *searchContext) getCustomFieldCopyrightAndPermissions(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if label, url, icon := s.getCopyrightLabelURLIcon(rc.doc); label != "" {
		rc.fieldCtx.field.Value = url
		rc.fieldCtx.field.Item = label
		rc.fieldCtx.field.Icon = icon
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldCoverImageURL(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	coverImageURL := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	if coverImageURL == "" {
		coverImageURL = s.getCoverImageURL(rc.fieldCtx.config.CustomInfo.CoverImageURL, rc.doc, rc.relations.authors.name)
	}

	if coverImageURL != "" {
		rc.fieldCtx.field.Value = coverImageURL
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldCreator(rc *recordContext) []v4api.RecordField {
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

func (s *searchContext) getCustomFieldDigitalContentURL(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if url := s.getDigitalContentURL(rc.doc, rc.fieldCtx.config.CustomInfo.DigitalContentURL.IDField); url != "" {
		rc.fieldCtx.field.Value = url
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldLanguage(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if len(values) == 0 {
		values = rc.doc.getStrings(rc.fieldCtx.config.CustomInfo.Language.AlternateField)
	}

	for _, value := range values {
		rc.fieldCtx.field.Value = value
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldOnlineRelated(rc *recordContext) []v4api.RecordField {
	return s.getLabelledURLs(rc.fieldCtx.field, rc.doc, rc.fieldCtx.config.CustomInfo.AccessURL)
}

func (s *searchContext) getCustomFieldPdfDownloadURL(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	pidValues := rc.doc.getStrings(rc.fieldCtx.config.CustomInfo.PdfDownloadURL.PIDField)

	if len(pidValues) <= rc.fieldCtx.config.CustomInfo.PdfDownloadURL.MaxSupported {
		pdfURL := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.PdfDownloadURL.URLField)

		if pdfURL == "" {
			return fv
		}

		for _, pid := range pidValues {
			if pid == "" {
				return fv
			}

			statusURL := fmt.Sprintf("%s/%s%s", pdfURL, pid, s.pool.config.Global.Service.Pdf.Endpoints.Status)

			pdfStatus, pdfErr := s.getPdfStatus(statusURL)

			if pdfErr != nil {
				return fv
			}

			if sliceContainsString(s.pool.config.Global.Service.Pdf.ReadyValues, pdfStatus, true) == true {
				downloadURL := fmt.Sprintf("%s/%s%s", pdfURL, pid, s.pool.config.Global.Service.Pdf.Endpoints.Download)
				rc.fieldCtx.field.Value = downloadURL
				fv = append(fv, rc.fieldCtx.field)
			}
		}
	}

	return fv
}

func (s *searchContext) getCustomFieldPublishedLocation(rc *recordContext) []v4api.RecordField {
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

func (s *searchContext) getCustomFieldPublisherName(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if len(values) == 0 {
		values = rc.doc.getStrings(rc.fieldCtx.config.CustomInfo.PublisherName.AlternateField)
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

func (s *searchContext) getCustomFieldRelatedResources(rc *recordContext) []v4api.RecordField {
	return s.getLabelledURLs(rc.fieldCtx.field, rc.doc, rc.fieldCtx.config.CustomInfo.AccessURL)
}

func (s *searchContext) getCustomFieldSirsiURL(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isSirsi == true {
		idValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.SirsiURL.IDField)
		idPrefix := rc.fieldCtx.config.CustomInfo.SirsiURL.IDPrefix

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

func (s *searchContext) getCustomFieldSummaryHoldings(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	values := rc.doc.getStrings(rc.fieldCtx.config.Field)

	if summaryHoldings := s.getSummaryHoldings(values); summaryHoldings != nil {
		rc.fieldCtx.field.StructuredValue = summaryHoldings
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldTitleSubtitleEdition(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	titleValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.TitleSubtitleEdition.TitleField)
	subtitleValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.TitleSubtitleEdition.SubtitleField)
	editionValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.TitleSubtitleEdition.EditionField)
	vernacularValue := ""

	rc.fieldCtx.field.Value = s.buildTitle(titleValue, subtitleValue, editionValue, vernacularValue)

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func (s *searchContext) getCustomFieldVernacularizedAuthor(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	vernacularValue := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	authorValues := rc.relations.authors.name

	for _, authorValue := range authorValues {
		rc.fieldCtx.field.Value = authorValue
		if vernacularValue != "" {
			rc.fieldCtx.field.Value += "<p>" + vernacularValue
		}
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldVernacularizedComposerPerformer(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	vernacularValue := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	authorValues := rc.relations.authors.name

	for _, authorValue := range authorValues {
		rc.fieldCtx.field.Value = authorValue
		if vernacularValue != "" {
			rc.fieldCtx.field.Value += "<p>" + vernacularValue
		}
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldVernacularizedCreator(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	vernacularValue := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	authorValues := rc.relations.authors.name

	for _, authorValue := range authorValues {
		rc.fieldCtx.field.Value = authorValue
		if vernacularValue != "" {
			rc.fieldCtx.field.Value += "<p>" + vernacularValue
		}
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}

func (s *searchContext) getCustomFieldVernacularizedTitle(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	titleValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.TitleSubtitleEdition.TitleField)
	subtitleValue := ""
	editionValue := ""
	vernacularValue := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	rc.fieldCtx.field.Value = s.buildTitle(titleValue, subtitleValue, editionValue, vernacularValue)

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func (s *searchContext) getCustomFieldVernacularizedTitleSubtitleEdition(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	titleValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.TitleSubtitleEdition.TitleField)
	subtitleValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.TitleSubtitleEdition.SubtitleField)
	editionValue := rc.doc.getFirstString(rc.fieldCtx.config.CustomInfo.TitleSubtitleEdition.EditionField)
	vernacularValue := rc.doc.getFirstString(rc.fieldCtx.config.Field)

	rc.fieldCtx.field.Value = s.buildTitle(titleValue, subtitleValue, editionValue, vernacularValue)

	fv = append(fv, rc.fieldCtx.field)

	return fv
}

func (s *searchContext) getCustomFieldWSLSCollectionDescription(rc *recordContext) []v4api.RecordField {
	var fv []v4api.RecordField

	if rc.isWSLS == true {
		rc.fieldCtx.field.Value = s.client.localize(rc.fieldCtx.config.CustomInfo.WSLSCollectionDescription.ValueXID)
		fv = append(fv, rc.fieldCtx.field)
	}

	return fv
}
