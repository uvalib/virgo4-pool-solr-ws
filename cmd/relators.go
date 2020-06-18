package main

import (
	"regexp"
	"strings"
)

type parsedRelation struct {
	dr []string // possible date, possible relation
	dx []string // possible date, no relation
	xr []string // no date, possible relation
	xx []string // no date, no relation
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

	for _, term := range s.pool.config.Global.Service.Relators.Map {
		terms = append(terms, term.Term)
	}

	r := relationContext{
		search: s,
		matchTermsRE: regexp.MustCompile(`([\s(]*)([^\s()]+)([\s)]*)`),
		cleanTermsRE: regexp.MustCompile(`(?i)([\s(]*(` + strings.Join(terms, "|") + `)[\s)]*)`),
		cleanDatesRE: regexp.MustCompile(`([\s\[,]*)(\d{4}(-)?(\d{4})?)([\s\]]*)`),
	}

	for _, entry := range entries {
		code := r.getRelatorCode(entry)

		switch {
		case sliceContainsString(s.pool.config.Global.Service.Relators.AuthorCodes, code):
			r.addAuthor(entry)

		case sliceContainsString(s.pool.config.Global.Service.Relators.AdvisorCodes, code):
			r.addAdvisor(entry)

		case sliceContainsString(s.pool.config.Global.Service.Relators.EditorCodes, code):
			r.addAuthor(entry)

		default:
			r.addOther(entry)
		}
	}

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
	dr := strings.TrimSpace(strings.ReplaceAll(entry, "  ", " "))

	// strip any date(s) from original string
	xr := strings.TrimSpace(r.cleanDatesRE.ReplaceAllString(dr, " "))

	// strip any term from original string
	dx := strings.TrimSpace(r.cleanTermsRE.ReplaceAllString(dr, " "))

	// strip any date(s) from term-stripped string
	xx := strings.TrimSpace(r.cleanDatesRE.ReplaceAllString(dx, " "))

	p := parsedRelation{
		dr: []string{dr},
		dx: []string{dx},
		xr: []string{xr},
		xx: []string{xx},
	}

	return p
}

func (r *relationContext) addAuthor(entry string) {
	p := r.parseEntry(entry)

	r.relations.authors.dr = append(r.relations.authors.dr, p.dr...)
	r.relations.authors.dx = append(r.relations.authors.dx, p.dx...)
	r.relations.authors.xr = append(r.relations.authors.xr, p.xr...)
	r.relations.authors.xx = append(r.relations.authors.xx, p.xx...)
}

func (r *relationContext) addAdvisor(entry string) {
	p := r.parseEntry(entry)

	r.relations.advisors.dr = append(r.relations.advisors.dr, p.dr...)
	r.relations.advisors.dx = append(r.relations.advisors.dx, p.dx...)
	r.relations.advisors.xr = append(r.relations.advisors.xr, p.xr...)
	r.relations.advisors.xx = append(r.relations.advisors.xx, p.xx...)
}

func (r *relationContext) addEditor(entry string) {
	p := r.parseEntry(entry)

	r.relations.editors.dr = append(r.relations.editors.dr, p.dr...)
	r.relations.editors.dx = append(r.relations.editors.dx, p.dx...)
	r.relations.editors.xr = append(r.relations.editors.xr, p.xr...)
	r.relations.editors.xx = append(r.relations.editors.xx, p.xx...)
}

func (r *relationContext) addOther(entry string) {
	p := r.parseEntry(entry)

	r.relations.others.dr = append(r.relations.others.dr, p.dr...)
	r.relations.others.dx = append(r.relations.others.dx, p.dx...)
	r.relations.others.xr = append(r.relations.others.xr, p.xr...)
	r.relations.others.xx = append(r.relations.others.xx, p.xx...)
}
