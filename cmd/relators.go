package main

import (
	"fmt"
	"regexp"
	"strings"
)

type parsedRelation struct {
	NameDateRelation string `json:"name_date_relation,omitempty"` // possible date, possible relation
	NameDate         string `json:"name_date,omitempty"`          // possible date, no relation
	NameRelation     string `json:"name_relation,omitempty"`      // no date, possible relation
	Name             string `json:"name,omitempty"`               // no date, no relation
}

type relationList struct {
	values []parsedRelation
	exist  map[string]bool
}

type categorizedRelations struct {
	authors     relationList
	advisors    relationList
	editors     relationList
	compilers   relationList
	translators relationList
	others      relationList
}

type relationContext struct {
	relations    categorizedRelations
	search       *searchContext
	matchTermsRE *regexp.Regexp
	cleanTermsRE *regexp.Regexp
	cleanDatesRE *regexp.Regexp
}

func (l *relationList) addRelation(relation parsedRelation) {
	if l.exist == nil {
		l.exist = make(map[string]bool)
	}

	key := fmt.Sprintf("%s|%s|%s|%s", relation.Name, relation.NameRelation, relation.NameDate, relation.NameDateRelation)

	if l.exist[key] == true {
		return
	}

	l.values = append(l.values, relation)

	l.exist[key] = true
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

		parsed := r.parseEntry(entry)

		switch {
		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.AuthorCodes, codes, true) || len(codes) == 0:
			r.relations.authors.addRelation(parsed)

		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.AdvisorCodes, codes, true):
			r.relations.advisors.addRelation(parsed)

		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.EditorCodes, codes, true):
			r.relations.editors.addRelation(parsed)

		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.CompilerCodes, codes, true):
			r.relations.compilers.addRelation(parsed)

		case sliceContainsAnyValueFromSlice(s.pool.config.Global.Relators.TranslatorCodes, codes, true):
			r.relations.translators.addRelation(parsed)

		default:
			r.relations.others.addRelation(parsed)
		}
	}

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
