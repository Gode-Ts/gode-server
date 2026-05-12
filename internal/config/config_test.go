package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gode-Ts/gode-server/internal/config"
)

func TestLoadValidatesManifestAndFlattensRoutes(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "gode.json")
	err := os.WriteFile(manifest, []byte(`{
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
	}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(manifest)
	if err != nil {
		t.Fatal(err)
	}
	routes := cfg.FlatRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected one route, got %d", len(routes))
	}
	route := routes[0]
	if route.Method != "GET" || route.FullPath != "/users/:id" || route.Handler != "getUser" {
		t.Fatalf("unexpected route: %+v", route)
	}
	if got := strings.Join(route.Middlewares, ","); got != "requestLogger,requireAuth,auditUserRead" {
		t.Fatalf("unexpected middleware order: %s", got)
	}
	if cfg.ContextFieldForHeader("authorization") != "headerAuthorization" {
		t.Fatalf("unexpected header field")
	}
}

func TestValidateRejectsUnknownMiddlewareAndDuplicateRoute(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Middlewares = []string{"missingMiddleware"}
	cfg.Server.Resources = map[string]config.Resource{
		"users": {
			Prefix: "/users",
			Routes: []config.Route{
				{Method: "GET", Path: "/:id", Handler: "getUser"},
				{Method: "GET", Path: "/:id", Handler: "getUserAgain"},
			},
		},
	}
	cfg = cfg.WithKnownNames("getUser", "getUserAgain")
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	} else if !strings.Contains(err.Error(), "duplicate route") || !strings.Contains(err.Error(), "unknown middleware") {
		t.Fatalf("expected duplicate route and unknown middleware errors, got %v", err)
	}
}

func TestLoadGopressManifestDoesNotRequireResourceRoutes(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "app.ts"), []byte(`import gopress from "gopress"
const app = gopress()
export default app
`), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, "gode.json")
	if err := os.WriteFile(manifest, []byte(`{
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
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Framework != "gopress" {
		t.Fatalf("expected gopress framework, got %q", cfg.Framework)
	}
	if len(cfg.FlatRoutes()) != 0 {
		t.Fatalf("gopress manifest should not require manifest routes: %+v", cfg.FlatRoutes())
	}
}
