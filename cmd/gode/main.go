package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/Gode-Ts/gode-server/internal/builder"
	"github.com/Gode-Ts/gode-server/internal/config"
	"github.com/Gode-Ts/gode-server/internal/supervisor"
)

const version = "0.1.7"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: gode <init|check|build|dev|serve|version>")
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "gode %s\n", version)
		return 0
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "dev":
		return runDev(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gode init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "project directory")
	framework := fs.String("framework", "gode", "project framework: gode or gopress")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	manifest := starterManifest
	app := starterApp
	switch *framework {
	case "", "gode":
	case "gopress":
		manifest = gopressStarterManifest
		app = gopressStarterApp
	default:
		fmt.Fprintf(stderr, "init failed: unsupported framework %q\n", *framework)
		return 2
	}
	if err := os.MkdirAll(filepath.Join(*dir, "src"), 0o755); err != nil {
		fmt.Fprintf(stderr, "init failed: %v\n", err)
		return 1
	}
	files := map[string]string{
		filepath.Join(*dir, "gode.json"):     manifest,
		filepath.Join(*dir, "src", "app.ts"): app,
	}
	for path, content := range files {
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(stderr, "init failed: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "created Gode project in %s\n", *dir)
	return 0
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gode check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "gode.json", "config path")
	compilerPath := fs.String("compiler", "", "godec path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return 1
	}
	if err := builder.Check(context.Background(), cfg, *compilerPath); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "gode check passed")
	return 0
}

func runBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gode build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "gode.json", "config path")
	output := fs.String("output", "", "output binary")
	compilerPath := fs.String("compiler", "", "godec path")
	host := fs.String("host", "", "server host")
	port := fs.Int("port", 0, "server port")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	result, err := builder.Build(context.Background(), builder.Options{ConfigPath: *cfgPath, Output: *output, Compiler: *compilerPath, Host: *host, Port: *port})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "built %s\n", result.BinaryPath)
	return 0
}

func runDev(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gode dev", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "gode.json", "config path")
	compilerPath := fs.String("compiler", "", "godec path")
	host := fs.String("host", "", "server host")
	port := fs.Int("port", 0, "server port")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := supervisor.RunDev(context.Background(), supervisor.Options{ConfigPath: *cfgPath, Compiler: *compilerPath, Host: *host, Port: *port, Stdout: stdout, Stderr: stderr}); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gode serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bin := fs.String("bin", "", "binary to run")
	host := fs.String("host", "", "server host")
	port := fs.Int("port", 0, "server port")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *bin == "" {
		fmt.Fprintln(stderr, "missing --bin")
		return 2
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, *bin)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	env := os.Environ()
	if *host != "" {
		env = append(env, "GODE_HOST="+*host)
	}
	if *port != 0 {
		env = append(env, "GODE_PORT="+strconv.Itoa(*port))
	}
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

const starterManifest = `{
  "entry": "./src",
  "out": "./.gode/gen",
  "package": "app",
  "server": {
    "host": "127.0.0.1",
    "port": 3000,
    "middlewares": ["requestLogger"],
    "context": {
      "headers": ["authorization"],
      "query": ["page"],
      "cookies": ["session"]
    },
    "resources": {
      "users": {
        "prefix": "/users",
        "middlewares": ["requireAuth"],
        "routes": [
          {
            "method": "GET",
            "path": "/:id",
            "handler": "getUser",
            "middlewares": ["auditUserRead"]
          }
        ]
      }
    }
  },
  "build": {
    "workDir": "./.gode/work",
    "binDir": "./.gode/bin",
    "distDir": "./dist",
    "reloadDebounce": "250ms",
    "startupTimeout": "5s",
    "shutdownTimeout": "10s"
  }
}
`

const starterApp = `export async function requestLogger(ctx: GodeContext): Promise<GodeMiddlewareResult> {
  return { continue: true, ctx }
}

export async function requireAuth(ctx: GodeContext): Promise<GodeMiddlewareResult> {
  if (ctx.headerAuthorization == "") {
    return { continue: false, ctx, status: 401, body: "unauthorized", contentType: "text/plain" }
  }

  return { continue: true, ctx }
}

export async function auditUserRead(ctx: GodeContext): Promise<GodeMiddlewareResult> {
  return { continue: true, ctx }
}

export async function getUser(ctx: GodeContext): Promise<GodeResponse> {
  return { status: 200, body: ctx.paramId, contentType: "text/plain" }
}
`

const gopressStarterManifest = `{
  "framework": "gopress",
  "entry": "./src",
  "out": "./.gode/gen",
  "package": "app",
  "server": {
    "host": "127.0.0.1",
    "port": 3000
  },
  "build": {
    "workDir": "./.gode/work",
    "binDir": "./.gode/bin",
    "distDir": "./dist",
    "reloadDebounce": "250ms",
    "startupTimeout": "5s",
    "shutdownTimeout": "10s"
  }
}
`

const gopressStarterApp = `import gopress, { Request, Response } from "gopress"

const app = gopress()

app.use(gopress.json())

app.get("/users/:id", async (req: Request, res: Response) => {
  res.status(200).send(req.params.id)
})

export default app
`
