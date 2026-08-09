package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStoreSaveLoadDelete(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: filepath.Join(dir, "ocstats")}
	if _, err := s.Load(); err != ErrNoSession {
		t.Fatalf("expected ErrNoSession, got %v", err)
	}
	cookie := "Fe26.2**test-value"
	if err := s.Save(cookie); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != cookie {
		t.Fatalf("got %q, want %q", got, cookie)
	}
	mode, err := s.FileMode()
	if err != nil {
		t.Fatal(err)
	}
	if mode != 0o600 {
		t.Fatalf("file mode = %o, want 600", mode)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err != ErrNoSession {
		t.Fatalf("expected ErrNoSession after delete, got %v", err)
	}
}

func TestStoreRejectsEmpty(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.Save(""); err == nil {
		t.Fatal("expected error for empty cookie")
	}
}

func TestStoreOverwriteKeepsMode(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	if err := s.Save("first"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("second"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Load()
	if got != "second" {
		t.Fatalf("got %q", got)
	}
	mode, _ := s.FileMode()
	if mode != 0o600 {
		t.Fatalf("file mode = %o", mode)
	}
}

func TestProfilesParse(t *testing.T) {
	root := t.TempDir()
	ini := `[General]
StartWithLastProfile=1

[Profile0]
Name=default-release
IsRelative=1
Path=t4648y9w.default-release
Default=1

[Profile1]
Name=dev
IsRelative=1
Path=z5yt2v13.dev
`
	if err := os.WriteFile(filepath.Join(root, "profiles.ini"), []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}
	old := firefoxRoot
	firefoxRoot = func() (string, error) { return root, nil }
	defer func() { firefoxRoot = old }()
	profiles, err := Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles", len(profiles))
	}
	if profiles[0].Name != "default-release" {
		t.Fatalf("default profile first, got %+v", profiles)
	}
	if profiles[0].Dir != filepath.Join(root, "t4648y9w.default-release") {
		t.Fatalf("unexpected dir %q", profiles[0].Dir)
	}
}

func TestReadFirefoxCookie(t *testing.T) {
	profile := t.TempDir()
	root := filepath.Dir(profile)
	old := firefoxRoot
	firefoxRoot = func() (string, error) { return root, nil }
	defer func() { firefoxRoot = old }()
	if err := os.WriteFile(filepath.Join(root, "profiles.ini"), []byte(
		"[Profile0]\nName=default\nPath="+filepath.Base(profile)+"\nDefault=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	createCookieDB(t, filepath.Join(profile, "cookies.sqlite"))
	cookie, err := ReadFirefoxCookieFromDefaultProfile()
	if err != nil {
		t.Fatal(err)
	}
	if cookie != "Fe26.2**firefox-test" {
		t.Fatalf("unexpected cookie %q", cookie)
	}
}

func createCookieDB(t *testing.T, path string) {
	t.Helper()
	schema := `CREATE TABLE moz_cookies (
  id INTEGER PRIMARY KEY, originAttributes TEXT NOT NULL DEFAULT '',
  name TEXT, value TEXT, host TEXT, path TEXT, expiry INTEGER,
  lastAccessed INTEGER, creationTime INTEGER, isSecure INTEGER, isHttpOnly INTEGER,
  inBrowserElement INTEGER DEFAULT 0, sameSite INTEGER DEFAULT 0, schemeMap INTEGER DEFAULT 0,
  isPartitionedAttributeSet INTEGER DEFAULT 0, updateTime INTEGER,
  CONSTRAINT moz_uniqueid UNIQUE (name, host, path, originAttributes));
INSERT INTO moz_cookies (name, value, host, path, expiry, isSecure, isHttpOnly) VALUES
  ('auth', 'Fe26.2**firefox-test', 'opencode.ai', '/', 1811535857385, 0, 1),
  ('oc_locale', 'en', 'opencode.ai', '/', 1817231747381, 0, 0);
`
	out, err := exec.Command("/usr/bin/sqlite3", path, schema).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3: %v %s", err, out)
	}
}
