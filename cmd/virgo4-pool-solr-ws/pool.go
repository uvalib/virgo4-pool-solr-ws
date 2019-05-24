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

func registerPool() {
	req := VirgoPoolRegistration{}

	req.Name = program
	req.Url = config.poolServiceUrl.value

	jsonReq, _ := json.Marshal(req)

	// short delay to allow router to start up, otherwise interpool search might check health before we're ready
	time.Sleep(3 * time.Second)

	// loop until registered
	for {
		if regErr := attemptPoolRegistration(jsonReq); regErr != nil {
			log.Printf("Pool registration failed: [%s]", regErr.Error())
			time.Sleep(15 * time.Second)
		} else {
			break
		}
	}

	log.Printf("Pool registration succeeded")
}

func attemptPoolRegistration(jsonReq []byte) error {
	registrationUrl := fmt.Sprintf("%s/api/pools/register", config.interpoolSearchUrl.value)

	req, reqErr := http.NewRequest("POST", registrationUrl, bytes.NewBuffer(jsonReq))
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
