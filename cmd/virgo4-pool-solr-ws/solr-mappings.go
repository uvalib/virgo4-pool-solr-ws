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

func solrBuildParameterStart(s int) string {
	// default, if requested value doesn't make sense
	startnum := 0

	if s >= 0 {
		startnum = s
	}

	start := fmt.Sprintf("%d", startnum)

	return start
}

func solrBuildParameterRows(r int) string {
	// default, if requested value doesn't make sense
	rownum := 10

	if r > 0 {
		rownum = r
	}

	rows := fmt.Sprintf("%d", rownum)

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

func solrBuildParameterFq() string {
	// leaders must be defined with beginning + or -

	fqall := []string{config.solrParameterFq.value, config.poolLeaders.value}
	fqs := []string{}

	for _, s := range fqall {
		if s != "" {
			fqs = append(fqs, s)
		}
	}

	fq := strings.Join(fqs, " ")

	return fq
}

func solrBuildParameterFl() string {
	fl := config.solrParameterFl.value

	return fl
}

func solrRequestWithDefaults(v VirgoSearchRequest) solrRequest {
	var solrReq solrRequest

	solrReq.params = make(solrParamsMap)

	// fill out as much as we can for a generic request
	solrReq.params["q"] = solrBuildParameterQ(v)
	solrReq.params["qt"] = solrBuildParameterQt()
	solrReq.params["defType"] = solrBuildParameterDefType()
	solrReq.params["fq"] = solrBuildParameterFq()
	solrReq.params["fl"] = solrBuildParameterFl()

	if v.Pagination != nil {
		solrReq.params["start"] = solrBuildParameterStart(v.Pagination.Start)
		solrReq.params["rows"] = solrBuildParameterRows(v.Pagination.Rows)
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

	// override these values from defaults
	solrReq.params["start"] = "0"
	solrReq.params["rows"] = "2"

	return &solrReq, nil
}
