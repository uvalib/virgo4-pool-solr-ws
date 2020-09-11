package main

import (
	"regexp"
	"strings"
)

type parsedRelation struct {
	nameDateRelation []string // possible date, possible relation
	nameDate         []string // possible date, no relation
	nameRelation     []string // no date, possible relation
	name             []string // no date, no relation
}

type categorizedRelations struct {
	authors     parsedRelation
	advisors    parsedRelation
	editors     parsedRelation
	compilers   parsedRelation
	translators parsedRelation
	others      parsedRelation
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
		terms = append(terms, term.Term)
	}

	r := relationContext{
		search:       s,
		matchTermsRE: regexp.MustCompile(`([\s(]*)([^\s()]+)([\s)]*)`),
		cleanTermsRE: regexp.MustCompile(`(?i)([\s]*\((` + strings.Join(terms, "|") + `)\)[\s]*)`),
		cleanDatesRE: regexp.MustCompile(`([\s\[,]*)(\d{4}(-)?(\d{4})?)([\s\]]*)`),
	}

	for _, entry := range entries {
		code := r.getRelatorCode(entry)

		switch {
		case sliceContainsString(s.pool.config.Global.Relators.AuthorCodes, code, true):
			r.addAuthor(entry)

		case sliceContainsString(s.pool.config.Global.Relators.AdvisorCodes, code, true):
			r.addAdvisor(entry)

		case sliceContainsString(s.pool.config.Global.Relators.EditorCodes, code, true):
			r.addEditor(entry)

		case sliceContainsString(s.pool.config.Global.Relators.CompilerCodes, code, true):
			r.addCompiler(entry)

		case sliceContainsString(s.pool.config.Global.Relators.TranslatorCodes, code, true):
			r.addTranslator(entry)

		default:
			r.addOther(entry)
		}
	}

	r.removeDuplicateRelations()

	return r.relations
}

func (r *relationContext) getRelatorCode(entry string) string {
	// find first matching relator term and return its code

	terms := r.matchTermsRE.FindAllStringSubmatch(entry, -1)
	for _, term := range terms {
		code := r.search.pool.maps.relatorCodes[term[2]]
		if code != "" {
			return code
		}
	}

	// no match; assume author
	return "aut"
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
		nameDateRelation: []string{nameDateRelation},
		nameDate:         []string{nameDate},
		nameRelation:     []string{nameRelation},
		name:             []string{name},
	}

	return p
}

func (r *relationContext) addAuthor(entry string) {
	p := r.parseEntry(entry)

	r.relations.authors.nameDateRelation = append(r.relations.authors.nameDateRelation, p.nameDateRelation...)
	r.relations.authors.nameDate = append(r.relations.authors.nameDate, p.nameDate...)
	r.relations.authors.nameRelation = append(r.relations.authors.nameRelation, p.nameRelation...)
	r.relations.authors.name = append(r.relations.authors.name, p.name...)
}

func (r *relationContext) addAdvisor(entry string) {
	p := r.parseEntry(entry)

	r.relations.advisors.nameDateRelation = append(r.relations.advisors.nameDateRelation, p.nameDateRelation...)
	r.relations.advisors.nameDate = append(r.relations.advisors.nameDate, p.nameDate...)
	r.relations.advisors.nameRelation = append(r.relations.advisors.nameRelation, p.nameRelation...)
	r.relations.advisors.name = append(r.relations.advisors.name, p.name...)
}

func (r *relationContext) addEditor(entry string) {
	p := r.parseEntry(entry)

	r.relations.editors.nameDateRelation = append(r.relations.editors.nameDateRelation, p.nameDateRelation...)
	r.relations.editors.nameDate = append(r.relations.editors.nameDate, p.nameDate...)
	r.relations.editors.nameRelation = append(r.relations.editors.nameRelation, p.nameRelation...)
	r.relations.editors.name = append(r.relations.editors.name, p.name...)
}

