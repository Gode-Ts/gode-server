package builder_test

import (
	"strings"
	"testing"

	"github.com/Gode-Ts/gode-server/internal/builder"
	"github.com/Gode-Ts/gode-server/internal/config"
)

func TestGeneratePreludeIncludesFlatContextFields(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Context.Headers = []string{"authorization"}
	cfg.Server.Context.Query = []string{"page"}
	cfg.Server.Context.Cookies = []string{"session"}
	cfg.Server.Resources = map[string]config.Resource{
		"users": {Prefix: "/users", Routes: []config.Route{{Method: "GET", Path: "/:id", Handler: "getUser"}}},
	}

	got := builder.GeneratePrelude(cfg)
	for _, want := range []string{"paramId: string", "queryPage: string", "headerAuthorization: string", "cookieSession: string"} {
		if !strings.Contains(got, want) {
			t.Fatalf("prelude missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateWrapperContainsMiddlewareOrderAndHandlerCall(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Middlewares = []string{"requestLogger"}
	cfg.Server.Resources = map[string]config.Resource{
		"users": {
			Prefix:      "/users",
			Middlewares: []string{"requireAuth"},
			Routes: []config.Route{{
				Method:      "GET",
				Path:        "/:id",
				Handler:     "getUser",
				Middlewares: []string{"auditUserRead"},
			}},
		},
	}
	src, err := builder.GenerateWrapper(cfg, "127.0.0.1", 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"func main()", `Handler: GetUser`, `Middlewares: []middlewareFunc{RequestLogger, RequireAuth, AuditUserRead}`, `"/__gode/ready"`} {
		if !strings.Contains(src, want) {
			t.Fatalf("wrapper missing %q:\n%s", want, src)
		}
	}
}
