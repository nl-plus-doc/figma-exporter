package main

import (
	"encoding/json"
	"flag"
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

// ImagesResponse - Figma image response
type ImagesResponse struct {
	Images map[string]string `json:"images"`
	Err    interface{}       `json:"err"`
}

// FigmaNode - Figma node
type FigmaNode struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Visible          bool        `json:"visible"`
	Type             string      `json:"type"`
	PluginData       interface{} `json:"pluginData"`
	SharedPluginData interface{} `json:"sharedPluginData"`
	Children         []FigmaNode `json:"children"`
}

// FigmaFilesResponse - Figma file response
type FigmaFilesResponse struct {
	Name          string                 `json:"name"`
	Role          string                 `json:"role"`
	LastModified  string                 `json:"lastModified"`
	ThumbnailURL  string                 `json:"thumbnailUrl"`
	Version       string                 `json:"version"`
	Document      FigmaNode              `json:"document"`
	Components    map[string]interface{} `json:"components"`
	SchemaVersion int64                  `json:"schemaVersion"`
	Styles        map[string]interface{} `json:"styles"`
}

func saveImage(url, pureFileName, saveDir string) {
	response, err := http.Get(url)
	if err != nil {
		log.Fatalf("failed to http get request: %+v", err)
	}
	defer response.Body.Close()

	fileName := fmt.Sprintf("%s.%s", pureFileName, extension)

	file, err := os.Create(filepath.Join(saveDir, fileName))
	if err != nil {
		log.Fatalf("failed to file creation: %+v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, response.Body); err != nil {
		log.Fatalf("failed to io copy: %+v", err)
	}
}

func getTopNodes(projectID, token string) []FigmaNode {
	uri := filepath.Join(
		host, version, "files", projectID,
	)
	uri = "https://" + uri

	client := new(http.Client)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatalf("failed to initialize http instance: %+v", err)
	}
	req.Header.Set("X-FIGMA-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to http request: %+v", err)
	}
	defer resp.Body.Close()

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to io read dir: %+v", err)
	}

	var decoded FigmaFilesResponse
	if err = json.Unmarshal(bodyText, &decoded); err != nil {
		log.Fatalf("failed to json unmarshal: %+v", err)
	}

	topNodes := make([]FigmaNode, 0)
	for _, canvas := range decoded.Document.Children {
		for _, topNode := range canvas.Children {
			topNodes = append(topNodes, topNode)
		}
	}
	return topNodes
}

func getExportedURLs(projectID string, token string, nodeIDs []string) map[string]string {
	params := url.Values{}
	params.Set("ids", strings.Join(nodeIDs, ","))
	params.Set("format", extension)

	uri := filepath.Join(
		host, version, "images", projectID,
	)
	uri = fmt.Sprintf("https://%s?%s", uri, params.Encode())

	client := new(http.Client)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatalf("failed to initialize http instance: %+v", err)
	}
	req.Header.Set("X-FIGMA-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to http request: %+v", err)
	}
	defer resp.Body.Close()

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to io read dir: %+v", err)
	}

	var decoded ImagesResponse
	if err = json.Unmarshal(bodyText, &decoded); err != nil {
		log.Fatalf("failed to json unmarshal: %+v", err)
	}

	return decoded.Images
}

func main() {
	var saveDir string
	flag.StringVar(&saveDir, "dir", "", "image directory to search. ex: `-dir images`")
	flag.Parse()

	if saveDir == "" {
		log.Fatal("please specify a directory. ex: `-dir images`")
	}

	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to read .env file: %+v", err)
	}

	projectID := os.Getenv("ProjectID")
	figmaToken := os.Getenv("FigmaToken")

	topNodes := getTopNodes(projectID, figmaToken)

	nodeNameToNodeIDMap := make(map[string]string)
	nodeIDToNodeNameMap := make(map[string]string) // nodeID: frameName
	nodeIDs := make([]string, len(topNodes))
	for i, node := range topNodes {
		nodeNameToNodeIDMap[node.Name] = node.ID
		nodeIDToNodeNameMap[node.ID] = node.Name
		nodeIDs[i] = node.ID
	}

	fifos, err := ioutil.ReadDir(saveDir)
	if err != nil {
		log.Fatalf("failed to io read dir: %+v", err)
	}

	imageURLs := getExportedURLs(projectID, figmaToken, nodeIDs)

	for _, fifo := range fifos {
		if fifo.IsDir() {
			continue
		}
		splitFileName := strings.Split(fifo.Name(), ".")
		pureFileName := strings.Join(splitFileName[:len(splitFileName)-1], ".")

		if nodeID, ok := nodeNameToNodeIDMap[pureFileName]; ok {
			imageURL := imageURLs[nodeID]
			saveImage(imageURL, pureFileName, saveDir)
		}
	}
}
