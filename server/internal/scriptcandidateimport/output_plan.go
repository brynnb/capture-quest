package scriptcandidateimport

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// outputPlan separates compilation from filesystem mutation. Every candidate
// family must finish successfully before Apply publishes any generated file.
type outputPlan struct {
	entries map[string]*plannedFile
	rename  func(string, string) error
	report  func(ReportEvent)
}

type plannedFile struct {
	path     string
	original []byte
	existed  bool
	staged   bool
	content  []byte
}

func newOutputPlan(report ...func(ReportEvent)) *outputPlan {
	plan := &outputPlan{entries: make(map[string]*plannedFile)}
	if len(report) > 0 {
		plan.report = report[0]
	}
	return plan
}

func (plan *outputPlan) ReadFile(path string) ([]byte, error) {
	if entry := plan.entries[path]; entry != nil {
		return append([]byte(nil), entry.content...), nil
	}
	return os.ReadFile(path)
}

// Stage returns whether this operation changes the content visible to later
// planning steps. This preserves sequential merge semantics without touching
// the filesystem before the whole import has been validated.
func (plan *outputPlan) Stage(path string, content []byte) (bool, error) {
	entry := plan.entries[path]
	if entry == nil {
		existing, err := os.ReadFile(path)
		switch {
		case err == nil:
			entry = &plannedFile{
				path:     path,
				original: append([]byte(nil), existing...),
				existed:  true,
				content:  append([]byte(nil), existing...),
			}
		case errors.Is(err, os.ErrNotExist):
			entry = &plannedFile{path: path}
		default:
			return false, fmt.Errorf("read existing %s: %w", path, err)
		}
		plan.entries[path] = entry
	}

	changed := !bytes.Equal(entry.content, content) || (!entry.existed && !entry.staged)
	entry.content = append(entry.content[:0], content...)
	entry.staged = true
	return changed, nil
}

func (plan *outputPlan) Apply(dryRun bool) error {
	paths := plan.changedPaths()

	if dryRun {
		for _, path := range paths {
			plan.emit("would_write", path)
		}
		return nil
	}
	if len(paths) == 0 {
		return nil
	}

	rename := plan.rename
	if rename == nil {
		rename = os.Rename
	}
	temps := make(map[string]string, len(paths))
	backups := make(map[string]string, len(paths))
	cleanup := func() {
		for _, tempPath := range temps {
			_ = os.Remove(tempPath)
		}
		for _, backupPath := range backups {
			_ = os.Remove(backupPath)
		}
	}
	defer cleanup()

	// Prepare every replacement and backup before publishing the first file.
	for _, path := range paths {
		entry := plan.entries[path]
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create output directory for %s: %w", path, err)
		}
		tempPath, err := writeSyncedTemp(filepath.Dir(path), entry.content)
		if err != nil {
			return fmt.Errorf("prepare output for %s: %w", path, err)
		}
		temps[path] = tempPath
		if entry.existed {
			backupPath, err := writeSyncedTemp(filepath.Dir(path), entry.original)
			if err != nil {
				return fmt.Errorf("prepare rollback backup for %s: %w", path, err)
			}
			backups[path] = backupPath
		}
	}

	published := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := rename(temps[path], path); err != nil {
			rollbackErr := plan.rollbackPublished(rename, published, backups)
			if rollbackErr != nil {
				return fmt.Errorf("publish %s: %w (rollback also failed: %v)", path, err, rollbackErr)
			}
			return fmt.Errorf("publish %s: %w", path, err)
		}
		delete(temps, path)
		published = append(published, path)
		plan.emit("wrote", path)
	}
	for _, dir := range uniqueParentDirs(paths) {
		if err := syncDirectory(dir); err != nil {
			rollbackErr := plan.rollbackPublished(rename, published, backups)
			if rollbackErr != nil {
				return fmt.Errorf("sync output directory %s: %w (rollback also failed: %v)", dir, err, rollbackErr)
			}
			return fmt.Errorf("sync output directory %s: %w", dir, err)
		}
	}
	return nil
}

func (plan *outputPlan) emit(kind, path string) {
	if plan.report != nil {
		plan.report(ReportEvent{Kind: kind, Path: path})
	}
}

func (plan *outputPlan) changedPaths() []string {
	paths := make([]string, 0, len(plan.entries))
	for path, entry := range plan.entries {
		if entry.existed && bytes.Equal(entry.original, entry.content) {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func (plan *outputPlan) HasChanges() bool {
	return len(plan.changedPaths()) > 0
}

func writeSyncedTemp(dir string, content []byte) (string, error) {
	file, err := os.CreateTemp(dir, ".script-candidate-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Chmod(0644); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (plan *outputPlan) rollbackPublished(rename func(string, string) error, published []string, backups map[string]string) error {
	var failures []string
	for index := len(published) - 1; index >= 0; index-- {
		path := published[index]
		if backup := backups[path]; backup != "" {
			if err := rename(backup, path); err != nil {
				failures = append(failures, fmt.Sprintf("restore %s: %v", path, err))
			} else {
				delete(backups, path)
			}
		} else if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Sprintf("remove %s: %v", path, err))
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func uniqueParentDirs(paths []string) []string {
	seen := make(map[string]struct{})
	for _, path := range paths {
		seen[filepath.Dir(path)] = struct{}{}
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
