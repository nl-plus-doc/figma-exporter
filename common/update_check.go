package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

const repoURL = "https://api.github.com/repos/nl-plus-doc/figma-exporter/releases/latest"

var unexpectedError = fmt.Errorf("unexpected error")

// CheckUpdate - check release version
func CheckUpdate() {
	latest, err := getLatestVersion()
	if err != nil {
		log.Fatalf("error: %s", err.Error())
	}

	if latest == AppVersion {
		fmt.Println("Already up to date.")
		return
	}

	fmt.Printf("latest: %s\n", latest)
	return
}

func getLatestVersion() (string, error) {
	resp, err := http.Get(repoURL)
	if err != nil {
		return "", fmt.Errorf("check network status")
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", unexpectedError
	}

	var raw map[string]interface{}
	if err = json.Unmarshal(data, &raw); err != nil {
		return "", unexpectedError
	}

	tag, ok := raw["tag_name"]
	if !ok {
		return "", fmt.Errorf("could not get the items")
	}

	version, ok := tag.(string)
	if !ok {
		return "", fmt.Errorf("cast failed")
	}

	return version, nil
}
