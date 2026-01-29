package main

import (
	"regexp"
	"strings"
)

type parsedRelation struct {
	NameDateRelation string `json:"name_date_relation,omitempty"` // possible date, possible relation
	NameDate         string `json:"name_date,omitempty"`          // possible date, no relation
	NameRelation     string `json:"name_relation,omitempty"`      // no date, possible relation
	Name             string `json:"name,omitempty"`               // no date, no relation
}

type categorizedRelations struct {
	authors     []parsedRelation
	advisors    []parsedRelation
	editors     []parsedRelation
	compilers   []parsedRelation
	translators []parsedRelation
	others      []parsedRelation
}

type relationContext struct {
	relations    categorizedRelations
	search       *searchContext
	matchTermsRE *regexp.Regexp
	cleanTermsRE *regexp.Regexp
	cleanDatesRE *regexp.Regexp
}

func (s *searchContext) parseRelations(entries []string) categorizedRelations {
	terms := []string{}

	for _, term := range s.pool.config.Global.Relators.Map {
		terms = append(terms, term.Terms...)
	}

	r := relationContext{
		search:       s,
		matchTermsRE: regexp.MustCompile(`\(([^()]+)\)`),
		cleanTermsRE: regexp.MustCompile(`(?i)([\s]*\((` + strings.Join(terms, "|") + `)\)[\s]*)`),
		cleanDatesRE: regexp.MustCompile(`([\s\[,]*)((approximately )?\d{4}(-)?((approximately )?\d{4})?)([\s\]]*)`),
	}

	for _, entry := range entries {
		codes := r.getRelatorCodes(entry)

		switch {
		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.AuthorCodes, codes, true) || len(codes) == 0:
			r.addAuthor(entry)

		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.AdvisorCodes, codes, true):
			r.addAdvisor(entry)

		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.EditorCodes, codes, true):
			r.addEditor(entry)

		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.CompilerCodes, codes, true):
			r.addCompiler(entry)

		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.TranslatorCodes, codes, true):
			r.addTranslator(entry)

		default:
			r.addOther(entry)
		}
	}

	r.removeDuplicateRelations()

	return r.relations
}

func (r *relationContext) getRelatorCodes(entry string) []string {
	// find all matching relator terms and return their codes
	// NOTE: this assumes name entries contain parenthesized relator terms, which is
	//       the case if the preferred author solr field (author_facet_a) is populated.

	var codes []string

	terms := r.matchTermsRE.FindAllStringSubmatch(entry, -1)
	for _, term := range terms {
		if code := r.search.pool.maps.relatorCodes[strings.ToLower(term[1])]; code != "" {
			codes = append(codes, code)
		}
	}

	return codes
}

func (r *relationContext) parseEntry(entry string) parsedRelation {
	// start with fresh string with no extra spaces
	nameDateRelation := strings.TrimSpace(strings.ReplaceAll(entry, "  ", " "))

	// strip any date(s) from original string
	nameRelation := strings.TrimSpace(r.cleanDatesRE.ReplaceAllString(nameDateRelation, " "))

	// strip any term from original string
	nameDate := strings.TrimSpace(r.cleanTermsRE.ReplaceAllString(nameDateRelation, " "))

	// strip any date(s) from term-stripped string
	name := strings.TrimSpace(r.cleanDatesRE.ReplaceAllString(nameDate, " "))

	p := parsedRelation{
		NameDateRelation: nameDateRelation,
		NameDate:         nameDate,
		NameRelation:     nameRelation,
		Name:             name,
	}

	return p
}

func (r *relationContext) addAuthor(entry string) {
	r.relations.authors = append(r.relations.authors, r.parseEntry(entry))
}

func (r *relationContext) addAdvisor(entry string) {
	r.relations.advisors = append(r.relations.advisors, r.parseEntry(entry))
}

func (r *relationContext) addEditor(entry string) {
	r.relations.editors = append(r.relations.editors, r.parseEntry(entry))
}

