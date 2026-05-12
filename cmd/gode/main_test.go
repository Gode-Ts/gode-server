package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gode ") {
		t.Fatalf("unexpected version output %q", stdout.String())
	}
}

func TestRunInitCreatesStarterProject(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr strings.Builder
	code := run([]string{"init", "--dir", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d stderr=%s", code, stderr.String())
	}
	for _, path := range []string{"gode.json", filepath.Join("src", "app.ts")} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
}

func TestRunInitCreatesGopressStarterProject(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr strings.Builder
	code := run([]string{"init", "--framework", "gopress", "--dir", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d stderr=%s", code, stderr.String())
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "gode.json"))
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile(filepath.Join(dir, "src", "app.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"framework": "gopress"`, `"entry": "./src"`} {
		if !strings.Contains(string(manifest), want) {
			t.Fatalf("gopress manifest missing %q:\n%s", want, manifest)
		}
	}
	for _, want := range []string{`import gopress`, `app.get("/users/:id"`, `export default app`} {
		if !strings.Contains(string(app), want) {
			t.Fatalf("gopress starter app missing %q:\n%s", want, app)
		}
	}
}
