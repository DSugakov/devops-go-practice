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
			fmt.Println("Error parsing metrics:", err)
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
	return capacity, capacity
}

func getPercentageUsage(capacity, usage int) (int, int) {
	if capacity == 0 {
		return 0, 0
	}
	return usage * 100 / capacity, usage * 100 / capacity
}

func getFreeDiskSpace(capacity, usage int) (int, int) {
	if capacity == 0 {
		return 0, 0
	}
	freeResource := (capacity - usage) / (1024 * 1024)
	return usage * 100 / capacity, freeResource
}

func getFreeNetworkBandwidth(capacity, usage int) (int, int) {
	if capacity == 0 {
		return 0, 0
	}
	freeResource := (capacity - usage) / (1000 * 1000)
	return usage * 100 / capacity, freeResource
}

func startPolling(url string, retries int) func() chan string {
	return func() chan string {
		dataChannel := make(chan string)
		client := http.Client{Timeout: httpTimeout}
		errorCounter := 0

		go func() {
			defer close(dataChannel)

			for {
				time.Sleep(requestInterval)

				if errorCounter >= retries {
					fmt.Println("Не удалось получить статистику сервера после нескольких попыток.")
					break
				}

				response, err := client.Get(url)
				errorCounter = processResponseError(response, err, errorCounter)
				if errorCounter > 0 {
					continue
				}

				body, err := io.ReadAll(response.Body)
				if err != nil {
					errorCounter = incrementErrorCount(err, errorCounter, "не удалось прочитать тело ответа")
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
		fmt.Printf("Ошибка при отправке запроса: %s\n", err)
		return incrementErrorCount(err, errorCounter, "")
	}
	if response.StatusCode != http.StatusOK {
		fmt.Printf("Неверный код статуса: %d\n", response.StatusCode)
		return incrementErrorCount(fmt.Errorf("invalid status code"), errorCounter, "")
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
		return ServerMetrics{}, fmt.Errorf("некорректный формат данных")
	}

	values := make([]int, expectedMetricsLength)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return ServerMetrics{}, fmt.Errorf("некорректное число: %s", part)
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
