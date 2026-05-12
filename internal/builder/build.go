package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Gode-Ts/gode-server/internal/compiler"
	"github.com/Gode-Ts/gode-server/internal/config"
)

type Options struct {
	ConfigPath string
	Output     string
	Compiler   string
	RootDir    string
	Host       string
	Port       int
}

type Result struct {
	BinaryPath string
	WorkDir    string
	WrapperDir string
}

func Build(ctx context.Context, opts Options) (Result, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return Result{}, err
	}
	if opts.RootDir != "" {
		cfg.RootDir = opts.RootDir
	}
	if opts.Host != "" {
		cfg.Server.Host = opts.Host
	}
	if opts.Port != 0 {
		cfg.Server.Port = opts.Port
	}
	resolved, err := compiler.Resolve(compiler.ResolveOptions{Explicit: opts.Compiler, RootDir: cfg.RootDir})
	if err != nil {
		return Result{}, err
	}
	workRoot := cfg.Abs(cfg.Build.WorkDir)
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		return Result{}, err
	}
	buildID := buildID(cfg)
	workDir := filepath.Join(workRoot, buildID)
	srcDir := filepath.Join(workDir, "src")
	wrapperDir := filepath.Join(workDir, "wrapper")
	if err := os.RemoveAll(workDir); err != nil {
		return Result{}, err
	}
	if err := copyDir(cfg.Abs(cfg.Entry), srcDir); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(srcDir, "gode_prelude.ts"), []byte(GeneratePrelude(cfg)), 0o644); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := runCommand(ctx, resolved, "check", srcDir, "--config", cfg.Path, "--format", "json"); err != nil {
		return Result{}, err
	}
	if err := runCommand(ctx, resolved, "compile", srcDir, "--config", cfg.Path, "--out", wrapperDir, "--package", "main"); err != nil {
		return Result{}, err
	}
	wrapper, err := GenerateWrapper(cfg, cfg.Server.Host, cfg.Server.Port)
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(wrapperDir, "main.go"), []byte(wrapper), 0o644); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(wrapperDir, "go.mod"), []byte(GenerateWorkerGoMod(wrapperDir, cfg.RootDir, cfg.Framework)), 0o644); err != nil {
		return Result{}, err
	}
	output := opts.Output
	if output == "" {
		output = filepath.Join(cfg.Abs(cfg.Build.DistDir), "app")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return Result{}, err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", output, ".")
	cmd.Dir = wrapperDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return Result{}, fmt.Errorf("go build failed: %w\n%s", err, string(out))
	}
	return Result{BinaryPath: output, WorkDir: workDir, WrapperDir: wrapperDir}, nil
}

func Check(ctx context.Context, cfg config.Config, compilerPath string) error {
	resolved, err := compiler.Resolve(compiler.ResolveOptions{Explicit: compilerPath, RootDir: cfg.RootDir})
	if err != nil {
		return err
	}
	workRoot := cfg.Abs(cfg.Build.WorkDir)
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		return err
	}
	workDir := filepath.Join(workRoot, "check-"+buildID(cfg))
	srcDir := filepath.Join(workDir, "src")
	if err := os.RemoveAll(workDir); err != nil {
		return err
	}
	if err := copyDir(cfg.Abs(cfg.Entry), srcDir); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(srcDir, "gode_prelude.ts"), []byte(GeneratePrelude(cfg)), 0o644); err != nil {
		return err
	}
	return runCommand(ctx, resolved, "check", srcDir, "--config", cfg.Path, "--format", "json")
}

const (
	runtimeModule  = "github.com/Gode-Ts/gode-runtime"
	runtimeVersion = "v0.1.0"
	gopressModule  = "github.com/Gode-Ts/gopress"
	gopressVersion = "v0.1.0"
)

func GenerateWorkerGoMod(wrapperDir string, rootDir string, framework string) string {
	var b strings.Builder
	b.WriteString("module gode.app/worker\n\n")
	b.WriteString("go 1.23.0\n\n")
	b.WriteString("require (\n")
	if framework == "gopress" {
		b.WriteString("\t")
		b.WriteString(gopressModule)
		b.WriteString(" ")
		b.WriteString(gopressVersion)
		b.WriteString("\n")
	} else {
		b.WriteString("\t")
		b.WriteString(runtimeModule)
		b.WriteString(" ")
		b.WriteString(runtimeVersion)
		b.WriteString("\n")
	}
	b.WriteString("\tgolang.org/x/sync v0.15.0\n")
	b.WriteString(")\n")
	if framework == "gopress" {
		if local, ok := localModulePath(rootDir, wrapperDir, "gopress"); ok {
			b.WriteString("\n")
			b.WriteString("replace ")
			b.WriteString(gopressModule)
			b.WriteString(" => ")
			b.WriteString(filepath.ToSlash(local))
			b.WriteString("\n")
		}
		return b.String()
	}
	if local, ok := localModulePath(rootDir, wrapperDir, "gode-runtime"); ok {
		b.WriteString("\n")
		b.WriteString("replace ")
		b.WriteString(runtimeModule)
		b.WriteString(" => ")
		b.WriteString(filepath.ToSlash(local))
		b.WriteString("\n")
	}
	return b.String()
}

func localModulePath(rootDir string, wrapperDir string, moduleDir string) (string, bool) {
	candidates := moduleCandidates(rootDir, moduleDir)
	seen := map[string]bool{}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
			continue
		}
		rel, err := filepath.Rel(wrapperDir, abs)
		if err != nil {
			return abs, true
		}
		if rel == "" {
			rel = "."
		}
		return rel, true
	}
	return "", false
}

func moduleCandidates(rootDir string, moduleDir string) []string {
	var bases []string
	addBase := func(path string) {
		if path == "" {
			return
		}
		bases = append(bases, path)
	}
	addBase(rootDir)
	if wd, err := os.Getwd(); err == nil {
		addBase(wd)
		addBase(filepath.Dir(wd))
	}
	addBase(".")
	addBase("..")

	var out []string
	for _, base := range bases {
		out = append(out,
			filepath.Join(base, moduleDir),
			filepath.Join(base, "..", moduleDir),
		)
	}
	return out
}

func runCommand(ctx context.Context, resolved compiler.Command, args ...string) error {
	cmd := exec.CommandContext(ctx, resolved.Command, resolved.ArgsFor(args...)...)
	cmd.Dir = resolved.Dir
	cmd.Env = append(os.Environ(), "PATH=/opt/homebrew/Cellar/go/1.24.4/libexec/bin:"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w\n%s", resolved.Command, strings.Join(resolved.ArgsFor(args...), " "), err, string(out))
	}
	return nil
}

func buildID(cfg config.Config) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", cfg.Path, cfg.Server.Port, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:])[:12]
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		return copyFile(src, filepath.Join(dst, filepath.Base(src)))
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		if !strings.HasSuffix(path, ".ts") {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
