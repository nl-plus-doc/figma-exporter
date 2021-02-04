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

// FigmaNode - FigmaのNode
type FigmaNode struct {
	Id               string      `json:"id"`
	Name             string      `json:"name"`
	Visible          bool        `json:"visible"`
	Type             string      `json:"type"`
	PluginData       interface{} `json:"pluginData"`
	SharedPluginData interface{} `json:"sharedPluginData"`
	Children         []FigmaNode `json:"children"`
}

// FigmaFilesResponse - FigmaのFileのレスポンス
type FigmaFilesResponse struct {
	Name          string                 `json:"name"`
	Role          string                 `json:"role"`
	LastModified  string                 `json:"lastModified"`
	ThumbnailUrl  string                 `json:"thumbnailUrl"`
	Version       string                 `json:"version"`
	Document      FigmaNode              `json:"document"`
	Components    map[string]interface{} `json:"components"`
	SchemaVersion int64                  `json:"schemaVersion"`
	Styles        map[string]interface{} `json:"styles"`
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

func getTopNodes(projectId, token string) []FigmaNode {
	uri := filepath.Join(
		host, version, "files", projectId,
	)
	uri = "https://" + uri

	client := new(http.Client)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatal("failed to initialize http instance: %+v", err)
	}
	req.Header.Set("X-FIGMA-TOKEN", token)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("failed to http request: %+v", err)
	}
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("failed to io read dir: %+v", err)
	}
	var decoded FigmaFilesResponse
	if err = json.Unmarshal(bodyText, &decoded); err != nil {
		log.Fatal("failed to json unmarshal: %+v", err)
	}

	topNodes := make([]FigmaNode, 0)
	for _, canvas := range decoded.Document.Children {
		for _, topNode := range canvas.Children {
			topNodes = append(topNodes, topNode)
		}
	}
	return topNodes
}

func getExportedUrls(projectId string, token string, nodeIds []string) map[string]string {
	params := url.Values{}
	params.Set("ids", strings.Join(nodeIds, ","))
	params.Set("format", extension)
	uri := filepath.Join(
		host, version, "images", projectId,
	)
	uri = fmt.Sprintf("https://%s?%s", uri, params.Encode())

	client := new(http.Client)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatal("failed to initialize http instance: %+v", err)
	}
	req.Header.Set("X-FIGMA-TOKEN", token)
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

	return decoded.Images
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to read .env file: %+v", err)
	}

	projectID := os.Getenv("ProjectID")
	figmaToken := os.Getenv("FigmaToken")

	topNodes := getTopNodes(projectID, figmaToken)

	nodeMap := make(map[string]string)  // frameName: nodeID
	frameMap := make(map[string]string) // nodeID: frameName
	nodeIDs := make([]string, 0)
	for _, node := range topNodes {
		nodeMap[node.Name] = node.Id
		frameMap[node.Id] = node.Name
		nodeIDs = append(nodeIDs, node.Id)
	}

	fifos, err := ioutil.ReadDir("images")
	if err != nil {
		log.Fatal("failed to io read dir: %+v", err)
	}

	for _, fifo := range fifos {
		if fifo.IsDir() {
			continue
		}
		if _, ok := frameMap[fifo.Name()]; ok {
			// processing
		}
	}

	imageUrls := getExportedUrls(projectID, figmaToken, nodeIDs)

	for nodeId, imageUrl := range imageUrls {
		fmt.Println(frameMap[nodeId])
		fmt.Println(imageUrl)
	}
}
