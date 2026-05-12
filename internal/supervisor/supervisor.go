package supervisor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/Gode-Ts/gode-server/internal/builder"
	"github.com/Gode-Ts/gode-server/internal/config"
)

type Options struct {
	ConfigPath string
	Compiler   string
	Host       string
	Port       int
	Stdout     io.Writer
	Stderr     io.Writer
}

func RunDev(ctx context.Context, opts Options) error {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return err
	}
	if opts.Host != "" {
		cfg.Server.Host = opts.Host
	}
	if opts.Port != 0 {
		cfg.Server.Port = opts.Port
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	var target atomic.Value
	target.Store("")
	var current *worker
	rebuild := func() {
		output := filepath.Join(cfg.Abs(cfg.Build.BinDir), fmt.Sprintf("app-worker-%d", time.Now().UnixNano()))
		result, err := builder.Build(ctx, builder.Options{ConfigPath: cfg.Path, Compiler: opts.Compiler, Output: output, Host: cfg.Server.Host, Port: cfg.Server.Port})
		if err != nil {
			fmt.Fprintf(stderr, "gode rebuild failed: %v\n", err)
			return
		}
		next, err := startWorker(ctx, result.BinaryPath, cfg, stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "gode worker failed: %v\n", err)
			return
		}
		old := current
		current = next
		target.Store(next.url)
		if old != nil {
			_ = old.stop()
		}
		fmt.Fprintf(stdout, "gode dev serving %s via %s\n", publicAddr(cfg), next.url)
	}
	rebuild()
	listener, err := net.Listen("tcp", publicAddr(cfg))
	if err != nil {
		if current != nil {
			_ = current.stop()
		}
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: proxyHandler(&target), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()
	go watchLoop(ctx, cfg, rebuild)
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		if current != nil {
			_ = current.stop()
		}
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func proxyHandler(target *atomic.Value) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := target.Load().(string)
		if raw == "" {
			http.Error(w, "gode worker is not ready", http.StatusServiceUnavailable)
			return
		}
		u, err := url.Parse(raw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		httputil.NewSingleHostReverseProxy(u).ServeHTTP(w, r)
	})
}

type worker struct {
	cmd *exec.Cmd
	url string
}

func (w *worker) stop() error {
	if w == nil || w.cmd == nil || w.cmd.Process == nil {
		return nil
	}
	_ = w.cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- w.cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		_ = w.cmd.Process.Kill()
		return nil
	}
}

func startWorker(ctx context.Context, binary string, cfg config.Config, stdout io.Writer, stderr io.Writer) (*worker, error) {
	handshake := filepath.Join(cfg.Abs(cfg.Build.WorkDir), fmt.Sprintf("worker-%d.addr", time.Now().UnixNano()))
	cmd := exec.CommandContext(ctx, binary)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "GODE_WORKER_ADDR=127.0.0.1:0", "GODE_WORKER_HANDSHAKE="+handshake)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(cfg.StartupTimeout())
	var addr string
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(handshake)
		if err == nil && len(data) > 0 {
			addr = string(data)
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if addr == "" {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("worker did not publish address before timeout")
	}
	workerURL := "http://" + addr
	if err := waitReady(workerURL, cfg.StartupTimeout()); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	return &worker{cmd: cmd, url: workerURL}, nil
}

func waitReady(rawURL string, timeout time.Duration) error {
	client := http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(rawURL + "/__gode/ready")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("worker readiness failed")
}

func watchLoop(ctx context.Context, cfg config.Config, rebuild func()) {
	ticker := time.NewTicker(cfg.ReloadDebounce())
	defer ticker.Stop()
	last := latestModTime(cfg)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := latestModTime(cfg)
			if now.After(last) {
				last = now
				rebuild()
			}
		}
	}
}

func latestModTime(cfg config.Config) time.Time {
	var latest time.Time
	paths := []string{cfg.Abs(cfg.Entry), cfg.Path}
	for _, root := range paths {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".gode" || name == ".git" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err == nil && info.ModTime().After(latest) {
				latest = info.ModTime()
			}
			return nil
		})
	}
	return latest
}

func publicAddr(cfg config.Config) string {
	return fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
}
