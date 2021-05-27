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

	"github.com/go-utils/cont"
	"github.com/joho/godotenv"
	"github.com/nl-plus-doc/figma-exporter/common"
)

const (
	host    = "api.figma.com"
	version = "v1"
)

var extensions = [3]string{"jpg", "png", "svg"}

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

func saveImage(url, pureFileName, saveDir, extension string) {
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

func appendNodes(nodes []FigmaNode, depth int) []FigmaNode {
	if depth == 0 {
		return nodes
	}

	allNodes := make([]FigmaNode, len(nodes))
	copy(allNodes, nodes)

	for _, node := range nodes {
		allNodes = append(allNodes, node.Children...)
	}

	return appendNodes(allNodes, depth-1)
}

func getNodes(projectID, token string, depth int) []FigmaNode {
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

	return appendNodes(decoded.Document.Children, depth)
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

func getUri(projectID string, nodeIDs []string, extension string) string {
	params := url.Values{}
	params.Set("ids", strings.Join(nodeIDs, ","))
	params.Set("format", extension)

	uri := filepath.Join(
		host, version, "images", projectID,
	)
	uri = fmt.Sprintf("https://%s?%s", uri, params.Encode())
	return uri
}

func processRequest(projectID string, token string, nodeIDs []string, extension string) map[string]string {
	uri := getUri(projectID, nodeIDs, extension)
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

func getExportedURLs(projectID string, token string, nodeIDs []string, extension string) map[string]string {

	nodeIdChunks := chunkBy(nodeIDs, 20)
	urlMaps := make([]map[string]string, len(nodeIdChunks))
	wg := new(sync.WaitGroup)
	mu := new(sync.Mutex)

	for i, chunk := range nodeIdChunks {
		chunk := chunk
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := processRequest(projectID, token, chunk, extension)
			mu.Lock()
			urlMaps[i] = result
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
		extension       string
		depth           int
		versionFlag     bool
		updateCheckFlag bool
		formatListFlag  bool
	)

	flag.StringVar(&saveDir, "dir", "", "image directory to search.\nex: `-dir images`")
	flag.StringVar(&extension, "format", "jpg", "Image format to export.\ndefault: jpg\nex: `-format jpg`")
	flag.IntVar(&depth, "depth", 1, "Depth of node to search.\ndefault: 1\nex: `-depth 1`")
	flag.BoolVar(&versionFlag, "v", false, "print version")
	flag.BoolVar(&updateCheckFlag, "update-check", false, "check for updates")
	flag.BoolVar(&formatListFlag, "format-list", false, "image format list")
	flag.Parse()

	if versionFlag {
		fmt.Println(common.AppVersion)
		return
	}

	if updateCheckFlag {
		common.CheckUpdate()
		return
	}

	if formatListFlag {
		fmt.Println("supported format:")
		for _, extension := range extensions {
			fmt.Println(extension)
		}
		return
	}

	if saveDir == "" {
		log.Fatal("please specify a directory. ex: `-dir images`")
	}

	if !cont.Contains(extensions, extension) {
		log.Fatalf("'%s' is unsupported format.", extension)
	}

	if depth < 1 {
		log.Fatal("Please set to 1 or more.")
	}

	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to read .env file: %+v", err)
	}

	projectID := os.Getenv("PROJECT_ID")
	figmaToken := os.Getenv("FIGMA_TOKEN")

	topNodes := getNodes(projectID, figmaToken, depth)

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

	imageURLs := getExportedURLs(projectID, figmaToken, savedNodeIDs, extension)

	wg := new(sync.WaitGroup)
	for _, nodeID := range savedNodeIDs {
		wg.Add(1)
		imageURL := imageURLs[nodeID]
		fileName := nodeIDToNodeNameMap[nodeID]
		go func() {
			defer wg.Done()
			saveImage(imageURL, fileName, saveDir, extension)
		}()
	}
	wg.Wait()
}