func (r *relationContext) addCompiler(entry string) {
	p := r.parseEntry(entry)

	r.relations.compilers.nameDateRelation = append(r.relations.compilers.nameDateRelation, p.nameDateRelation...)
	r.relations.compilers.nameDate = append(r.relations.compilers.nameDate, p.nameDate...)
	r.relations.compilers.nameRelation = append(r.relations.compilers.nameRelation, p.nameRelation...)
	r.relations.compilers.name = append(r.relations.compilers.name, p.name...)
}

func (r *relationContext) addTranslator(entry string) {
	p := r.parseEntry(entry)

	r.relations.translators.nameDateRelation = append(r.relations.translators.nameDateRelation, p.nameDateRelation...)
	r.relations.translators.nameDate = append(r.relations.translators.nameDate, p.nameDate...)
	r.relations.translators.nameRelation = append(r.relations.translators.nameRelation, p.nameRelation...)
	r.relations.translators.name = append(r.relations.translators.name, p.name...)
}

func (r *relationContext) addOther(entry string) {
	p := r.parseEntry(entry)

	r.relations.others.nameDateRelation = append(r.relations.others.nameDateRelation, p.nameDateRelation...)
	r.relations.others.nameDate = append(r.relations.others.nameDate, p.nameDate...)
	r.relations.others.nameRelation = append(r.relations.others.nameRelation, p.nameRelation...)
	r.relations.others.name = append(r.relations.others.name, p.name...)
}

func (r *relationContext) removeDuplicateAuthors() {
	r.relations.authors.nameDateRelation = uniqueStrings(r.relations.authors.nameDateRelation)
	r.relations.authors.nameDate = uniqueStrings(r.relations.authors.nameDate)
	r.relations.authors.nameRelation = uniqueStrings(r.relations.authors.nameRelation)
	r.relations.authors.name = uniqueStrings(r.relations.authors.name)
}

func (r *relationContext) removeDuplicateAdvisors() {
	r.relations.advisors.nameDateRelation = uniqueStrings(r.relations.advisors.nameDateRelation)
	r.relations.advisors.nameDate = uniqueStrings(r.relations.advisors.nameDate)
	r.relations.advisors.nameRelation = uniqueStrings(r.relations.advisors.nameRelation)
	r.relations.advisors.name = uniqueStrings(r.relations.advisors.name)
}

func (r *relationContext) removeDuplicateEditors() {
	r.relations.editors.nameDateRelation = uniqueStrings(r.relations.editors.nameDateRelation)
	r.relations.editors.nameDate = uniqueStrings(r.relations.editors.nameDate)
	r.relations.editors.nameRelation = uniqueStrings(r.relations.editors.nameRelation)
	r.relations.editors.name = uniqueStrings(r.relations.editors.name)
}

func (r *relationContext) removeDuplicateCompilers() {
	r.relations.compilers.nameDateRelation = uniqueStrings(r.relations.compilers.nameDateRelation)
	r.relations.compilers.nameDate = uniqueStrings(r.relations.compilers.nameDate)
	r.relations.compilers.nameRelation = uniqueStrings(r.relations.compilers.nameRelation)
	r.relations.compilers.name = uniqueStrings(r.relations.compilers.name)
}

func (r *relationContext) removeDuplicateTranslators() {
	r.relations.translators.nameDateRelation = uniqueStrings(r.relations.translators.nameDateRelation)
	r.relations.translators.nameDate = uniqueStrings(r.relations.translators.nameDate)
	r.relations.translators.nameRelation = uniqueStrings(r.relations.translators.nameRelation)
	r.relations.translators.name = uniqueStrings(r.relations.translators.name)
}

func (r *relationContext) removeDuplicateOthers() {
	r.relations.others.nameDateRelation = uniqueStrings(r.relations.others.nameDateRelation)
	r.relations.others.nameDate = uniqueStrings(r.relations.others.nameDate)
	r.relations.others.nameRelation = uniqueStrings(r.relations.others.nameRelation)
	r.relations.others.name = uniqueStrings(r.relations.others.name)
}

func (r *relationContext) removeDuplicateRelations() {
	r.removeDuplicateAuthors()
	r.removeDuplicateAdvisors()
	r.removeDuplicateEditors()
	r.removeDuplicateCompilers()
	r.removeDuplicateTranslators()
	r.removeDuplicateOthers()
}
