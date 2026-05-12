package e2e_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestBuildProducesStandaloneServer(t *testing.T) {
	root := repoRoot(t)
	appDir := t.TempDir()
	runGode(t, root, "init", "--dir", appDir)
	binPath := filepath.Join(appDir, "dist", "app")
	runGode(t, root, "build", "--config", filepath.Join(appDir, "gode.json"), "--output", binPath)

	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GODE_PORT=%d", port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	logFile, err := os.Create(filepath.Join(appDir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	}()

	body := waitBody(t, fmt.Sprintf("http://127.0.0.1:%d/users/abc", port), "Bearer x", "abc")
	if body != "abc" {
		t.Fatalf("expected abc, got %q", body)
	}
}

func TestGopressBuildProducesStandaloneServer(t *testing.T) {
	root := repoRoot(t)
	appDir := t.TempDir()
	runGode(t, root, "init", "--framework", "gopress", "--dir", appDir)
	binPath := filepath.Join(appDir, "dist", "app")
	runGode(t, root, "build", "--config", filepath.Join(appDir, "gode.json"), "--output", binPath)

	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GODE_PORT=%d", port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	logFile, err := os.Create(filepath.Join(appDir, "gopress-server.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	}()

	body := waitBody(t, fmt.Sprintf("http://127.0.0.1:%d/users/abc", port), "", "abc")
	if body != "abc" {
		t.Fatalf("expected abc, got %q", body)
	}
}

func TestDevReloadKeepsOldWorkerOnBuildFailure(t *testing.T) {
	root := repoRoot(t)
	appDir := t.TempDir()
	runGode(t, root, "init", "--dir", appDir)

	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin(), "run", "./cmd/gode", "dev", "--config", filepath.Join(appDir, "gode.json"), "--port", fmt.Sprintf("%d", port))
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	logFile, err := os.Create(filepath.Join(appDir, "dev.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = exec.Command("pkill", "-f", filepath.Join(appDir, ".gode", "bin", "app-worker")).Run()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	}()

	waitBody(t, fmt.Sprintf("http://127.0.0.1:%d/users/abc", port), "Bearer x", "abc")
	appTS := filepath.Join(appDir, "src", "app.ts")
	data, err := os.ReadFile(appTS)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appTS, []byte(strings.ReplaceAll(string(data), "body: ctx.paramId", `body: "v2"`)), 0o644); err != nil {
		t.Fatal(err)
	}
	waitBody(t, fmt.Sprintf("http://127.0.0.1:%d/users/abc", port), "Bearer x", "v2")
	if err := os.WriteFile(appTS, append([]byte(strings.ReplaceAll(string(data), "body: ctx.paramId", `body: "v2"`)), []byte("\nexport class Broken {}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	body := mustBody(t, fmt.Sprintf("http://127.0.0.1:%d/users/abc", port), "Bearer x")
	if body != "v2" {
		t.Fatalf("expected old worker to keep serving v2 after bad rebuild, got %q", body)
	}
}

func runGode(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command(goBin(), append([]string{"run", "./cmd/gode"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH=/opt/homebrew/Cellar/go/1.24.4/libexec/bin:"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gode %v failed: %v\n%s", args, err, out)
	}
}

func waitBody(t *testing.T, rawURL string, auth string, want string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		body := mustBody(t, rawURL, auth)
		last = body
		if body == want {
			return body
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q, last body %q", want, last)
	return ""
}

func mustBody(t *testing.T, rawURL string, auth string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	client := http.Client{Timeout: time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repo root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func goBin() string {
	if path, err := exec.LookPath("go"); err == nil {
		return path
	}
	return "/opt/homebrew/Cellar/go/1.24.4/libexec/bin/go"
}
