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
	Capacity   int
	Usage      int
	Threshold  int
	Message    string
	Unit       string
	CheckUsage func(int, int) (int, int)
}

func main() {
	metricsStream := startPolling(serverURL, maxRetryCount)

	for response := range metricsStream() {
		metrics, err := parseMetrics(response)
		if err != nil {
			continue
		}

		metricList := []Metric{
			newMetric(metrics.CPULoad, metrics.CPULoad, cpuLoadThreshold, "Load Average is too high: %d\n", "", getDirectUsage),
			newMetric(metrics.MemoryCapacity, metrics.MemoryUsage, memoryUsageThreshold, "Memory usage too high: %d%%\n", "%", getPercentageUsage),
			newMetric(metrics.DiskCapacity, metrics.DiskUsage, diskUsageThreshold, "Free disk space is too low: %d Mb left\n", "Mb", getFreeDiskSpace),
			newMetric(metrics.NetworkCapacity, metrics.NetworkActivity, networkUsageThreshold, "Network bandwidth usage high: %d Mbit/s available\n", "Mbit/s", getFreeNetworkBandwidth),
		}

		for _, metric := range metricList {
			validateResourceUsage(metric)
		}
	}
}

func newMetric(capacity, usage, threshold int, message, unit string, checkUsage func(int, int) (int, int)) Metric {
	return Metric{Capacity: capacity, Usage: usage, Threshold: threshold, Message: message, Unit: unit, CheckUsage: checkUsage}
}

func validateResourceUsage(m Metric) {
	usagePercent, freeResource := m.CheckUsage(m.Capacity, m.Usage)

	if usagePercent > m.Threshold {
		if m.Unit == "%" || m.Unit == "" {
			fmt.Printf(m.Message, usagePercent)
		} else {
			fmt.Printf(m.Message, freeResource)
		}
	}
}

func getDirectUsage(capacity, _ int) (int, int) {
	return capacity, capacity
}

func getPercentageUsage(capacity, usage int) (int, int) {
	if capacity == 0 {
		return 0, 0
	}
	return usage * fullPercent / capacity, usage * fullPercent / capacity
}

func getFreeDiskSpace(capacity, usage int) (int, int) {
	if capacity == 0 {
		return 0, 0
	}
	freeResource := (capacity - usage) / bytesInMegabyte
	return usage * fullPercent / capacity, freeResource
}

func getFreeNetworkBandwidth(capacity, usage int) (int, int) {
	if capacity == 0 {
		return 0, 0
	}
	freeResource := (capacity - usage) / bytesInMegabit
	return usage * fullPercent / capacity, freeResource
}

func startPolling(url string, retries int) func() chan string {
	return func() chan string {
		dataChannel := make(chan string)
		client := http.Client{Timeout: httpTimeout}

		go func() {
			defer close(dataChannel)

			errorCounter := 0

			for {
				time.Sleep(requestInterval)

				if errorCounter >= retries {
					fmt.Println("Unable to fetch server statistics")
					break
				}

				response, err := client.Get(url)
				if err != nil || response.StatusCode != http.StatusOK {
					errorCounter++
					fmt.Printf("Request error: %v\n", err)
					continue
				}

				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					errorCounter++
					fmt.Printf("Error reading response: %v\n", err)
					continue
				}

				dataChannel <- string(body)
				errorCounter = 0
			}
		}()

		return dataChannel
	}
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
