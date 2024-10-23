package alerts

import (
	"devops-go-practice/metrics"
	"fmt"
)

const (
	cpuLoadThreshold      = 20
	memoryUsageThreshold  = 75
	diskUsageThreshold    = 90
	networkUsageThreshold = 85

	bytesInMegabyte = 1024 * 1024
	bytesInMegabit  = 1000 * 1000
	fullPercent     = 100
)

type Metric struct {
	Total          int
	Used           int
	Threshold      int
	AlertMessage   string
	Unit           string
	CalculateUsage func(total, used int) (int, int)
}

func CheckMetrics(serverMetrics metrics.ServerMetrics) {
	metricList := []Metric{
		{
			Total:          serverMetrics.CPULoad,
			Used:           serverMetrics.CPULoad,
			Threshold:      cpuLoadThreshold,
			AlertMessage:   "Load Average is too high: %d\n",
			Unit:           "",
			CalculateUsage: calculateDirectUsage,
		},
		{
			Total:          serverMetrics.MemoryTotal,
			Used:           serverMetrics.MemoryUsed,
			Threshold:      memoryUsageThreshold,
			AlertMessage:   "Memory usage too high: %d%%\n",
			Unit:           "%",
			CalculateUsage: calculatePercentageUsage,
		},
		{
			Total:          serverMetrics.DiskTotal,
			Used:           serverMetrics.DiskUsed,
			Threshold:      diskUsageThreshold,
			AlertMessage:   "Free disk space is too low: %d Mb left\n",
			Unit:           "Mb",
			CalculateUsage: calculateFreeResource,
		},
		{
			Total:          serverMetrics.NetworkTotal,
			Used:           serverMetrics.NetworkUsed,
			Threshold:      networkUsageThreshold,
			AlertMessage:   "Network bandwidth usage high: %d Mbit/s available\n",
			Unit:           "Mbit/s",
			CalculateUsage: calculateFreeNetworkResource,
		},
	}

	for _, metric := range metricList {
		checkResourceUsage(metric)
	}
}

func checkResourceUsage(m Metric) {
	usagePercent, freeResource := m.CalculateUsage(m.Total, m.Used)

	if usagePercent > m.Threshold {
		if m.Unit == "%" || m.Unit == "" {
			fmt.Printf(m.AlertMessage, usagePercent)
		} else {
			fmt.Printf(m.AlertMessage, freeResource)
		}
	}
}

func calculateDirectUsage(total, _ int) (int, int) {
	return total, total
}

func calculatePercentageUsage(total, used int) (int, int) {
	usagePercent := used * fullPercent / total
	return usagePercent, usagePercent
}

func calculateFreeResource(total, used int) (int, int) {
	usagePercent := used * fullPercent / total
	freeResource := (total - used) / bytesInMegabyte
	return usagePercent, freeResource
}

func calculateFreeNetworkResource(total, used int) (int, int) {
	usagePercent := used * fullPercent / total
	freeResource := (total - used) / bytesInMegabit
	return usagePercent, freeResource
}
