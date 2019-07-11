package main

import (
	"fmt"
	"strings"
)

// functions that map virgo data into solr data

func solrBuildParameterQ(v VirgoSearchRequest) string {
	q := v.solrQuery

	return q
}

func restrictValue(val int, min int, fallback int) int {
	// default, if requested value isn't large enough
	res := fallback

	if val >= min {
		res = val
	}

	return res
}

func nonemptyValues(val []string) []string {
	res := []string{}

	for _, s := range val {
		if s != "" {
			res = append(res, s)
		}
	}

	return res
}

func solrBuildParameterStart(s int) int {
	start := restrictValue(s, 0, 0)

	return start
}

func solrBuildParameterRows(r int) int {
	rows := restrictValue(r, 1, 10)

	return rows
}

func solrBuildParameterQt() string {
	qt := config.solrParameterQt.value

	return qt
}

func solrBuildParameterDefType() string {
	deftype := config.solrParameterDefType.value

	return deftype
}

func solrBuildParameterFq() []string {
	// leaders must be defined with beginning + or -

	fqall := []string{config.solrParameterFq.value, config.poolLeaders.value}

	fq := nonemptyValues(fqall)

	return fq
}

func solrBuildParameterFl() []string {
	flall := strings.Split(config.solrParameterFl.value, ",")

	fl := nonemptyValues(flall)

	return fl
}

func solrRequestWithDefaults(v VirgoSearchRequest) solrRequest {
	var solrReq solrRequest

	// fill out as much as we can for a generic request
	solrReq.json.Params.Q = solrBuildParameterQ(v)
	solrReq.json.Params.Qt = solrBuildParameterQt()
	solrReq.json.Params.DefType = solrBuildParameterDefType()
	solrReq.json.Params.appendFq(solrBuildParameterFq())
	solrReq.json.Params.appendFl(solrBuildParameterFl())

	if v.Pagination != nil {
		solrReq.json.Params.Start = solrBuildParameterStart(v.Pagination.Start)
		solrReq.json.Params.Rows = solrBuildParameterRows(v.Pagination.Rows)
	}

	return solrReq
}

func solrSearchRequest(v VirgoSearchRequest) (*solrRequest, error) {
	var err error

	var p *solrParserInfo

	// caller might have already supplied a Solr query
	if v.solrQuery == "" {
		if p, err = virgoQueryConvertToSolr(v.Query); err != nil {
			return nil, fmt.Errorf("Virgo query to Solr conversion error: %s", err.Error())
		}

		v.solrQuery = p.query
	}

	solrReq := solrRequestWithDefaults(v)

	solrReq.parserInfo = p

	return &solrReq, nil
}

func solrRecordRequest(v VirgoSearchRequest) (*solrRequest, error) {
	solrReq := solrRequestWithDefaults(v)

	// override these values from defaults.  specify two rows to catch
	// the (impossible?) scenario of multiple records with the same id
	solrReq.json.Params.Start = 0
	solrReq.json.Params.Rows = 2

	return &solrReq, nil
}
