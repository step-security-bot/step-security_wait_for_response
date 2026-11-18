package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

func validateSubscription() {
	var repoPrivate *bool
	if eventPath := os.Getenv("GITHUB_EVENT_PATH"); eventPath != "" {
		if data, err := os.ReadFile(eventPath); err == nil {
			var event struct {
				Repository struct {
					Private *bool `json:"private"`
				} `json:"repository"`
			}
			if json.Unmarshal(data, &event) == nil {
				repoPrivate = event.Repository.Private
			}
		}
	}

	upstream := "cygnetdigital/wait_for_response"
	action := os.Getenv("GITHUB_ACTION_REPOSITORY")
	docsURL := "https://docs.stepsecurity.io/actions/stepsecurity-maintained-actions"

	fmt.Println("")
	fmt.Println("\u001b[1;36mStepSecurity Maintained Action\u001b[0m")
	fmt.Printf("Secure drop-in replacement for %s\n", upstream)
	if repoPrivate != nil && !*repoPrivate {
		fmt.Println("\u001b[32m\u2713 Free for public repositories\u001b[0m")
	}
	fmt.Printf("\u001b[36mLearn more:\u001b[0m %s\n", docsURL)
	fmt.Println("")

	if repoPrivate != nil && !*repoPrivate {
		return
	}

	serverURL := os.Getenv("GITHUB_SERVER_URL")
	if serverURL == "" {
		serverURL = "https://github.com"
	}
	body := map[string]string{"action": action}
	if serverURL != "https://github.com" {
		body["ghes_server"] = serverURL
	}
	payload, _ := json.Marshal(body)

	apiURL := fmt.Sprintf("https://agent.api.stepsecurity.io/v1/github/%s/actions/maintained-actions-subscription", os.Getenv("GITHUB_REPOSITORY"))
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Println("Timeout or API not reachable. Continuing to next step.")
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusForbidden {
		fmt.Printf("::error::\u001b[1;31mThis action requires a StepSecurity subscription for private repositories.\u001b[0m\n")
		fmt.Printf("::error::\u001b[31mLearn how to enable a subscription: %s\u001b[0m\n", docsURL)
		os.Exit(1)
	}
}

func main() {
	validateSubscription()

	url := os.Getenv("INPUT_URL")
	responseCode := os.Getenv("INPUT_RESPONSECODE")
	timeout, err := strconv.Atoi(os.Getenv("INPUT_TIMEOUT"))
	if err != nil {
		fmt.Println("Timeout must be an integer")
		os.Exit(1)
	}
	interval, err := strconv.Atoi(os.Getenv("INPUT_INTERVAL"))
	if err != nil {
		fmt.Println("Interval must be an integer")
		os.Exit(1)
	}
	localhost := os.Getenv("INPUT_LOCALHOST")

	username := os.Getenv("INPUT_USERNAME")
	password := os.Getenv("INPUT_PASSWORD")

	fmt.Printf("Polling URL `%s` for response code %s for up to %d ms at %d ms intervals\n", url, responseCode, timeout, interval)
	startTime := time.Now()
	timeoutDuration := time.Duration(timeout) * time.Millisecond
	sleepDuration := time.Duration(interval) * time.Millisecond

	if localhost != "" && strings.Contains(url, "localhost") {
		url = strings.ReplaceAll(url, "localhost", localhost)
	}

	client := &http.Client{}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		fmt.Printf("Request Construction: %v\n", err)
		os.Exit(1)
	}

	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	codes := strings.Split(responseCode, ",")

	for {
		res, err := client.Do(req)
		if err == nil {
			code := res.StatusCode
			_ = res.Body.Close()
			if slices.Contains(codes, strconv.Itoa(code)) {
				fmt.Printf("Received expected response code: %d\n", code)
				os.Exit(0)
			}
		}
		time.Sleep(sleepDuration)
		elapsed := time.Since(startTime)
		if elapsed > timeoutDuration {
			fmt.Printf("Timed out\n")
			os.Exit(1)
		}
	}
}
