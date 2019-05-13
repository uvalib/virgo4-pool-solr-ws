package tests

import (
	"github.com/uvalib/virgo4-pool-solr-ws/cmd/client"
	"net/http"
	"strings"
	"testing"
)

//
// version tests
//

func TestVersionCheck(t *testing.T) {
	expected := http.StatusOK
	status, version := client.VersionCheck(cfg.Endpoint)
	if status != expected {
		t.Fatalf("Expected %v, got %v\n", expected, status)
	}

	if len(version) == 0 {
		t.Fatalf("Expected non-zero length version string\n")
	}

	if strings.Contains(version, "build-") == false {
		t.Fatalf("Expected \"build-nnn\" value in version info\n")
	}
}

//
// end of file
//
