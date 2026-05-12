package compiler

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

type Command struct {
	Command string
	Args    []string
	Dir     string
}

type ResolveOptions struct {
	Explicit   string
	RootDir    string
	LookupPath func(string) (string, error)
}

func Resolve(opts ResolveOptions) (Command, error) {
	if opts.Explicit != "" {
		return Command{Command: opts.Explicit}, nil
	}
	if env := os.Getenv("GODEC"); env != "" {
		return Command{Command: env}, nil
	}
	lookup := opts.LookupPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	if path, err := lookup("godec"); err == nil && path != "" {
		return Command{Command: path}, nil
	}
	root := opts.RootDir
	if root == "" {
		root = "."
	}
	for _, base := range uniqueRoots(root, ".", "..") {
		local := filepath.Join(base, "gode-compiler")
		if _, err := os.Stat(filepath.Join(local, "go.mod")); err == nil {
			return Command{Command: "go", Args: []string{"run", "./cmd/godec"}, Dir: local}, nil
		}
	}
	return Command{}, errors.New("godec not found; pass --compiler, set GODEC, install godec in PATH, or keep ./gode-compiler available")
}

func (c Command) ArgsFor(extra ...string) []string {
	args := append([]string{}, c.Args...)
	args = append(args, extra...)
	return args
}

func uniqueRoots(roots ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			abs = root
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}
