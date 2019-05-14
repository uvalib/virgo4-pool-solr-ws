package tests

import (
	"net/http"
	"strings"
	"testing"
)

//
// version tests
//

func TestVersionCheck(t *testing.T) {
	expected := http.StatusOK
	status, version := VersionCheck(cfg.Endpoint)
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
