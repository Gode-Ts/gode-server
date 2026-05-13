package builder

import (
	"path/filepath"
	"testing"

	"github.com/Gode-Ts/gode-server/internal/config"
)

func TestBuildOutputPathResolvesRelativeOutputFromProjectRoot(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.RootDir = root

	got := buildOutputPath(cfg, "./dist/app")
	want := filepath.Join(root, "./dist/app")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildOutputPathKeepsAbsoluteOutput(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.RootDir = root
	want := filepath.Join(t.TempDir(), "app")

	got := buildOutputPath(cfg, want)
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
