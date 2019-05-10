package tests

import (
	"github.com/uvalib/virgo4-pool-solr-ws/cmd/client"
	"net/http"
	"testing"
)

//
// healthcheck tests
//

func TestHealthCheck(t *testing.T) {
	expected := http.StatusOK
	status := client.HealthCheck(cfg.Endpoint)
	if status != expected {
		t.Fatalf("Expected %v, got %v\n", expected, status)
	}
}

//
// end of file
//
