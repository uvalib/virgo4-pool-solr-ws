package tests

import (
	"encoding/json"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

var debugHTTP = false
var serviceTimeout = 5

//
// HealthCheck -- calls the service health check method
//
func HealthCheck(endpoint string) int {

	url := fmt.Sprintf("%s/healthcheck", endpoint)
	//fmt.Printf( "%s\n", url )

	resp, _, errs := gorequest.New().
		SetDebug(debugHTTP).
		Get(url).
		Timeout(time.Duration(serviceTimeout) * time.Second).
		End()

	if errs != nil {
		return http.StatusInternalServerError
	}

	defer io.Copy(ioutil.Discard, resp.Body)
	defer resp.Body.Close()

	return resp.StatusCode
}

//
// VersionCheck -- calls the service version check method
//
func VersionCheck(endpoint string) (int, string) {

	url := fmt.Sprintf("%s/version", endpoint)
	//fmt.Printf( "%s\n", url )

	resp, body, errs := gorequest.New().
		SetDebug(debugHTTP).
		Get(url).
		Timeout(time.Duration(serviceTimeout) * time.Second).
		End()

	if errs != nil {
		return http.StatusInternalServerError, ""
	}

	defer io.Copy(ioutil.Discard, resp.Body)
	defer resp.Body.Close()

	r := VersionResponse{}
	err := json.Unmarshal([]byte(body), &r)
	if err != nil {
		return http.StatusInternalServerError, ""
	}

	return resp.StatusCode, r.Version
}

//
// MetricsCheck -- calls the service metrics method
//
func MetricsCheck(endpoint string) (int, string) {

	url := fmt.Sprintf("%s/metrics", endpoint)
	//fmt.Printf( "%s\n", url )

	resp, body, errs := gorequest.New().
		SetDebug(debugHTTP).
		Get(url).
		Timeout(time.Duration(serviceTimeout) * time.Second).
		End()

	if errs != nil {
		return http.StatusInternalServerError, ""
	}

	defer io.Copy(ioutil.Discard, resp.Body)
	defer resp.Body.Close()

	return resp.StatusCode, body
}

//
// end of file
//