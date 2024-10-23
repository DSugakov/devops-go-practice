package metrics

import (
	"errors"
	"strconv"
	"strings"
)

const expectedMetricsLength = 7

type ServerMetrics struct {
	CPULoad      int
	MemoryTotal  int
	MemoryUsed   int
	DiskTotal    int
	DiskUsed     int
	NetworkTotal int
	NetworkUsed  int
}

func ParseMetrics(metrics string) (ServerMetrics, error) {
	parts := strings.Split(metrics, ",")
	if len(parts) != expectedMetricsLength {
		return ServerMetrics{}, errors.New("invalid metrics data")
	}
	values := make([]int, expectedMetricsLength)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return ServerMetrics{}, errors.New("invalid data format: %s")
		}
		values[i] = value
	}
	return ServerMetrics{
		CPULoad:      values[0],
		MemoryTotal:  values[1],
		MemoryUsed:   values[2],
		DiskTotal:    values[3],
		DiskUsed:     values[4],
		NetworkTotal: values[5],
		NetworkUsed:  values[6],
	}, nil
}
