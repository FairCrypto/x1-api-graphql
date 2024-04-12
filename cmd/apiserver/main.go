// Package main implements the API server entry point.
package main

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

// init initializes the package and sets some important app-wide options
func init() {
	/*
	   Safety net for 'too many open files' issue on legacy code.
	   Set a sane timeout duration for the http.DefaultClient, to ensure idle connections are terminated.
	   Reference: https://stackoverflow.com/questions/37454236/net-http-server-too-many-open-files-error
	*/
	http.DefaultClient.Timeout = time.Second * 10
}

// main initializes the API server and starts it when ready.
func main() {
	go makeMetricsServer()

	app := apiServer{}
	app.init()
	app.run()
}

func makeMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(":2112", nil)
	if err != nil {
		log.Crit(err.Error())
	}
}
