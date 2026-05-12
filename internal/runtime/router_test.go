package runtime_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Gode-Ts/gode-server/internal/config"
	gruntime "github.com/Gode-Ts/gode-server/internal/runtime"
)

func TestRouteMatcherExtractsGeneratedContextFields(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Context.Query = []string{"page"}
	cfg.Server.Context.Headers = []string{"authorization"}
	cfg.Server.Context.Cookies = []string{"session"}
	cfg.Server.Resources = map[string]config.Resource{
		"users": {
			Prefix: "/users",
			Routes: []config.Route{{Method: "GET", Path: "/:id", Handler: "getUser"}},
		},
	}
	router := gruntime.NewRouter(cfg.FlatRoutes())
	req := httptest.NewRequest(http.MethodGet, "/users/123?page=2", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})

	match, ok := router.Match(req)
	if !ok {
		t.Fatal("expected route match")
	}
	if match.Route.Handler != "getUser" {
		t.Fatalf("unexpected handler %s", match.Route.Handler)
	}
	if match.Params["id"] != "123" {
		t.Fatalf("expected id param, got %+v", match.Params)
	}
}
