package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Gode-Ts/gode-server/internal/compiler"
)

func TestResolverUsesExplicitCompilerBeforeEnvironment(t *testing.T) {
	t.Setenv("GODEC", "/env/godec")
	resolved, err := compiler.Resolve(compiler.ResolveOptions{Explicit: "/explicit/godec"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Command != "/explicit/godec" || len(resolved.Args) != 0 {
		t.Fatalf("unexpected resolver output: %+v", resolved)
	}
}

func TestResolverFallsBackToLocalCompiler(t *testing.T) {
	dir := t.TempDir()
	compilerDir := filepath.Join(dir, "gode-compiler")
	if err := os.MkdirAll(filepath.Join(compilerDir, "cmd", "godec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(compilerDir, "go.mod"), []byte("module gode.dev/gode-compiler\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := compiler.Resolve(compiler.ResolveOptions{RootDir: dir, LookupPath: func(string) (string, error) {
		return "", os.ErrNotExist
	}})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Command != "go" {
		t.Fatalf("expected go fallback command, got %+v", resolved)
	}
	if len(resolved.Args) != 2 || resolved.Args[0] != "run" || resolved.Args[1] != "./cmd/godec" || resolved.Dir != compilerDir {
		t.Fatalf("unexpected local fallback: %+v", resolved)
	}
}

func TestResolverFallsBackToRemoteCompiler(t *testing.T) {
	dir := t.TempDir()
	resolved, err := compiler.Resolve(compiler.ResolveOptions{RootDir: dir, LookupPath: func(string) (string, error) {
		return "", os.ErrNotExist
	}})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Command != "go" {
		t.Fatalf("expected go remote fallback command, got %+v", resolved)
	}
	if len(resolved.Args) != 2 || resolved.Args[0] != "run" || resolved.Args[1] != "github.com/Gode-Ts/gode-compiler/cmd/godec@latest" || resolved.Dir != "" {
		t.Fatalf("unexpected remote fallback: %+v", resolved)
	}
}
