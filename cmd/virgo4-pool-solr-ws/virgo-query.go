package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/uvalib/virgo4-parser/v4parser"
)

// Solr is an implementation of the v4Parser that transforms queries into solr queries
type Solr struct {
	*v4parser.BaseVirgoQueryListener
	searchStack []string
}

// EnterSearch_string is called when the parser begins parsing a search string
func (s *Solr) EnterSearch_string(ctx *v4parser.Search_stringContext) {
	// push a new blank string onto the search string stack. Search_part will populate it,
	// and ExitSearch completes it
	s.searchStack = append(s.searchStack, "")
}

// ExitSearch_string is called when the parser finshes with all parts of a search string
func (s *Solr) ExitSearch_string(ctx *v4parser.Search_stringContext) {
	// pop last node from stack
	out := s.searchStack[len(s.searchStack)-1]
	s.searchStack = s.searchStack[:len(s.searchStack)-1]
	log.Printf("SEARCH STRING: %s", out)
}

// ExitSearch_part is called when the parser has parsed part of a search term; a single word or quotes that
// will go around the currently parsing search string
func (s *Solr) ExitSearch_part(ctx *v4parser.Search_partContext) {
	// get just the text of the part that was just parsed; if
	// you try just GetText on ctx, you get the full search string all
	// mashed together with no spaces
	st := ctx.GetStop()
	tt := st.GetTokenType()
	t := ctx.GetToken(tt, 0)
	val := t.GetText()
	qs := s.searchStack[len(s.searchStack)-1]
	s.searchStack = s.searchStack[:len(s.searchStack)-1]
	if val != `"` {
		if qs != "" {
			qs += " "
		}
		qs += val
	} else {
		qs = fmt.Sprintf(`"%s"`, qs)
	}
	s.searchStack = append(s.searchStack, qs)
}

func (s *Solr) ExitField_type(ctx *v4parser.Field_typeContext) {
	log.Printf("ExitField_type: %s", ctx.GetText())
}

func (s *Solr) ExitField_query(ctx *v4parser.Field_queryContext) {
	log.Printf("ExitField_query: %s", ctx.GetText())
}

// ExitBoolean_op is called when an OR or AND has just been parsed
func (s *Solr) ExitBoolean_op(ctx *v4parser.Boolean_opContext) {
	log.Printf("ExitBoolean_op: %s", ctx.GetText())
}

// Convert convert a v4 query string into solr
func (v *Solr) Convert(src string) (string, error) {
	// EXAMPLE: `( title : {"susan sontag" OR music title}   AND keyword:{ Maunsell } ) OR author:{ liberty }`
	// SOLR: ( ( ((_query_:"{!edismax qf=$title_qf pf=$title_pf}(\" susan sontag \")" OR _query_:"{!edismax qf=$title_qf pf=$title_pf}(music title)")
	//              AND _query_:"{!edismax}(Maunsell)") )  OR _query_:"{!edismax qf=$author_qf pf=$author_pf}(liberty)")
	//
	log.Printf("Convert to Solr: %s", src)
	is := antlr.NewInputStream(src)
	lexer := v4parser.NewVirgoQueryLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	queryTree := v4parser.NewVirgoQuery(stream)
	solrConv := Solr{}
	antlr.ParseTreeWalkerDefault.Walk(&solrConv, queryTree.Query())

	return "", errors.New("Not Implemented")
}
