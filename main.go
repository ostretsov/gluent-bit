package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/hpcloud/tail"
	gelf "github.com/robertkowalski/graylog-golang"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type pod struct {
	Metadata struct {
		Annotations struct {
			Logging string `json:"logging"`
		} `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		NodeName string `json:"nodeName"`
	} `json:"spec"`
}

type dockerLogEntry struct {
	Log  string    `json:"log"`
	Time time.Time `json:"time"`
}

func main() {
	// make sure required vars are set
	getEnv("KUBERNETES_SERVICE_HOST")
	getEnv("KUBERNETES_SERVICE_PORT")
	getEnv("GRAYLOG_HOST")
	getEnv("GRAYLOG_PORT")

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

	log.Println("processing log file", logFile)

	defer func() {
		if err := recover(); err != nil {
			log.Println("recovering from panic while processing log file", logFile, err)
			time.Sleep(1 * time.Second)
			go processLogs(logFile)
		}
	}()

	podName, podNamespace := parseFileName(logFile)
	if podName == "" || podNamespace == "" {
		log.Println(logFile, "doesn't seem like a kubernetes docker log file: couldn't determine pod name and namespace")
		return
	}

	pod, err := getPod(podName, podNamespace)
	if err != nil {
		panic(err)
	}
	if pod.Metadata.Annotations.Logging != "enabled" {
		log.Println("logging annotation was not enabled for pod", podName, podNamespace)
		return
	}

	t, err := tail.TailFile(logFile, tail.Config{Follow: true, MustExist: true, ReOpen: true})
	if err != nil {
		log.Println("tail is impossible", logFile, err)
	}
	graylogCh := getGraylogChan()
	for line := range t.Lines {
		entry := &dockerLogEntry{}
		err := json.Unmarshal([]byte(line.Text), entry)
		if err != nil {
			log.Println("err on parsing docker json log line", err)
			continue
		}

		message := map[string]string{
			"version":       "1.1",
			"host":          pod.Spec.NodeName,
			"short_message": entry.Log,
			"timestamp":     fmt.Sprintf("%.4f", float64(entry.Time.UnixNano())/float64(time.Second.Nanoseconds())),
		}
		encodedMessage, err := json.Marshal(message)
		if err != nil {
			log.Println("err on preparing message for graylog", err, entry.Log)
			continue
		}

		graylogCh <- string(encodedMessage)
	}
	err = t.Err()
	if err != nil {
		panic(err)
	}
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

func getGraylogChan() chan<- string {
	ch := make(chan string, 100)

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

	go func() {
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

func getPod(name, namespace string) (p *pod, err error) {
	k8sHost := getEnv("KUBERNETES_SERVICE_HOST")
	k8sPort := getEnv("KUBERNETES_SERVICE_PORT")
	caCertFile := getEnvDefault("CA_CERT_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	tokenFile := getEnvDefault("TOKEN_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/token")

	caCertPool := x509.NewCertPool()
	caCertFileContent, err := ioutil.ReadFile(caCertFile)
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
	tokenFileContent, err := ioutil.ReadFile(tokenFile)
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
		log.Println("response body", string(body))
		return
	}

	return
}
