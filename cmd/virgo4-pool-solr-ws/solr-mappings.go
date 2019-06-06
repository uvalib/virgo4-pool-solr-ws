package main

import (
	"errors"
	"fmt"
)

// functions that map virgo data into solr data

func solrBuildParameterQ(v VirgoSearchRequest) string {
	q := v.SolrQuery

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

	fq := fmt.Sprintf("%s %s", config.solrParameterFq.value, pool.leaders)

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

	if v.SolrQuery, err = virgoQueryConvertToSolr(v.Query); err != nil {
		return nil, errors.New(fmt.Sprintf("Virgo query to Solr conversion error: %s", err.Error()))
	}

	solrReq := solrRequestWithDefaults(v)

	return &solrReq, nil
}

func solrRecordRequest(id string) (*solrRequest, error) {
	v := VirgoSearchRequest{}

	v.SolrQuery = fmt.Sprintf("id:%s", id)

	solrReq := solrRequestWithDefaults(v)

	// override these values from defaults
	solrReq.params["start"] = "0"
	solrReq.params["rows"] = "2"

	return &solrReq, nil
}
