package client

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) TriviaResearchPP/1.0"

type HTTPClient struct {
	Client  *http.Client
	Timeout time.Duration
}

func New() *HTTPClient {
	timeout := 30 * time.Second
	return &HTTPClient{
		Client: &http.Client{
			Timeout: timeout,
		},
		Timeout: timeout,
	}
}

func (c *HTTPClient) Get(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request creation: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml,application/json;q=0.9,*/*;q=0.8")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read body %s: %w", url, err)
	}

	return string(body), nil
}

func (c *HTTPClient) GetWithRetry(url string, retries int) (string, error) {
	var lastErr error
	for i := 0; i <= retries; i++ {
		body, err := c.Get(url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if i < retries {
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
	}
	return "", lastErr
}

func (c *HTTPClient) GetRaw(url string) ([]byte, error) {
	body, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	return []byte(body), nil
}

func StripHTML(text string) string {
	var builder strings.Builder
	inTag := false
	for _, r := range text {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
