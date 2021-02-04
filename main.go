package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// ImagesResponse - Figmaのレスポンス
type ImagesResponse struct {
	Images map[string]string `json:"images"`
	Err    interface{}       `json:"err"`
}

func saveImage(url, filename string) {
	response, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	filename = strings.Replace(filename, ":", "-", -1)

	file, err := os.Create(filename + ".jpg")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	client := &http.Client{}
	projectID := os.Getenv("ProjectID")
	nodeID := "0:2"
	url := fmt.Sprintf("https://api.figma.com/v1/images/%s?ids=%s&format=jpg", projectID, nodeID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	figmatoken := os.Getenv("FigmaToken")
	req.Header.Set("X-FIGMA-TOKEN", figmatoken)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var decoded ImagesResponse
	json.Unmarshal([]byte(bodyText), &decoded)
	for k, v := range decoded.Images {
		saveImage(v, k)
	}
}
