// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client wraps batchexecute RPC calls for NotebookLM.
type Client struct {
	Session *Session
}

// NewClient bootstraps a session and returns an RPC client.
func NewClient(ctx context.Context, httpClient *http.Client) (*Client, error) {
	sess, err := Bootstrap(ctx, httpClient)
	if err != nil {
		return nil, err
	}
	return &Client{Session: sess}, nil
}

// Call executes a single batchexecute RPC and returns the decoded inner payload.
func (c *Client) Call(ctx context.Context, rpcid, sourcePath string, params any) (json.RawMessage, error) {
	inner := []any{rpcid, mustJSONString(params), nil, "generic"}
	freq, err := json.Marshal([]any{[]any{inner}})
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("f.req", string(freq))
	if c.Session.AT != "" {
		form.Set("at", c.Session.AT)
	}

	u := c.Session.BuildBatchURL(rpcid, sourcePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Origin", BaseURL)
	req.Header.Set("Referer", BaseURL+"/")

	resp, err := c.Session.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batchexecute %s: status %d: %s", rpcid, resp.StatusCode, truncate(string(body), 200))
	}
	frames, err := ParseFrames(string(body))
	if err != nil {
		return nil, err
	}
	for i := len(frames) - 1; i >= 0; i-- {
		if frames[i].RPCID == rpcid {
			return DecodeInnerJSON(frames[i].Payload)
		}
	}
	return nil, fmt.Errorf("batchexecute %s: response missing frame", rpcid)
}

func mustJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
