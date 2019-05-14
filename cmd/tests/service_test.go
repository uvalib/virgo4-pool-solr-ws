package tests

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type testConfig struct {
	Endpoint string
}

var cfg = loadConfig()

func emptyFields(fields []string) bool {

	for _, field := range fields {
		if emptyField(field) == true {
			return true
		}
	}
	return false
}

func emptyField(field string) bool {
	return len(strings.TrimSpace(field)) == 0
}

func loadConfig() testConfig {

	data, err := ioutil.ReadFile("service_test.yml")
	if err != nil {
		log.Fatal(err)
	}

	var c testConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		log.Fatal(err)
	}

	// allow environment variables to override the configuration file
	if len( os.Getenv( "TC_ENDPOINT" ) ) != 0 {
		c.Endpoint = os.Getenv( "TC_ENDPOINT" )
	}

	log.Printf("endpoint [%s]\n", c.Endpoint)

	return c
}

//
// end of file
//