func (r *relationContext) addCompiler(entry string) {
	r.relations.compilers = append(r.relations.compilers, r.parseEntry(entry))
}

func (r *relationContext) addTranslator(entry string) {
	r.relations.translators = append(r.relations.translators, r.parseEntry(entry))
}

func (r *relationContext) addOther(entry string) {
	r.relations.others = append(r.relations.others, r.parseEntry(entry))
}

func (r *relationContext) removeDuplicateAuthors() {
	/*
	   	for _, n := range r.relations.authors {
	   	}

	   /*

	   	r.relations.authors.NameDateRelation = uniqueStrings(r.relations.authors.NameDateRelation)
	   	r.relations.authors.NameDate = uniqueStrings(r.relations.authors.NameDate)
	   	r.relations.authors.NameRelation = uniqueStrings(r.relations.authors.NameRelation)
	   	r.relations.authors.name = uniqueStrings(r.relations.authors.name)
	*/
}

func (r *relationContext) removeDuplicateAdvisors() {
	/*
	   	for _, n := range r.relations.advisors {
	   	}

	   /*

	   	r.relations.advisors.NameDateRelation = uniqueStrings(r.relations.advisors.NameDateRelation)
	   	r.relations.advisors.NameDate = uniqueStrings(r.relations.advisors.NameDate)
	   	r.relations.advisors.NameRelation = uniqueStrings(r.relations.advisors.NameRelation)
	   	r.relations.advisors.name = uniqueStrings(r.relations.advisors.name)
	*/
}

func (r *relationContext) removeDuplicateEditors() {
	/*
	   	for _, n := range r.relations.editors {
	   	}

	   /*

	   	r.relations.editors.NameDateRelation = uniqueStrings(r.relations.editors.NameDateRelation)
	   	r.relations.editors.NameDate = uniqueStrings(r.relations.editors.NameDate)
	   	r.relations.editors.NameRelation = uniqueStrings(r.relations.editors.NameRelation)
	   	r.relations.editors.name = uniqueStrings(r.relations.editors.name)
	*/
}

func (r *relationContext) removeDuplicateCompilers() {
	/*
	   	for _, n := range r.relations.compilers {
	   	}

	   /*

	   	r.relations.compilers.NameDateRelation = uniqueStrings(r.relations.compilers.NameDateRelation)
	   	r.relations.compilers.NameDate = uniqueStrings(r.relations.compilers.NameDate)
	   	r.relations.compilers.NameRelation = uniqueStrings(r.relations.compilers.NameRelation)
	   	r.relations.compilers.name = uniqueStrings(r.relations.compilers.name)
	*/
}

func (r *relationContext) removeDuplicateTranslators() {
	/*
	   	for _, n := range r.relations.translators {
	   	}

	   /*

	   	r.relations.translators.NameDateRelation = uniqueStrings(r.relations.translators.NameDateRelation)
	   	r.relations.translators.NameDate = uniqueStrings(r.relations.translators.NameDate)
	   	r.relations.translators.NameRelation = uniqueStrings(r.relations.translators.NameRelation)
	   	r.relations.translators.name = uniqueStrings(r.relations.translators.name)
	*/
}

func (r *relationContext) removeDuplicateOthers() {
	/*
	   	for _, n := range r.relations.others {
	   	}

	   /*

	   	r.relations.others.NameDateRelation = uniqueStrings(r.relations.others.NameDateRelation)
	   	r.relations.others.NameDate = uniqueStrings(r.relations.others.NameDate)
	   	r.relations.others.NameRelation = uniqueStrings(r.relations.others.NameRelation)
	   	r.relations.others.name = uniqueStrings(r.relations.others.name)
	*/
}

func (r *relationContext) removeDuplicateRelations() {
	r.removeDuplicateAuthors()
	r.removeDuplicateAdvisors()
	r.removeDuplicateEditors()
	r.removeDuplicateCompilers()
	r.removeDuplicateTranslators()
	r.removeDuplicateOthers()
}
