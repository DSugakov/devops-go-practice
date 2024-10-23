package main

import (
	"devops-go-practice/alerts"
	"devops-go-practice/metrics"
	"devops-go-practice/polling"
	"fmt"
)

const serverURL = "http://srv.msk01.gigacorp.local/_stats"
const maxRetryCount = 3

func main() {
	pollServer := polling.InitiatePolling(serverURL, maxRetryCount)

	for response := range pollServer() {
		serverMetrics, err := metrics.ParseMetrics(response)
		if err != nil {
			fmt.Printf("Error parsing metrics: %v\n", err)
			continue
		}

		alerts.CheckMetrics(serverMetrics)
	}
}
