package fetch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const (
	BaseURL    = "https://opencode.ai"
	FnQuota    = "c7389bd0e731f80f49593e5ee53835475f4e28594dd6bd83eb229bab753498cd"
	FnLastSeen = "2ce91b3e3223afcebef79e386bb9ca6d735e38770a345ca9570ace4526a6ae56"
)

var instanceCounter atomic.Uint64

type Kind string

const (
	KindAuth        Kind = "auth"
	KindFetch       Kind = "fetch"
	KindNoWorkspace Kind = "no_workspace"
	KindDecode      Kind = "decode"
)

type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func IsKind(err error, kind Kind) bool {
	var e *Error
	return errors.As(err, &e) && e.Kind == kind
}

type Bucket struct {
	Status       string `json:"status"`
	UsagePercent int    `json:"usagePercent"`
	ResetInSec   int    `json:"resetInSec"`
}

type Quota struct {
	Mine         bool     `json:"mine"`
	UseBalance   bool     `json:"useBalance"`
	Region       []string `json:"region"`
	RollingUsage Bucket   `json:"rollingUsage"`
	WeeklyUsage  Bucket   `json:"weeklyUsage"`
	MonthlyUsage Bucket   `json:"monthlyUsage"`
}

type Snapshot struct {
	WorkspaceID  string    `json:"workspaceId"`
	Rolling      Bucket    `json:"rolling"`
	Weekly       Bucket    `json:"weekly"`
	Monthly      Bucket    `json:"monthly"`
	Subscription bool      `json:"subscription"`
	FetchedAt    time.Time `json:"fetchedAt"`
}

type Client struct {
	Base   string
	Cookie string
	HTTP   *http.Client
	// NewCookie, when non-empty after a request, holds the rotated session
	// seal from the response's Set-Cookie header.
	NewCookie string
}

func (c *Client) do(fnID string, args []string) (any, error) {
	u := c.Base + "/_server?id=" + url.QueryEscape(fnID)
	if len(args) > 0 {
		encoded, err := json.Marshal(args)
		if err != nil {
			return nil, &Error{Kind: KindDecode, Msg: "encode args: " + err.Error()}
		}
		u += "&args=" + url.QueryEscape(string(encoded))
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, &Error{Kind: KindFetch, Msg: err.Error()}
	}
	req.Header.Set("X-Server-Id", fnID)
	req.Header.Set("X-Server-Instance", fmt.Sprintf("server-fn:%d", instanceCounter.Add(1)))
	if c.Cookie != "" {
		req.Header.Set("Cookie", "auth="+c.Cookie)
	}
	httpc := c.HTTP
	if httpc == nil {
		httpc = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, &Error{Kind: KindFetch, Msg: "request failed: " + err.Error()}
	}
	defer resp.Body.Close()
	if sealed := cookieFromSetCookie(resp.Header); sealed != "" {
		c.NewCookie = sealed
	}
	if resp.Header.Get("x-error") == "true" || resp.Header.Get("location") != "" {
		return nil, &Error{Kind: KindAuth, Msg: "session rejected by opencode.ai (location: " + resp.Header.Get("location") + ")"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &Error{Kind: KindFetch, Msg: fmt.Sprintf("server returned HTTP %d", resp.StatusCode)}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Kind: KindFetch, Msg: "read response: " + err.Error()}
	}
	val, err := decodeSeroval(body)
	if err != nil {
		return nil, &Error{Kind: KindDecode, Msg: "decode server response: " + err.Error()}
	}
	return val, nil
}

func (c *Client) ResolveWorkspace() (string, error) {
	val, err := c.do(FnLastSeen, nil)
	if err != nil {
		return "", err
	}
	id, ok := val.(string)
	if !ok || id == "" {
		return "", &Error{Kind: KindNoWorkspace, Msg: "no workspace found for this account"}
	}
	return id, nil
}

func (c *Client) QueryQuota(workspaceID string) (Quota, bool, error) {
	var q Quota
	val, err := c.do(FnQuota, []string{workspaceID})
	if err != nil {
		return q, false, err
	}
	if val == nil {
		return q, false, nil
	}
	b, err := json.Marshal(val)
	if err != nil {
		return q, false, &Error{Kind: KindDecode, Msg: "re-marshal: " + err.Error()}
	}
	if err := json.Unmarshal(b, &q); err != nil {
		return q, false, &Error{Kind: KindDecode, Msg: "unexpected quota shape: " + err.Error()}
	}
	if q.RollingUsage.UsagePercent < 0 || q.RollingUsage.UsagePercent > 100 ||
		q.WeeklyUsage.UsagePercent < 0 || q.WeeklyUsage.UsagePercent > 100 ||
		q.MonthlyUsage.UsagePercent < 0 || q.MonthlyUsage.UsagePercent > 100 ||
		q.RollingUsage.ResetInSec < 0 || q.WeeklyUsage.ResetInSec < 0 || q.MonthlyUsage.ResetInSec < 0 {
		return q, false, &Error{Kind: KindDecode, Msg: "quota fields out of range"}
	}
	return q, true, nil
}

func (c *Client) ValidateSession() error {
	if c.Cookie == "" {
		return &Error{Kind: KindAuth, Msg: "no session cookie"}
	}
	req, err := http.NewRequest(http.MethodGet, c.Base+"/auth/status", nil)
	if err != nil {
		return &Error{Kind: KindFetch, Msg: err.Error()}
	}
	req.Header.Set("Cookie", "auth="+c.Cookie)
	httpc := c.HTTP
	if httpc == nil {
		httpc = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return &Error{Kind: KindFetch, Msg: "request failed: " + err.Error()}
	}
	defer resp.Body.Close()
	if sealed := cookieFromSetCookie(resp.Header); sealed != "" {
		c.NewCookie = sealed
	}
	if resp.StatusCode != http.StatusOK {
		return &Error{Kind: KindAuth, Msg: fmt.Sprintf("auth check failed: HTTP %d", resp.StatusCode)}
	}
	var data struct {
		Current string `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return &Error{Kind: KindDecode, Msg: "auth check response: " + err.Error()}
	}
	if data.Current == "" {
		return &Error{Kind: KindAuth, Msg: "no logged-in account at opencode.ai; log in there first"}
	}
	return nil
}

func cookieFromSetCookie(h http.Header) string {
	for _, v := range h.Values("Set-Cookie") {
		if strings.HasPrefix(v, "auth=") {
			if end := strings.Index(v, ";"); end >= 0 {
				return v[len("auth="):end]
			}
			return v[len("auth="):]
		}
	}
	return ""
}

func Query(workspaceID, cookie string) (Snapshot, error) {
	c := &Client{Base: BaseURL, Cookie: cookie}
	q, ok, err := c.QueryQuota(workspaceID)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		WorkspaceID:  workspaceID,
		Rolling:      q.RollingUsage,
		Weekly:       q.WeeklyUsage,
		Monthly:      q.MonthlyUsage,
		Subscription: ok,
		FetchedAt:    time.Now(),
	}, nil
}
