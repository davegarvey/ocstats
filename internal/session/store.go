package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const sessionFileName = "session"

type Store struct {
	Dir string
}

func DefaultStore() *Store {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return &Store{Dir: filepath.Join(dir, "ocstats")}
}

func (s *Store) path() string {
	return filepath.Join(s.Dir, sessionFileName)
}

func (s *Store) Save(cookie string) error {
	if cookie == "" {
		return fmt.Errorf("refusing to store empty session")
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	path := s.path()
	tmp, err := os.CreateTemp(s.Dir, ".session-*")
	if err != nil {
		return fmt.Errorf("temp session file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(cookie); err != nil {
		tmp.Close()
		return fmt.Errorf("write session: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod session: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install session: %w", err)
	}
	return nil
}

func (s *Store) Load() (string, error) {
	path := s.path()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoSession
		}
		return "", fmt.Errorf("read session: %w", err)
	}
	cookie := string(data)
	if cookie == "" {
		return "", ErrNoSession
	}
	return cookie, nil
}

func (s *Store) Delete() error {
	err := os.Remove(s.path())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

var ErrNoSession = errors.New("no stored session; run \"ocstats login\" first")

func (s *Store) FileMode() (os.FileMode, error) {
	info, err := os.Stat(s.path())
	if err != nil {
		return 0, err
	}
	return info.Mode().Perm(), nil
}
