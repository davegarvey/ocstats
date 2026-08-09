package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"ocstats/internal/fetch"
)

const (
	cacheVersion   = 2
	maxHistoryAge  = 31 * 24 * time.Hour
	maxHistorySize = 2000
)

type cacheFile struct {
	Version    int                       `json:"version"`
	Workspaces map[string]workspaceCache `json:"workspaces"`
}

type workspaceCache struct {
	// Latest is the most recent successful fetch for the workspace, whatever
	// its subscription state; it backs the offline stale fallback.
	Latest fetch.Snapshot `json:"latest"`
	// History holds subscribed snapshots in chronological order, bounded by
	// retention; it feeds consumption rate estimation.
	History []fetch.Snapshot `json:"history"`
}

type cache struct {
	path string
}

func defaultCache() *cache {
	if v := os.Getenv("OCSTATS_CACHE_FILE"); v != "" {
		return &cache{path: v}
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	return &cache{path: filepath.Join(dir, "ocstats", "quota.json")}
}

func (c *cache) load() cacheFile {
	if c == nil {
		return cacheFile{}
	}
	b, err := os.ReadFile(c.path)
	if err != nil {
		return cacheFile{}
	}
	var f cacheFile
	if err := json.Unmarshal(b, &f); err != nil || f.Version != cacheVersion {
		return cacheFile{}
	}
	if f.Workspaces == nil {
		f.Workspaces = map[string]workspaceCache{}
	}
	return f
}

// latest returns the most recently stored snapshot for the workspace,
// whatever its subscription state.
func (c *cache) latest(ws string) (fetch.Snapshot, bool) {
	f := c.load()
	w, ok := f.Workspaces[ws]
	if !ok || w.Latest.FetchedAt.IsZero() {
		return fetch.Snapshot{}, false
	}
	return w.Latest, true
}

// history returns the subscribed snapshots retained for the workspace.
func (c *cache) history(ws string) []fetch.Snapshot {
	return c.load().Workspaces[ws].History
}

// append records a successful fetch for the workspace. Subscribed snapshots
// are appended to the bounded history used for rate estimation; every
// successful fetch updates the latest snapshot regardless of subscription
// state. It returns the updated history so pace computation can proceed even
// if persisting fails.
func (c *cache) append(ws string, s fetch.Snapshot, now time.Time) ([]fetch.Snapshot, error) {
	f := c.load()
	if f.Version == 0 {
		f = cacheFile{Version: cacheVersion, Workspaces: map[string]workspaceCache{}}
	}
	if f.Workspaces == nil {
		f.Workspaces = map[string]workspaceCache{}
	}
	w := f.Workspaces[ws]
	w.Latest = s
	if s.Subscription {
		w.History = append(w.History, s)
		w.History = trimHistory(w.History, now)
	}
	f.Workspaces[ws] = w
	return w.History, c.save(f)
}

func trimHistory(h []fetch.Snapshot, now time.Time) []fetch.Snapshot {
	i := 0
	for i < len(h) && now.Sub(h[i].FetchedAt) > maxHistoryAge {
		i++
	}
	h = h[i:]
	if len(h) > maxHistorySize {
		h = h[len(h)-maxHistorySize:]
	}
	return h
}

func (c *cache) save(f cacheFile) error {
	if c == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
