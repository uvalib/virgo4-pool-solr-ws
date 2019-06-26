package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type poolInfo struct {
	name string // pool type
	desc string // localized description
	url  string // public (service) url
}

// identifying info about the specific type of Solr pool we are
var pool poolInfo

func configurePool() {
	pool.name = config.poolType.value
	pool.desc = config.poolDescription.value
	pool.url = config.poolServiceURL.value
}

func attemptPoolRegistration(jsonReq []byte) error {
	registrationURL := fmt.Sprintf("%s/api/pools/register", config.interpoolSearchURL.value)

	req, reqErr := http.NewRequest("POST", registrationURL, bytes.NewBuffer(jsonReq))
	if reqErr != nil {
		log.Printf("NewRequest() failed: %s", reqErr.Error())
		return errors.New("Failed to create pool registration post request")
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Duration(15) * time.Second}

	res, resErr := client.Do(req)
	if resErr != nil {
		log.Printf("client.Do() failed: %s", resErr.Error())
		return errors.New("Failed to receive pool registration post response")
	}

	if res.StatusCode != 200 {
		log.Printf("Unexpected StatusCode: %d", res.StatusCode)
		return errors.New("Received unexpected pool registration post response status code")
	}

	defer res.Body.Close()

	buf, _ := ioutil.ReadAll(res.Body)

	log.Printf("Pool registration response: [%s]", buf)

	if strings.Contains(string(buf), "registered") == false {
		log.Printf("Unexpected response text: [%s]", buf)
		return errors.New("Received unexpected pool registration post response text")
	}

	return nil
}

func registerPool() {
	log.Printf("Registering pool...")

	req := VirgoPoolRegistration{}

	req.Name = config.poolType.value
	req.URL = config.poolPrivateURL.value

	jsonReq, _ := json.Marshal(req)

	// re-attempt registration every 5 seconds until successful
	for {
		if regErr := attemptPoolRegistration(jsonReq); regErr != nil {
			log.Printf("Pool registration failed: [%s]", regErr.Error())
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	log.Printf("Pool registration succeeded")
}

func poolRegistrationLoop() {
	if strings.Contains(config.interpoolSearchURL.value, "http") == false {
		log.Printf("Pool registration skipped")
		return
	}

	// short delay to allow router to start up, otherwise interpool search might check health before we're ready
	time.Sleep(3 * time.Second)

	for {
		// re-register 5 minutes after every successful registration
		registerPool()
		time.Sleep(30 * time.Second)
	}
}
