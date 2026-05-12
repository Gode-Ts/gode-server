package builder_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gode-Ts/gode-server/internal/builder"
)

func TestGenerateWorkerGoModUsesLocalRuntimeReplace(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	wrapperDir := filepath.Join(appDir, ".gode", "work", "abc123", "wrapper")
	runtimeDir := filepath.Join(root, "gode-runtime")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "go.mod"), []byte("module github.com/Gode-Ts/gode-runtime\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := builder.GenerateWorkerGoMod(wrapperDir, appDir)
	rel, err := filepath.Rel(wrapperDir, runtimeDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"require (",
		"github.com/Gode-Ts/gode-runtime v0.0.0",
		"golang.org/x/sync v0.15.0",
		"replace github.com/Gode-Ts/gode-runtime => " + filepath.ToSlash(rel),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("worker go.mod missing %q:\n%s", want, got)
		}
	}
}
