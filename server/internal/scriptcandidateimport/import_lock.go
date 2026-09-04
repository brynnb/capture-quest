package scriptcandidateimport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type importLock struct {
	file *os.File
}

func acquireImportLock(ctx context.Context, outputDir string) (*importLock, error) {
	lockDir := filepath.Dir(filepath.Clean(outputDir))
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("create import lock directory: %w", err)
	}
	// Lock the stable output-root directory itself. A removable lock file can
	// create two lock inodes during concurrent cleanup and accidentally admit
	// multiple writers.
	file, err := os.Open(lockDir)
	if err != nil {
		return nil, fmt.Errorf("open import lock: %w", err)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &importLock{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			file.Close()
			return nil, fmt.Errorf("lock script candidate output: %w", err)
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, fmt.Errorf("wait for script candidate import lock: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (lock *importLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	closeErr := lock.file.Close()
	if unlockErr != nil {
		return fmt.Errorf("unlock script candidate output: %w", unlockErr)
	}
	return closeErr
}
