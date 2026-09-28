package service

import (
	"path/filepath"
	"testing"
)

func TestBatchImportInitialDirectory(t *testing.T) {
	existingDirectory := t.TempDir()
	if got := batchImportInitialDirectory(existingDirectory); got != filepath.Clean(existingDirectory) {
		t.Fatalf("expected existing directory %q, got %q", existingDirectory, got)
	}

	if got := batchImportInitialDirectory(filepath.Join(existingDirectory, "missing")); got != "" {
		t.Fatalf("expected missing directory to be rejected, got %q", got)
	}

	if got := batchImportInitialDirectory(`/Users/example/Games`); got != "" {
		t.Fatalf("expected Unix absolute path to be rejected, got %q", got)
	}
}
