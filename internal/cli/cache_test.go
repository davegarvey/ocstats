package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ocstats/internal/fetch"
)

func snap(ws string, sub bool, pct int, at time.Time) fetch.Snapshot {
	return fetch.Snapshot{
		WorkspaceID:  ws,
		Rolling:      fetch.Bucket{Status: "ok", UsagePercent: pct, ResetInSec: 100},
		Weekly:       fetch.Bucket{Status: "ok", UsagePercent: pct, ResetInSec: 200},
		Monthly:      fetch.Bucket{Status: "ok", UsagePercent: pct, ResetInSec: 300},
		Subscription: sub,
		FetchedAt:    at,
	}
}

func newTestCache(t *testing.T) *cache {
	t.Helper()
	return &cache{path: filepath.Join(t.TempDir(), "quota.json")}
}

func TestCacheWriteRead(t *testing.T) {
	c := newTestCache(t)
	now := time.Now()
	s := snap("wrk_test", true, 53, now)
	if _, err := c.append("wrk_test", s, now); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(c.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cache mode = %o, want 600", info.Mode().Perm())
	}
	got, ok := c.latest("wrk_test")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.WorkspaceID != s.WorkspaceID || !got.FetchedAt.Equal(now) {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	h := c.history("wrk_test")
	if len(h) != 1 || !h[0].FetchedAt.Equal(now) {
		t.Fatalf("history mismatch: %+v", h)
	}
}

func TestCacheHistoryOrdering(t *testing.T) {
	c := newTestCache(t)
	base := time.Now()
	for i := 0; i < 5; i++ {
		if _, err := c.append("wrk_x", snap("wrk_x", true, 10+i, base.Add(time.Duration(i)*time.Hour)), base.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	h := c.history("wrk_x")
	if len(h) != 5 {
		t.Fatalf("history length = %d, want 5", len(h))
	}
	for i := 1; i < len(h); i++ {
		if !h[i].FetchedAt.After(h[i-1].FetchedAt) {
			t.Fatalf("history not chronological: %v then %v", h[i-1].FetchedAt, h[i].FetchedAt)
		}
	}
}

func TestCacheRetentionEvictionAge(t *testing.T) {
	c := newTestCache(t)
	now := time.Now()
	old := snap("wrk_x", true, 5, now.Add(-32*24*time.Hour))
	newer := snap("wrk_x", true, 6, now.Add(-time.Hour))
	if _, err := c.append("wrk_x", old, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.append("wrk_x", newer, now); err != nil {
		t.Fatal(err)
	}
	h := c.history("wrk_x")
	if len(h) != 1 || !h[0].FetchedAt.Equal(newer.FetchedAt) {
		t.Fatalf("expected old snapshot evicted, got: %+v", h)
	}
}

func TestCacheRetentionEvictionCount(t *testing.T) {
	c := newTestCache(t)
	now := time.Now()
	h := make([]fetch.Snapshot, 0, maxHistorySize+5)
	for i := 0; i < maxHistorySize+5; i++ {
		h = append(h, snap("wrk_x", true, 1, now.Add(-time.Duration(maxHistorySize+5-i)*time.Minute)))
	}
	over := cacheFile{Version: cacheVersion, Workspaces: map[string]workspaceCache{
		"wrk_x": {History: h},
	}}
	if err := c.save(over); err != nil {
		t.Fatal(err)
	}
	if _, err := c.append("wrk_x", snap("wrk_x", true, 2, now), now); err != nil {
		t.Fatal(err)
	}
	got := c.history("wrk_x")
	if len(got) != maxHistorySize {
		t.Fatalf("history length = %d, want %d", len(got), maxHistorySize)
	}
	if !got[len(got)-1].FetchedAt.Equal(now) {
		t.Fatalf("newest snapshot missing from retained history")
	}
}

func TestCacheVersionMismatch(t *testing.T) {
	c := newTestCache(t)
	now := time.Now()
	legacy := []byte(`{"snapshot":{"workspaceId":"wrk_x","fetchedAt":"2026-01-01T00:00:00Z"}}`)
	if err := os.WriteFile(c.path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.latest("wrk_x"); ok {
		t.Fatal("v1-shaped cache must be treated as empty")
	}
	unknown := `{"version":99,"workspaces":{}}`
	if err := os.WriteFile(c.path, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.latest("wrk_x"); ok {
		t.Fatal("unknown-version cache must be treated as empty")
	}
	if _, err := c.append("wrk_x", snap("wrk_x", true, 1, now), now); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.latest("wrk_x"); !ok {
		t.Fatal("cache not rebuilt after version mismatch")
	}
}

func TestCacheStaleReadAndNoSubExclusion(t *testing.T) {
	c := newTestCache(t)
	now := time.Now()
	if _, err := c.append("wrk_x", snap("wrk_x", true, 42, now.Add(-time.Hour)), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.append("wrk_x", snap("wrk_x", false, 0, now), now); err != nil {
		t.Fatal(err)
	}
	latest, ok := c.latest("wrk_x")
	if !ok {
		t.Fatal("expected latest snapshot")
	}
	if latest.Subscription {
		t.Fatal("latest must reflect the no-subscription fetch")
	}
	h := c.history("wrk_x")
	if len(h) != 1 {
		t.Fatalf("no-subscription snapshot leaked into history: %+v", h)
	}
	if h[0].Subscription != true {
		t.Fatal("history must only hold subscribed snapshots")
	}
}

func TestCachePerWorkspaceIsolation(t *testing.T) {
	c := newTestCache(t)
	now := time.Now()
	if _, err := c.append("wrk_a", snap("wrk_a", true, 10, now), now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.append("wrk_b", snap("wrk_b", true, 90, now), now); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.latest("wrk_nope"); ok {
		t.Fatal("unexpected snapshot for unknown workspace")
	}
	if got, _ := c.latest("wrk_a"); got.WorkspaceID != "wrk_a" {
		t.Fatalf("workspace isolation broken: %+v", got)
	}
	if len(c.history("wrk_a")) != 1 || len(c.history("wrk_b")) != 1 {
		t.Fatal("per-workspace history corrupted")
	}
}

func TestCacheReadCorrupt(t *testing.T) {
	c := newTestCache(t)
	if err := os.WriteFile(c.path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.latest("wrk_x"); ok {
		t.Fatal("expected no cache hit for corrupt file")
	}
}

func TestCacheReadMissing(t *testing.T) {
	c := newTestCache(t)
	if _, ok := c.latest("wrk_x"); ok {
		t.Fatal("expected no cache hit for missing file")
	}
}

func TestCacheFileJSONShape(t *testing.T) {
	c := newTestCache(t)
	now := time.Now()
	if _, err := c.append("wrk_x", snap("wrk_x", true, 1, now), now); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(c.path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if v, ok := raw["version"].(float64); !ok || v != cacheVersion {
		t.Fatalf("unexpected cache shape: %s", b)
	}
}
