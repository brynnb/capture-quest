package scriptcandidateimport

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

// outputPlan separates compilation from filesystem mutation. Every candidate
// family must finish successfully before Apply publishes any generated file.
type outputPlan struct {
	entries map[string]*plannedFile
}

type plannedFile struct {
	path     string
	original []byte
	existed  bool
	staged   bool
	content  []byte
}

func newOutputPlan() *outputPlan {
	return &outputPlan{entries: make(map[string]*plannedFile)}
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
	paths := make([]string, 0, len(plan.entries))
	for path, entry := range plan.entries {
		if entry.existed && bytes.Equal(entry.original, entry.content) {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)

	if dryRun {
		for _, path := range paths {
			log.Printf("[ScriptCandidates] Would write %s", path)
		}
		return nil
	}

	temps := make(map[string]string, len(paths))
	cleanup := func() {
		for _, tempPath := range temps {
			_ = os.Remove(tempPath)
		}
	}
	defer cleanup()

	// Prepare every file before publishing the first one. This prevents an
	// encoding, directory, or write failure from producing a partial import.
	for _, path := range paths {
		entry := plan.entries[path]
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create output directory for %s: %w", path, err)
		}
		file, err := os.CreateTemp(filepath.Dir(path), ".script-candidate-*")
		if err != nil {
			return fmt.Errorf("create temporary output for %s: %w", path, err)
		}
		tempPath := file.Name()
		temps[path] = tempPath
		if err := file.Chmod(0644); err != nil {
			_ = file.Close()
			return fmt.Errorf("set permissions on temporary output for %s: %w", path, err)
		}
		if _, err := file.Write(entry.content); err != nil {
			_ = file.Close()
			return fmt.Errorf("write temporary output for %s: %w", path, err)
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return fmt.Errorf("sync temporary output for %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close temporary output for %s: %w", path, err)
		}
	}

	for _, path := range paths {
		if err := os.Rename(temps[path], path); err != nil {
			return fmt.Errorf("publish %s: %w", path, err)
		}
		delete(temps, path)
		log.Printf("[ScriptCandidates] Wrote %s", path)
	}
	return nil
}
