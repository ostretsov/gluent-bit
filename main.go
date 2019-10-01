package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/fsnotify/fsnotify"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

/**
1. read all files in logs dir
2. launch log parser for each file in the dir. recover from panic if it happens.
	log parser asks k8s is it a cluster resource.
3. inotify on file creation in the logs dir
*/
func main() {
	targetLogsDir := getEnvDefault("K8S_CONTAINERS_LOGS_DIR", "/var/log/containers/")
	if !strings.HasSuffix(targetLogsDir, "/") {
		targetLogsDir = targetLogsDir + "/"
	}

	logFiles, err := getLogFiles(targetLogsDir)
	if err != nil {
		log.Fatalln("error reading from logs dir", err)
	}

	for _, logFile := range logFiles {
		go processLogs(logFile)
	}

	// watch for new files in logs dir
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalln(err)
	}
	defer watcher.Close()
	err = watcher.Add(targetLogsDir)
	if err != nil {
		log.Fatalln(err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Fatalln("inotify is aborted (events chan)")
			}

			if event.Op == fsnotify.Create {
				go processLogs(event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				log.Fatalln("inotify is aborted (errors chan)")
			}
			log.Fatalln(err)
		}
	}
}

func processLogs(logFile string) {
	fileInfo, err := os.Stat(logFile)
	if err != nil {
		log.Println("err on getting log file stat:", err)
		return
	}

	if fileInfo.IsDir() {
		log.Println(logFile, "is a directory")
		return
	}

	if fileInfo.Mode()&0004 == 0 {
		log.Println(logFile, "doesn't have read permission for others")
		return
	}

	log.Println("processing log file", logFile)

	defer func() {
		if err := recover(); err != nil {
			log.Println("recovering from panic while processing log file", logFile, err)
			go processLogs(logFile)
		}
	}()

	// parse pod ns & name
	// ask k8s if the pod has logging annotation
	// tail logs
	// abandon if tail is no more possible
}

func getLogFiles(targetLogsDir string) ([]string, error) {
	dirItems, err := ioutil.ReadDir(targetLogsDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, dirItem := range dirItems {
		files = append(files, targetLogsDir+dirItem.Name())
	}
	return files, nil
}

type dockerLogEntry struct {
	Log    string    `json:"log"`
	Stream string    `json:"stream"`
	Time   time.Time `json:"time"`
}

//func processFileLogs(fileName string) {
//	name, namespace := parseFileName(fileName)
//	var p *pod
//	if name != "" && namespace != "" {
//		p, _ = getPod(name, namespace)
//	}
//
//	t, err := tail.TailFile(fileName, tail.Config{Follow: true})
//	if err != nil {
//		// todo abandon using channel
//	}
//	for line := range t.Lines {
//		entry := &dockerLogEntry{}
//		err := json.Unmarshal([]byte(line.Text), entry)
//		if err != nil {
//			continue
//		}
//	}
//}
//
//func getGraylogChan() chan<- string {
//	ch := make(chan<- string, 100)
//
//	go func() {
//		graylogHost := getEnv("GRAYLOG_HOST")
//		graylogPortStr := getEnv("GRAYLOG_PORT")
//		graylogPort, err := strconv.Atoi(graylogPortStr)
//		if err != nil || graylogPort <= 0 {
//			log.Fatalln("GRAYLOG_PORT must be a positive number")
//		}
//
//		graylog := gelf.New(gelf.Config{
//			GraylogHostname: graylogHost,
//			GraylogPort:     graylogPort,
//		})
//
//		for messageToSend := range ch {
//			graylog.Log(messageToSend)
//		}
//	}()
//
//	return ch
//}

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
		Annotations struct {
			Logging string `json:"logging"`
		} `json:"annotations"`
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
