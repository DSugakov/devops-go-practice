package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	serverURL       = "http://srv.msk01.gigacorp.local/_stats"
	maxRetryCount   = 3
	httpTimeout     = 5 * time.Second
	requestInterval = 500 * time.Millisecond

	expectedMetricsLength = 7

	cpuLoadThreshold      = 30
	memoryUsageThreshold  = 80
	diskUsageThreshold    = 90
	networkUsageThreshold = 90

	bytesInMegabyte = 1024 * 1024
	bytesInMegabit  = 1000 * 1000
	fullPercent     = 100
)

type Metric struct {
	capacity   int
	usage      int
	threshold  int
	message    string
	unit       string
	checkUsage func(capacity, usage int) (int, int)
}

func main() {
	resultStream := startPolling(serverURL, maxRetryCount)

	for response := range resultStream() {
		metrics, err := parseMetrics(response)
		if err != nil {
			continue
		}

		metricList := []Metric{
			{
				capacity:   metrics.CPULoad,
				usage:      metrics.CPULoad,
				threshold:  cpuLoadThreshold,
				message:    "Load Average is too high: %d\n",
				unit:       "",
				checkUsage: getDirectUsage,
			},
			{
				capacity:   metrics.MemoryCapacity,
				usage:      metrics.MemoryUsage,
				threshold:  memoryUsageThreshold,
				message:    "Memory usage too high: %d%%\n",
				unit:       "%",
				checkUsage: getPercentageUsage,
			},
			{
				capacity:   metrics.DiskCapacity,
				usage:      metrics.DiskUsage,
				threshold:  diskUsageThreshold,
				message:    "Free disk space is too low: %d Mb left\n",
				unit:       "Mb",
				checkUsage: getFreeDiskSpace,
			},
			{
				capacity:   metrics.NetworkCapacity,
				usage:      metrics.NetworkActivity,
				threshold:  networkUsageThreshold,
				message:    "Network bandwidth usage high: %d Mbit/s available\n",
				unit:       "Mbit/s",
				checkUsage: getFreeNetworkBandwidth,
			},
		}

		for _, metric := range metricList {
			validateResourceUsage(metric)
		}
	}
}

func validateResourceUsage(m Metric) {
	usagePercent, freeResource := m.checkUsage(m.capacity, m.usage)

	if usagePercent > m.threshold {
		if m.unit == "%" || m.unit == "" {
			fmt.Printf(m.message, usagePercent)
		} else {
			fmt.Printf(m.message, freeResource)
		}
	}
}

func getDirectUsage(capacity, _ int) (int, int) {
	usagePercent := capacity
	return usagePercent, usagePercent
}

func getPercentageUsage(capacity, usage int) (int, int) {
	usagePercent := usage * fullPercent / capacity
	return usagePercent, usagePercent
}

func getFreeDiskSpace(capacity, usage int) (int, int) {
	usagePercent := usage * fullPercent / capacity
	freeResource := (capacity - usage) / bytesInMegabyte
	return usagePercent, freeResource
}

func getFreeNetworkBandwidth(capacity, usage int) (int, int) {
	usagePercent := usage * fullPercent / capacity
	freeResource := (capacity - usage) / bytesInMegabit
	return usagePercent, freeResource
}

func startPolling(url string, retries int) func() chan string {
	return func() chan string {
		dataChannel := make(chan string, 3)
		client := http.Client{Timeout: httpTimeout}
		errorCounter := 0

		go func() {
			defer close(dataChannel)

			for {
				time.Sleep(requestInterval)

				if errorCounter >= retries {
					fmt.Println("Unable to fetch server statistics")
					break
				}

				response, err := client.Get(url)
				errorCounter = processResponseError(response, err, errorCounter)
				if errorCounter > 0 {
					continue
				}

				body, err := io.ReadAll(response.Body)
				if err != nil {
					errorCounter = incrementErrorCount(err, errorCounter, "failed to parse response")
					continue
				}

				response.Body.Close()
				dataChannel <- string(body)

				errorCounter = 0
			}
		}()

		return dataChannel
	}
}

func processResponseError(response *http.Response, err error, errorCounter int) int {
	if err != nil {
		return incrementErrorCount(err, errorCounter, "failed to send request")
	}
	if response.StatusCode != http.StatusOK {
		return incrementErrorCount(fmt.Errorf("invalid status code: %d", response.StatusCode), errorCounter, "")
	}
	return errorCounter
}

func incrementErrorCount(err error, errorCounter int, message string) int {
	if message != "" {
		fmt.Printf("%s: %s\n", message, err)
	}
	return errorCounter + 1
}

type ServerMetrics struct {
	CPULoad         int
	MemoryCapacity  int
	MemoryUsage     int
	DiskCapacity    int
	DiskUsage       int
	NetworkCapacity int
	NetworkActivity int
}

func parseMetrics(data string) (ServerMetrics, error) {
	parts := strings.Split(data, ",")
	if len(parts) != expectedMetricsLength {
		return ServerMetrics{}, fmt.Errorf("invalid data format")
	}

	values := make([]int, expectedMetricsLength)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return ServerMetrics{}, fmt.Errorf("invalid number: %s", part)
		}
		values[i] = value
	}

	return ServerMetrics{
		CPULoad:         values[0],
		MemoryCapacity:  values[1],
		MemoryUsage:     values[2],
		DiskCapacity:    values[3],
		DiskUsage:       values[4],
		NetworkCapacity: values[5],
		NetworkActivity: values[6],
	}, nil
}
