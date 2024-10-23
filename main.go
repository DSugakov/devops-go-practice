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
)

type Metric struct {
	threshold  int
	message    string
	unit       string
	checkUsage func() (int, int)
}

func main() {
	resultStream := startPolling(serverURL, maxRetryCount)

	for response := range resultStream() {
		metrics, err := parseMetrics(response)
		if err != nil {
			fmt.Println("Error parsing metrics:", err)
			continue
		}

		metricList := []Metric{
			newMetric(metrics.CPULoad, cpuLoadThreshold, "Load Average is too high: %d\n", ""),
			newMetricPercentage(metrics.MemoryUsage, metrics.MemoryCapacity, memoryUsageThreshold, "Memory usage too high: %d%%\n"),
			newMetricFreeResource(metrics.DiskUsage, metrics.DiskCapacity, diskUsageThreshold, "Free disk space is too low: %d Mb left\n", 1024*1024),
			newMetricFreeResource(metrics.NetworkActivity, metrics.NetworkCapacity, networkUsageThreshold, "Network bandwidth usage high: %d Mbit/s available\n", 1000*1000),
		}

		for _, metric := range metricList {
			validateResourceUsage(metric)
		}
	}
}

func newMetric(value, threshold int, message, unit string) Metric {
	return Metric{
		threshold:  threshold,
		message:    message,
		unit:       unit,
		checkUsage: func() (int, int) { return value, value },
	}
}

func newMetricPercentage(usage, capacity, threshold int, message string) Metric {
	return Metric{
		threshold:  threshold,
		message:    message,
		unit:       "%",
		checkUsage: func() (int, int) { return usage * 100 / capacity, 0 },
	}
}

func newMetricFreeResource(usage, capacity, threshold int, message string, unitInBytes int) Metric {
	return Metric{
		threshold:  threshold,
		message:    message,
		unit:       "Mb",
		checkUsage: func() (int, int) { return usage * 100 / capacity, (capacity - usage) / unitInBytes },
	}
}

func validateResourceUsage(m Metric) {
	usagePercent, freeResource := m.checkUsage()

	if usagePercent > m.threshold {
		if m.unit == "%" || m.unit == "" {
			fmt.Printf(m.message, usagePercent)
		} else {
			fmt.Printf(m.message, freeResource)
		}
	}
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
				if err != nil || response.StatusCode != http.StatusOK {
					errorCounter++
					fmt.Println("Error fetching server data:", err)
					continue
				}

				body, err := io.ReadAll(response.Body)
				if err != nil {
					errorCounter++
					fmt.Println("Failed to parse response:", err)
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
