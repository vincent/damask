//go:build desktop

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// fileLock wraps a locked file descriptor.
type fileLock struct {
	f *os.File
}

// acquireLock tries to exclusively lock {configDir}/damask.lock.
// Returns an error if the lock is already held by another process.
func acquireLock(configDir string) (*fileLock, error) {
	path := filepath.Join(configDir, "damask.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("lock: open: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock: already held")
	}
	return &fileLock{f: f}, nil
}

// Release unlocks and closes the lock file.
func (l *fileLock) Release() {
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}
