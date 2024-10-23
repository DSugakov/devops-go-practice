package polling

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const httpTimeout = 10 * time.Second
const requestInterval = 2 * time.Millisecond

func InitiatePolling(url string, retries int) func() chan string {
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
				errorCounter = handleResponseError(response, err, errorCounter)
				if errorCounter > 0 {
					continue
				}

				body, err := io.ReadAll(response.Body)
				if err != nil {
					errorCounter = handlePollingError(err, errorCounter, "failed to parse response")
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

func handleResponseError(response *http.Response, err error, errorCounter int) int {
	if err != nil {
		return handlePollingError(err, errorCounter, "failed to send request")
	}
	if response.StatusCode != http.StatusOK {
		return handlePollingError(fmt.Errorf("invalid status code: %d", response.StatusCode), errorCounter, "")
	}
	return errorCounter
}

func handlePollingError(err error, errorCounter int, message string) int {
	if message != "" {
		fmt.Printf("%s: %s\n", message, err)
	}
	return errorCounter + 1
}
