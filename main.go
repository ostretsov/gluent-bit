package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/robertkowalski/graylog-golang"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type managedFiles struct {
	sync.RWMutex
	fileNames map[string]bool
}

func (m *managedFiles) managed(fileName string) bool {
	m.RLock()
	defer m.RUnlock()

	if isManaged, ok := m.fileNames[fileName]; ok {
		return isManaged
	}
	return false
}

func (m *managedFiles) manage(fileName string) {
	m.Lock()
	defer m.Unlock()

	m.fileNames[fileName] = true
}

func (m *managedFiles) abandon(fileName string) {
	m.Lock()
	defer m.Unlock()

	m.fileNames[fileName] = false
}

func main() {
	mFiles := managedFiles{}
	graylogChan := getGraylogChan()
	for {
		logFiles, err := readLogsDir()
		if err != nil {
			log.Fatalln(err)
		}

		for _, logFile := range logFiles {
			if !mFiles.managed(logFile) {
				continue
			}

		}

		time.Sleep(1 * time.Second)
	}
}

func readLogsDir() ([]string, error) {
	dir := getEnvDefault("LOGS_DIR", "/var/log/containers/")
	dirItems, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, dirItem := range dirItems {
		if dirItem.IsDir() {
			continue
		}

		fileName := dirItem.Name()
		if strings.HasSuffix(fileName, ".log") {
			files = append(files, dir+fileName)
		}
	}
	return files, nil
}

func processLogs(fileName string) {

}

func getGraylogChan() chan<- string {
	ch := make(chan<- string, 100)

	go func() {
		graylogHost := getEnv("GRAYLOG_HOST")
		graylogPortStr := getEnv("GRAYLOG_PORT")
		graylogPort, err := strconv.Atoi(graylogPortStr)
		if err != nil || graylogPort <= 0 {
			log.Fatalln("GRAYLOG_PORT must be a positive number")
		}

		graylog := gelf.New(gelf.Config{
			GraylogHostname: graylogHost,
			GraylogPort:     graylogPort,
		})

		for messageToSend := range ch {
			graylog.Log(messageToSend)
		}
	}()

	return ch
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalln(key, "environment variable must be set")
	}
	return value
}

func getEnvDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func parseFileName(fileName string) (podName, podNamespace string) {
	metadata := strings.Split(fileName, "_")
	if len(metadata) < 3 {
		return "", ""
	}

	return metadata[0], metadata[1]
}

type pod struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Labels    struct {
			Logging string `json:"logging"`
		} `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		NodeName string `json:"nodeName"`
	} `json:"spec"`
	Status struct {
		HostIP string `json:"hostIP"`
		PodIP  string `json:"podIP"`
	} `json:"status"`
}

func getPod(name, namespace string) (p *pod, err error) {
	k8sHost := getEnv("KUBERNETES_SERVICE_HOST")
	k8sPort := getEnv("KUBERNETES_SERVICE_PORT")
	caCertFile := getEnvDefault("CA_CERT_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	tokenFile := getEnvDefault("TOKEN_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/token")

	caCertPool := x509.NewCertPool()
	caCertFileContent, err := cachedCat(caCertFile)
	if err != nil {
		return
	}
	caCertPool.AppendCertsFromPEM(caCertFileContent)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	request, err := http.NewRequest("GET", "https://"+k8sHost+":"+k8sPort, nil)
	if err != nil {
		return
	}
	tokenFileContent, err := cachedCat(tokenFile)
	if err != nil {
		return
	}
	request.Header.Add("Authorization", "Bearer: "+string(tokenFileContent))
	response, err := client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	p = &pod{}
	err = json.Unmarshal(body, p)
	if err != nil {
		return
	}

	return
}

type fileCache struct {
	sync.Mutex
	cache map[string][]byte
}

func (f *fileCache) get(fileName string) (content []byte, err error) {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	if content, ok := f.cache[fileName]; ok {
		return content, nil
	}

	content, err = ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	f.cache[fileName] = content
	return
}

var cache = fileCache{}

func cachedCat(fileName string) ([]byte, error) {
	return cache.get(fileName)
}
