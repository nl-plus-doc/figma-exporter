package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const (
	host      = "api.figma.com"
	version   = "v1"
	extension = "jpg"
)

// ImagesResponse - Figmaのレスポンス
type ImagesResponse struct {
	Images map[string]string `json:"images"`
	Err    interface{}       `json:"err"`
}

func saveImage(url, filename string) {
	response, err := http.Get(url)
	if err != nil {
		log.Fatalf("failed to http get request: %+v", err)
	}
	defer response.Body.Close()

	filename = strings.ReplaceAll(filename, ":", "-")

	file, err := os.Create(fmt.Sprintf("%s.%s", filename, extension))
	if err != nil {
		log.Fatalf("failed to file creation: %+v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, response.Body); err != nil {
		log.Fatalf("failed to io copy: %+v", err)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to read .env file: %+v", err)
	}

	client := new(http.Client)
	projectID := os.Getenv("ProjectID")
	nodeID := "0:2"

	params := url.Values{}
	params.Set("ids", nodeID)
	params.Set("format", extension)
	uri := filepath.Join(
		host, version, "images", projectID,
	)
	uri = fmt.Sprintf("https://%s?%s", uri, params.Encode())

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatal("failed to initialize http instance: %+v", err)
	}
	figmaToken := os.Getenv("FigmaToken")
	req.Header.Set("X-FIGMA-TOKEN", figmaToken)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("failed to http request: %+v", err)
	}
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("failed to io read dir: %+v", err)
	}

	var decoded ImagesResponse
	if err = json.Unmarshal(bodyText, &decoded); err != nil {
		log.Fatal("failed to json unmarshal: %+v", err)
	}

	for k, v := range decoded.Images {
		saveImage(v, k)
	}
}
