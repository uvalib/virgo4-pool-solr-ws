package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type digitalContentURLs struct {
	Delete   string `json:"delete,omitempty"`
	Download string `json:"download,omitempty"`
	Generate string `json:"generate,omitempty"`
	Status   string `json:"status,omitempty"`
}

type digitalContentPDF struct {
	URLs digitalContentURLs `json:"urls,omitempty"`
}

type digitalContentPart struct {
	IIIFManifestURL string            `json:"iiif_manifest_url,omitempty"`
	OembedURL       string            `json:"oembed_url,omitempty"`
	Label           string            `json:"label,omitempty"`
	PID             string            `json:"pid,omitempty"`
	ThumbnailURL    string            `json:"thumbnail_url,omitempty"`
	PDF             digitalContentPDF `json:"pdf,omitempty"`
}

type digitalContentCache struct {
	ID    string               `json:"id,omitempty"`
	Parts []digitalContentPart `json:"parts,omitempty"`
}

func (s *searchContext) getDigitalContentCache(url string) (*digitalContentCache, error) {
	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		s.log("Digital Content Cache: NewRequest() failed: %s", reqErr.Error())
		return nil, fmt.Errorf("failed to create Digital Content Cache status request")
	}

	start := time.Now()
	res, resErr := s.pool.digitalContent.client.Do(req)
	elapsedMS := int64(time.Since(start) / time.Millisecond)

	// external service failure logging

	if resErr != nil {
		status := http.StatusBadRequest
		errMsg := resErr.Error()
		if strings.Contains(errMsg, "Timeout") {
			status = http.StatusRequestTimeout
			errMsg = fmt.Sprintf("%s timed out", url)
		} else if strings.Contains(errMsg, "connection refused") {
			status = http.StatusServiceUnavailable
			errMsg = fmt.Sprintf("%s refused connection", url)
		}

		s.log("Digital Content Cache: client.Do() failed: %s", resErr.Error())
		s.err("Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, url, status, errMsg, elapsedMS)
		return nil, fmt.Errorf("failed to receive Digital Content Cache status response")
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
		errMsg := fmt.Errorf("unexpected status code %d", res.StatusCode)
		s.log("Digital Content Cache: unexpected status code %d", res.StatusCode)
		s.err("Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, url, res.StatusCode, errMsg, elapsedMS)
		return nil, fmt.Errorf("received Digital Content Cache status response code %d", res.StatusCode)
	}

	if res.StatusCode == http.StatusNotFound {
		s.err("Digital Content Cache does not (yet) exist: %s", url)
		return nil, errors.New("digital content cache not found")
	}

	var cache digitalContentCache

	decoder := json.NewDecoder(res.Body)

	// external service failure logging (scenario 2)

	if decErr := decoder.Decode(&cache); decErr != nil {
		s.log("Digital Content Cache: Decode() failed: %s", decErr.Error())
		s.err("Failed response from %s %s - %d:%s. Elapsed Time: %d (ms)", req.Method, url, http.StatusInternalServerError, decErr.Error(), elapsedMS)
		return nil, fmt.Errorf("failed to decode Digital Content Cache response")
	}

	// external service success logging

	s.log("Successful Digital Content Cache response from %s %s. Elapsed Time: %d (ms)", req.Method, url, elapsedMS)

	return &cache, nil
}
