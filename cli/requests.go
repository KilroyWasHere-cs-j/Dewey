package main

import (
	"errors"
	"fmt"
	"os"

	"dewey-httpclient"
)

// printResult renders body/err the way every CLI command already expects:
// a *httpclient.StatusError (backend responded, just not with success)
// prints the body in red and returns without exiting; any other error
// (transport/read failure) prints and exits non-zero. Success is handed
// to formatter for display, falling back to indented JSON in green if
// formatter is nil. Kept here (rather than in httpclient) since
// colored/exit-on-failure output is specific to this binary — dewey-mcp's
// equivalent just returns (value, error) instead (issue #433).
func printResult(body []byte, err error, formatter func([]byte) string) {
	var statusErr *httpclient.StatusError
	if errors.As(err, &statusErr) {
		fmt.Println(colorRed + prettyJSON(statusErr.Body) + colorReset)
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, colorRed+"request failed:"+colorReset, err)
		os.Exit(1)
	}

	if formatter != nil {
		fmt.Println(formatter(body))
		return
	}
	fmt.Println(colorGreen + prettyJSON(body) + colorReset)
}

func get(url string, password string, formatter func([]byte) string) {
	c := httpclient.Client{Password: password}
	body, err := c.Get(url)
	printResult(body, err, formatter)
}

func del(url string, password string, formatter func([]byte) string) {
	c := httpclient.Client{Password: password}
	body, err := c.Del(url)
	printResult(body, err, formatter)
}

func postJSON(url string, password string, payload any, formatter func([]byte) string) {
	c := httpclient.Client{Password: password}
	body, err := c.PostJSON(url, payload)
	printResult(body, err, formatter)
}

// upload sends filePath to the backend's /upload route as multipart/form-data,
// delegating the actual request-building to httpclient.Client.Upload.
func upload(url, password, filePath string, meta map[string]string) {
	c := httpclient.Client{Password: password}
	body, err := c.Upload(url, filePath, meta)
	printResult(body, err, nil)
}
