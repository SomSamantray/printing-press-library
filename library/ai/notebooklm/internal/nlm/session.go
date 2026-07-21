// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync/atomic"
)

var (
	wizFieldPattern = func(key string) *regexp.Regexp {
		return regexp.MustCompile(`"` + key + `"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`)
	}
	csrfPattern    = wizFieldPattern("SNlM0e")
	sessionPattern = wizFieldPattern("FdrFJe")
	blPattern      = regexp.MustCompile(`"cfb2h"\s*:\s*"([\w.-]+)"|\bbl=([\w.-]+)`)
)

// Session holds batchexecute session tokens scraped from the NotebookLM HTML bootstrap.
type Session struct {
	SID    string
	BL     string
	AT     string
	ReqID  atomic.Int64
	Client *http.Client
}

// Bootstrap loads the home page and extracts f.sid, bl, and at tokens.
func Bootstrap(ctx context.Context, client *http.Client) (*Session, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bootstrap page: status %d", resp.StatusCode)
	}

	html := string(body)
	sid, bl, at, err := extractSessionTokens(html)
	if err != nil {
		return nil, err
	}
	s := &Session{SID: sid, BL: bl, AT: at, Client: client}
	s.ReqID.Store(100000)
	return s, nil
}

func extractSessionTokens(html string) (sid, bl, at string, err error) {
	if m := sessionPattern.FindStringSubmatch(html); m != nil {
		sid = m[1]
	}
	if m := blPattern.FindStringSubmatch(html); m != nil {
		if m[1] != "" {
			bl = m[1]
		} else {
			bl = m[2]
		}
	}
	if m := csrfPattern.FindStringSubmatch(html); m != nil {
		at = m[1]
	}
	if sid == "" {
		return "", "", "", fmt.Errorf("could not extract session id (FdrFJe) — are you logged in?")
	}
	return sid, bl, at, nil
}

func (s *Session) nextReqID() string {
	return strconv.FormatInt(s.ReqID.Add(1), 10)
}

// BuildBatchURL constructs the batchexecute URL for a single rpcid.
func (s *Session) BuildBatchURL(rpcid, sourcePath string) string {
	q := url.Values{}
	q.Set("rpcids", rpcid)
	q.Set("source-path", sourcePath)
	if s.BL != "" {
		q.Set("bl", s.BL)
	}
	q.Set("f.sid", s.SID)
	q.Set("hl", "en")
	q.Set("authuser", "0")
	q.Set("_reqid", s.nextReqID())
	q.Set("rt", "c")
	return BaseURL + BatchExecutePath + "?" + q.Encode()
}

const chromeUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
