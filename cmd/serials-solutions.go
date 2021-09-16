package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type serialsSolutionsHoldingData struct {
	StartDate    string `xml:"startDate"`
	ProviderID   string `xml:"providerId"`
	ProviderName string `xml:"providerName"`
	DatabaseID   string `xml:"databaseId"`
	DatabaseName string `xml:"databaseName"`
}

type serialsSolutionsURL struct {
	URL  string `xml:",chardata"`
	Type string `xml:"type,attr"`
}

type serialsSolutionsLinkGroup struct {
	Type        string                      `xml:"type,attr"`
	HoldingData serialsSolutionsHoldingData `xml:"holdingData"`
	URLs        []serialsSolutionsURL       `xml:"url"`
}

type serialsSolutionsCitation struct {
	Source string `xml:"source"`
}

type serialsSolutionsResult struct {
	Format     string                      `xml:"format,attr"`
	Citation   serialsSolutionsCitation    `xml:"citation"`
	LinkGroups []serialsSolutionsLinkGroup `xml:"linkGroups>linkGroup"`
}

type serialsSolutionsResponse struct {
	Version string                   `xml:"version"`
	Results []serialsSolutionsResult `xml:"results>result"`
}

func (s *searchContext) serialsSolutionsLookup(genre string, serialType string, serials []string) (*serialsSolutionsResponse, error) {
	ctx := s.pool.serialsSolutions

	req, reqErr := http.NewRequest("GET", ctx.url, nil)
	if reqErr != nil {
		s.log("SSAPI: NewRequest() failed: %s", reqErr.Error())
		return nil, fmt.Errorf("failed to create Serials Solutions API request")
	}

	qp := req.URL.Query()

	qp.Add("version", "1.0")
	qp.Add("genre", genre)
	for _, serial := range serials {
		qp.Add(serialType, serial)
	}

	req.URL.RawQuery = qp.Encode()

	if s.client.opts.verbose == true {
		s.log("SSAPI: req: [%s]", req.URL.String())
	}

	start := time.Now()
	res, resErr := ctx.client.Do(req)
	elapsedMS := int64(time.Since(start) / time.Millisecond)

	// external service failure logging (scenario 1)

	if resErr != nil {
		status := http.StatusBadRequest
		errMsg := resErr.Error()
		if strings.Contains(errMsg, "Timeout") {
			status = http.StatusRequestTimeout
			errMsg = fmt.Sprintf("%s timed out", ctx.url)
		} else if strings.Contains(errMsg, "connection refused") {
			status = http.StatusServiceUnavailable
			errMsg = fmt.Sprintf("%s refused connection", ctx.url)
		}

		s.log("SSAPI: client.Do() failed: %s", resErr.Error())
		s.log("ERROR: Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, ctx.url, status, errMsg, elapsedMS)
		return nil, fmt.Errorf("failed to receive Serials Solutions API response")
	}

	defer res.Body.Close()

	var ssRes serialsSolutionsResponse

	decoder := xml.NewDecoder(res.Body)
	//decoder.DefaultSpace = "ssopenurl"

	if decErr := decoder.Decode(&ssRes); decErr != nil {
		s.log("SSAPI: Decode() failed: %s", decErr.Error())
		s.log("ERROR: Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, ctx.url, http.StatusInternalServerError, decErr.Error(), elapsedMS)
		return nil, fmt.Errorf("failed to decode Serials Solutions API response")
	}

	// external service success logging

	s.log("Successful Serials Solutions API response from %s %s. Elapsed Time: %d (ms)", req.Method, ctx.url, elapsedMS)

	return &ssRes, nil
}
