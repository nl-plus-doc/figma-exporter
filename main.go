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
	"sync"

	"github.com/joho/godotenv"
	"github.com/nl-plus-doc/figma-exporter/common"
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
		topNodes = append(topNodes, canvas.Children...)
	}
	return topNodes
}

func mergeMap(maps []map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func chunkBy(items []string, chunkSize int) (chunks [][]string) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}

	return append(chunks, items)
}

func getUri(projectID string, nodeIDs []string) string {
	params := url.Values{}
	params.Set("ids", strings.Join(nodeIDs, ","))
	params.Set("format", extension)

	uri := filepath.Join(
		host, version, "images", projectID,
	)
	uri = fmt.Sprintf("https://%s?%s", uri, params.Encode())
	return uri
}

func processRequest(projectID string, token string, nodeIDs []string) map[string]string {
	uri := getUri(projectID, nodeIDs)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatalf("failed to initialize http instance: %+v", err)
	}
	req.Header.Set("X-FIGMA-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
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

func getExportedURLs(projectID string, token string, nodeIDs []string) map[string]string {

	nodeIdChunks := chunkBy(nodeIDs, 20)
	urlMaps := make([]map[string]string, len(nodeIdChunks))
	wg := new(sync.WaitGroup)
	mu := new(sync.Mutex)

	for _, chunk := range nodeIdChunks {
		chunk := chunk
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := processRequest(projectID, token, chunk)
			mu.Lock()
			urlMaps = append(urlMaps, result)
			mu.Unlock()
		}()
	}

	wg.Wait()

	mergedUrlMap := mergeMap(urlMaps)

	return mergedUrlMap
}

func main() {
	var (
		saveDir         string
		versionFlag     bool
		updateCheckFlag bool
	)

	flag.StringVar(&saveDir, "dir", "", "image directory to search. ex: `-dir images`")
	flag.BoolVar(&versionFlag, "v", false, "print version")
	flag.BoolVar(&updateCheckFlag, "update-check", false, "check for updates")
	flag.Parse()

	if versionFlag {
		fmt.Println(common.AppVersion)
		return
	}

	if updateCheckFlag {
		common.CheckUpdate()
		return
	}

	if saveDir == "" {
		log.Fatal("please specify a directory. ex: `-dir images`")
	}

	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to read .env file: %+v", err)
	}

	projectID := os.Getenv("PROJECT_ID")
	figmaToken := os.Getenv("FIGMA_TOKEN")

	topNodes := getTopNodes(projectID, figmaToken)

	nodeNameToNodeIDMap := make(map[string]string)
	nodeIDToNodeNameMap := make(map[string]string) // nodeID: frameName
	for _, node := range topNodes {
		nodeNameToNodeIDMap[node.Name] = node.ID
		nodeIDToNodeNameMap[node.ID] = node.Name
	}

	fifos, err := ioutil.ReadDir(saveDir)
	if err != nil {
		log.Fatalf("failed to io read dir: %+v", err)
	}

	savedNodeIDs := make([]string, 0)
	for _, fifo := range fifos {
		if fifo.IsDir() {
			continue
		}
		splitFileName := strings.Split(fifo.Name(), ".")
		pureFileName := strings.Join(splitFileName[:len(splitFileName)-1], ".")
		if nodeID, ok := nodeNameToNodeIDMap[pureFileName]; ok {
			savedNodeIDs = append(savedNodeIDs, nodeID)
		}
	}

	imageURLs := getExportedURLs(projectID, figmaToken, savedNodeIDs)

	wg := new(sync.WaitGroup)
	for _, nodeID := range savedNodeIDs {
		wg.Add(1)
		imageURL := imageURLs[nodeID]
		fileName := nodeIDToNodeNameMap[nodeID]
		go func() {
			defer wg.Done()
			saveImage(imageURL, fileName, saveDir)
		}()
	}
	wg.Wait()
}
