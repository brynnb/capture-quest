package scriptcandidateimport

import (
	"context"
	"errors"
	"testing"
)

func TestImportLockSerializesWritersAndHonorsCancellation(t *testing.T) {
	outputDir := t.TempDir()
	first, err := acquireImportLock(context.Background(), outputDir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := acquireImportLock(ctx, outputDir); !errors.Is(err, context.Canceled) {
		t.Fatalf("second lock error = %v, want context.Canceled", err)
	}
}
