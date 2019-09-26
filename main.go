package main

import (
	"github.com/robertkowalski/graylog-golang"
	"log"
	"os"
	"strconv"
)

func main() {

}

func graylog() chan<- string {
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
