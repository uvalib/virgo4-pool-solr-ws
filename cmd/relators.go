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
	authors  parsedRelation
	advisors parsedRelation
	editors  parsedRelation
	others   parsedRelation
}

type relationContext struct {
	relations    categorizedRelations
	search       *searchContext
	matchTermsRE *regexp.Regexp
	cleanTermsRE *regexp.Regexp
	cleanDatesRE *regexp.Regexp
}

func (s *searchContext) parseRelators(entries []string) categorizedRelations {
	terms := []string{}

	for _, term := range s.pool.config.Global.Relators.Map {
		terms = append(terms, term.Term)
	}

	r := relationContext{
		search:       s,
		matchTermsRE: regexp.MustCompile(`([\s(]*)([^\s()]+)([\s)]*)`),
		cleanTermsRE: regexp.MustCompile(`(?i)([\s(]*(` + strings.Join(terms, "|") + `)[\s)]*)`),
		cleanDatesRE: regexp.MustCompile(`([\s\[,]*)(\d{4}(-)?(\d{4})?)([\s\]]*)`),
	}

	for _, entry := range entries {
		code := r.getRelatorCode(entry)

		switch {
		case sliceContainsString(s.pool.config.Global.Relators.AuthorCodes, code):
			r.addAuthor(entry)

		case sliceContainsString(s.pool.config.Global.Relators.AdvisorCodes, code):
			r.addAdvisor(entry)

		case sliceContainsString(s.pool.config.Global.Relators.EditorCodes, code):
			r.addEditor(entry)

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

func (r *relationContext) addOther(entry string) {
	p := r.parseEntry(entry)

	r.relations.others.nameDateRelation = append(r.relations.others.nameDateRelation, p.nameDateRelation...)
	r.relations.others.nameDate = append(r.relations.others.nameDate, p.nameDate...)
	r.relations.others.nameRelation = append(r.relations.others.nameRelation, p.nameRelation...)
	r.relations.others.name = append(r.relations.others.name, p.name...)
}

func (r *relationContext) removeDuplicateEntries(entries []string) []string {
	var unique []string

	seen := make(map[string]bool)

	for _, entry := range entries {
		key := strings.ToLower(entry)

		if seen[key] == false {
			unique = append(unique, entry)
			seen[key] = true
		}
	}

	return unique
}

func (r *relationContext) removeDuplicateAuthors() {
	r.relations.authors.nameDateRelation = r.removeDuplicateEntries(r.relations.authors.nameDateRelation)
	r.relations.authors.nameDate = r.removeDuplicateEntries(r.relations.authors.nameDate)
	r.relations.authors.nameRelation = r.removeDuplicateEntries(r.relations.authors.nameRelation)
	r.relations.authors.name = r.removeDuplicateEntries(r.relations.authors.name)
}

func (r *relationContext) removeDuplicateAdvisors() {
	r.relations.advisors.nameDateRelation = r.removeDuplicateEntries(r.relations.advisors.nameDateRelation)
	r.relations.advisors.nameDate = r.removeDuplicateEntries(r.relations.advisors.nameDate)
	r.relations.advisors.nameRelation = r.removeDuplicateEntries(r.relations.advisors.nameRelation)
	r.relations.advisors.name = r.removeDuplicateEntries(r.relations.advisors.name)
}

func (r *relationContext) removeDuplicateEditors() {
	r.relations.editors.nameDateRelation = r.removeDuplicateEntries(r.relations.editors.nameDateRelation)
	r.relations.editors.nameDate = r.removeDuplicateEntries(r.relations.editors.nameDate)
	r.relations.editors.nameRelation = r.removeDuplicateEntries(r.relations.editors.nameRelation)
	r.relations.editors.name = r.removeDuplicateEntries(r.relations.editors.name)
}

func (r *relationContext) removeDuplicateOthers() {
	r.relations.others.nameDateRelation = r.removeDuplicateEntries(r.relations.others.nameDateRelation)
	r.relations.others.nameDate = r.removeDuplicateEntries(r.relations.others.nameDate)
	r.relations.others.nameRelation = r.removeDuplicateEntries(r.relations.others.nameRelation)
	r.relations.others.name = r.removeDuplicateEntries(r.relations.others.name)
}

func (r *relationContext) removeDuplicateRelations() {
	r.removeDuplicateAuthors()
	r.removeDuplicateAdvisors()
	r.removeDuplicateEditors()
	r.removeDuplicateOthers()
}
