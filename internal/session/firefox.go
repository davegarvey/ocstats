package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	cookieHost = "opencode.ai"
	cookieName = "auth"
	sqliteBin  = "/usr/bin/sqlite3"
)

type Profile struct {
	Name string
	Dir  string
}

var firefoxRoot = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "Firefox"), nil
}

func Profiles() ([]Profile, error) {
	root, err := firefoxRoot()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "profiles.ini"))
	if err != nil {
		return nil, fmt.Errorf("no Firefox installation found at %s: %w", root, err)
	}
	type entry struct {
		name      string
		path      string
		isDefault bool
	}
	var entries []entry
	current := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "["):
			current = len(entries)
			entries = append(entries, entry{})
		case strings.HasPrefix(line, "Name="):
			if len(entries) > 0 {
				entries[current].name = strings.TrimPrefix(line, "Name=")
			}
		case strings.HasPrefix(line, "Path="):
			if len(entries) > 0 {
				entries[current].path = strings.TrimPrefix(line, "Path=")
			}
		case line == "Default=1":
			if len(entries) > 0 {
				entries[current].isDefault = true
			}
		}
	}
	var out []Profile
	for _, e := range entries {
		if e.name == "" || e.path == "" {
			continue
		}
		dir := e.path
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		p := Profile{Name: e.name, Dir: dir}
		if e.isDefault {
			out = append([]Profile{p}, out...)
		} else {
			out = append(out, p)
		}
	}
	return out, nil
}

func cookieDB(profileDir string) string {
	return filepath.Join(profileDir, "cookies.sqlite")
}

func ReadFirefoxCookie(profileDir string) (string, error) {
	db := cookieDB(profileDir)
	if _, err := os.Stat(db); err != nil {
		return "", fmt.Errorf("no cookies.sqlite in profile %s", profileDir)
	}
	tmp, err := os.CreateTemp("", "ocstats-cookies-*.sqlite")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)
	cp := exec.Command("/bin/cp", db, tmpName)
	if out, err := cp.CombinedOutput(); err != nil {
		return "", fmt.Errorf("copy cookie store: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	query := fmt.Sprintf(
		"SELECT value FROM moz_cookies WHERE host=%q AND name=%q AND path=%q ORDER BY expiry DESC LIMIT 1;",
		cookieHost, cookieName, "/",
	)
	cmd := exec.Command(sqliteBin, tmpName, query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read cookie store: %v", err)
	}
	cookie := strings.TrimSpace(string(out))
	if cookie == "" {
		return "", fmt.Errorf("no %s session cookie for %s found in Firefox profile %s", cookieName, cookieHost, profileDir)
	}
	return cookie, nil
}

func ReadFirefoxCookieFromDefaultProfile() (string, error) {
	profiles, err := Profiles()
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, p := range profiles {
		cookie, err := ReadFirefoxCookie(p.Dir)
		if err == nil {
			return cookie, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no Firefox profiles found")
	}
	return "", lastErr
}
