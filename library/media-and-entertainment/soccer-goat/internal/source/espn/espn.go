package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/cliutil"
)

const (
	searchURL     = "https://site.web.api.espn.com/apis/common/v3/search"
	desktopChrome = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	maxRetries    = 2
)

// Context is the small amount of honest player context exposed by ESPN search.
type Context struct {
	DisplayName string `json:"displayName"`
	Team        string `json:"team"`
	League      string `json:"league"`
	Link        string `json:"link"`
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 12 * time.Second},
		limiter: cliutil.NewAdaptiveLimiter(3),
	}
}

// Lookup returns the first athlete result that includes a web link.
func (c *Client) Lookup(ctx context.Context, name string) (Context, bool, error) {
	if cliutil.IsVerifyEnv() {
		return Context{}, false, nil
	}
	query := url.Values{}
	query.Set("query", name)
	query.Set("limit", "5")
	query.Set("sport", "soccer")
	target := searchURL + "?" + query.Encode()
	body, err := c.get(ctx, target)
	if err != nil {
		return Context{}, false, fmt.Errorf("espn lookup %q: %w", name, err)
	}

	var response struct {
		Items []struct {
			DisplayName string          `json:"displayName"`
			Type        string          `json:"type"`
			Subtitle    string          `json:"subtitle"`
			Team        json.RawMessage `json:"team"`
			League      json.RawMessage `json:"league"`
			Link        string          `json:"link"`
			Links       []struct {
				Href string   `json:"href"`
				Rel  []string `json:"rel"`
			} `json:"links"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return Context{}, false, fmt.Errorf("espn lookup %q: decode response: %w", name, err)
	}

	for _, item := range response.Items {
		if !strings.EqualFold(item.Type, "athlete") && !strings.EqualFold(item.Type, "player") {
			continue
		}
		link := item.Link
		if link == "" {
			link = webLink(item.Links)
		}
		if strings.HasPrefix(link, "/") {
			link = "https://www.espn.com" + link
		}
		if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
			continue
		}
		team := textField(item.Team)
		league := textField(item.League)
		if team == "" {
			team = strings.TrimSpace(item.Subtitle)
		}
		return Context{
			DisplayName: strings.TrimSpace(item.DisplayName),
			Team:        team,
			League:      league,
			Link:        link,
		}, true, nil
	}
	return Context{}, false, nil
}

func webLink(links []struct {
	Href string   `json:"href"`
	Rel  []string `json:"rel"`
}) string {
	for _, link := range links {
		if strings.HasPrefix(link.Href, "http://") || strings.HasPrefix(link.Href, "https://") {
			return link.Href
		}
	}
	return ""
}

func textField(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var object struct {
		DisplayName string `json:"displayName"`
		Name        string `json:"name"`
		Label       string `json:"label"`
	}
	if json.Unmarshal(raw, &object) != nil {
		return ""
	}
	for _, value := range []string{object.DisplayName, object.Name, object.Label} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (c *Client) get(ctx context.Context, target string) ([]byte, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", desktopChrome)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			retryAfter := cliutil.RetryAfter(resp)
			if attempt == maxRetries {
				return nil, &cliutil.RateLimitError{URL: target, RetryAfter: retryAfter, Body: bodySnippet(body)}
			}
			if err := wait(ctx, retryAfter); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GET %s returned HTTP %d: %s", target, resp.StatusCode, bodySnippet(body))
		}
		c.limiter.OnSuccess()
		return body, nil
	}
	return nil, fmt.Errorf("GET %s failed", target)
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func bodySnippet(body []byte) string {
	const max = 512
	text := strings.TrimSpace(string(body))
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}
