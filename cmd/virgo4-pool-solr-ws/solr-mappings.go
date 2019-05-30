package main

import (
	"fmt"
)

// functions that map virgo data into solr data

/*
func solrBuildParameterQBasic(v VirgoSearchRequest) string {
	switch {
	case v.Query.Title != "":
		return fmt.Sprintf("{!edismax qf=$title_qf pf=$title_pf}(%s)", v.Query.Title)

	case v.Query.Author != "":
		return fmt.Sprintf("{!edismax qf=$author_qf pf=$author_pf}(%s)", v.Query.Author)

	case v.Query.Subject != "":
		return fmt.Sprintf("{!edismax qf=$subject_qf pf=$subject_pf}(%s)", v.Query.Subject)

	default:
		return fmt.Sprintf("{!edismax qf=$qf pf=$pf}(%s)", v.Query.Keyword)
	}
}

func solrBuildParameterQAdvanced(v VirgoSearchRequest) string {
	return "buildme"
}

func solrBuildParameterQ(v VirgoSearchRequest) string {
	// check if this is a specific item search
	if v.Query.Id != "" {
		return fmt.Sprintf("id:%s", v.Query.Id)
	}

	// fall back to basic search
	return solrBuildParameterQBasic(v)
}
*/

func solrBuildParameterQ(v VirgoSearchRequest) string {
	// everything is a keword search for now
	q := fmt.Sprintf("{!edismax qf=$qf pf=$pf}(%s)", v.Query)

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
	qt := "search"

	return qt
}

func solrBuildParameterDefType() string {
	deftype := "lucene"

	return deftype
}

func solrBuildParameterFq() string {
	// leaders must be defined with beginning + or -

	fq := fmt.Sprintf("+shadowed_location_f:VISIBLE %s", pool.leaders)

	return fq
}

func solrBuildParameterFl() string {
	score := "*,score"

	return score
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

func solrSearchRequest(v VirgoSearchRequest) solrRequest {
	solrReq := solrRequestWithDefaults(v)

	return solrReq
}

func solrRecordRequest(id string) solrRequest {
	v := VirgoSearchRequest{}

	solrReq := solrRequestWithDefaults(v)

	// override these values from defaults
	solrReq.params["q"] = fmt.Sprintf("id:%s", id)
	solrReq.params["start"] = "0"
	solrReq.params["rows"] = "2"

	return solrReq
}
