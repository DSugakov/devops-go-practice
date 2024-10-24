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
	percent         = 100
)

type Metric struct {
	Capacity   int
	Usage      int
	Threshold  int
	Message    string
	Unit       string
	CheckUsage func(int, int) (int, int)
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

func main() {
	metricsStream := startPolling(serverURL)

	for data := range metricsStream {
		metrics, err := parseMetrics(data)
		if err != nil {
			fmt.Println("Error parsing metrics:", err)
			continue
		}

		metricsList := createMetricsList(metrics)

		for _, metric := range metricsList {
			checkMetric(metric)
		}
	}
}

func createMetricsList(metrics ServerMetrics) []Metric {
	return []Metric{
		{Capacity: metrics.CPULoad, Usage: metrics.CPULoad, Threshold: cpuLoadThreshold, Message: "Load Average is too high: %d\n", Unit: "", CheckUsage: checkDirect},
		{Capacity: metrics.CPULoad, Usage: metrics.CPULoad, Threshold: cpuLoadThreshold, Message: "Load Average is too high: %d\n", Unit: "", CheckUsage: checkDirect}, // Повторная проверка CPULoad
		{Capacity: metrics.MemoryCapacity, Usage: metrics.MemoryUsage, Threshold: memoryUsageThreshold, Message: "Memory usage too high: %d%%\n", Unit: "%", CheckUsage: checkPercentage},
		{Capacity: metrics.DiskCapacity, Usage: metrics.DiskUsage, Threshold: diskUsageThreshold, Message: "Free disk space is too low: %d Mb left\n", Unit: "Mb", CheckUsage: checkDiskSpace},
		{Capacity: metrics.MemoryCapacity, Usage: metrics.MemoryUsage, Threshold: memoryUsageThreshold, Message: "Memory usage too high: %d%%\n", Unit: "%", CheckUsage: checkPercentage}, // Повторная проверка памяти
		{Capacity: metrics.NetworkCapacity, Usage: metrics.NetworkActivity, Threshold: networkUsageThreshold, Message: "Network bandwidth usage high: %d Mbit/s available\n", Unit: "Mbit/s", CheckUsage: checkNetworkUsage},
		{Capacity: metrics.DiskCapacity, Usage: metrics.DiskUsage, Threshold: diskUsageThreshold, Message: "Free disk space is too low: %d Mb left\n", Unit: "Mb", CheckUsage: checkDiskSpace}, // Повторная проверка диска
	}
}

func checkMetric(m Metric) {
	usage, free := m.CheckUsage(m.Capacity, m.Usage)

	if usage > m.Threshold {
		if m.Unit == "%" || m.Unit == "" {
			fmt.Printf(m.Message, usage)
		} else {
			fmt.Printf(m.Message, free)
		}
	}
}

func checkDirect(capacity, usage int) (int, int) {
	return capacity, capacity
}

func checkPercentage(capacity, usage int) (int, int) {
	return usage * percent / capacity, usage
}

func checkDiskSpace(capacity, usage int) (int, int) {
	usagePercent := usage * percent / capacity
	freeSpace := (capacity - usage) / bytesInMegabyte
	return usagePercent, freeSpace
}

func checkNetworkUsage(capacity, usage int) (int, int) {
	usagePercent := usage * percent / capacity
	freeBandwidth := (capacity - usage) / bytesInMegabit
	return usagePercent, freeBandwidth
}

func startPolling(url string) chan string {
	dataChannel := make(chan string)
	client := http.Client{Timeout: httpTimeout}

	go func() {
		defer close(dataChannel)
		errorCounter := 0

		for {
			time.Sleep(requestInterval)

			if errorCounter >= maxRetryCount {
				fmt.Println("Max retries reached")
				break
			}

			response, err := client.Get(url)
			if err != nil {
				fmt.Println("Request error:", err)
				errorCounter++
				continue
			}
			defer response.Body.Close()

			body, err := io.ReadAll(response.Body)
			if err != nil {
				fmt.Println("Error reading response:", err)
				errorCounter++
				continue
			}

			dataChannel <- string(body)
			errorCounter = 0
		}
	}()

	return dataChannel
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
