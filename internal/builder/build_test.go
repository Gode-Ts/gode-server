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

	got := builder.GenerateWorkerGoMod(wrapperDir, appDir, "")
	rel, err := filepath.Rel(wrapperDir, runtimeDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"require (",
		"github.com/Gode-Ts/gode-runtime v0.1.2",
		"golang.org/x/sync v0.15.0",
		"replace github.com/Gode-Ts/gode-runtime => " + filepath.ToSlash(rel),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("worker go.mod missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateWorkerGoModUsesLocalGopressReplace(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	wrapperDir := filepath.Join(appDir, ".gode", "work", "abc123", "wrapper")
	gopressDir := filepath.Join(root, "gopress")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gopressDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gopressDir, "go.mod"), []byte("module github.com/Gode-Ts/gopress\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := builder.GenerateWorkerGoMod(wrapperDir, appDir, "gopress")
	rel, err := filepath.Rel(wrapperDir, gopressDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"require (",
		"github.com/Gode-Ts/gopress v0.1.6",
		"golang.org/x/sync v0.15.0",
		"replace github.com/Gode-Ts/gopress => " + filepath.ToSlash(rel),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("worker go.mod missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "github.com/Gode-Ts/gode-runtime") {
		t.Fatalf("gopress worker should not require gode-runtime:\n%s", got)
	}
}

func TestGenerateWorkerGoModUsesTaggedGopressWithoutLocalReplace(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	wrapperDir := filepath.Join(appDir, ".gode", "work", "abc123", "wrapper")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := builder.GenerateWorkerGoMod(wrapperDir, appDir, "gopress")

	for _, want := range []string{
		"github.com/Gode-Ts/gopress v0.1.6",
		"golang.org/x/sync v0.15.0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("worker go.mod missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "replace github.com/Gode-Ts/gopress") {
		t.Fatalf("remote gopress worker should not include a local replace:\n%s", got)
	}
}

func TestGenerateWorkerGoModUsesTaggedRuntimeWithoutLocalReplace(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	wrapperDir := filepath.Join(appDir, ".gode", "work", "abc123", "wrapper")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := builder.GenerateWorkerGoMod(wrapperDir, appDir, "")

	for _, want := range []string{
		"github.com/Gode-Ts/gode-runtime v0.1.2",
		"golang.org/x/sync v0.15.0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("worker go.mod missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "replace github.com/Gode-Ts/gode-runtime") {
		t.Fatalf("remote runtime worker should not include a local replace:\n%s", got)
	}
}
