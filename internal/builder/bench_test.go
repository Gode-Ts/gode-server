package builder_test

import (
	"testing"

	"github.com/Gode-Ts/gode-server/internal/builder"
	"github.com/Gode-Ts/gode-server/internal/config"
)

func BenchmarkGeneratePrelude(b *testing.B) {
	cfg := config.Default()
	cfg.Server.Context.Headers = []string{"authorization"}
	cfg.Server.Context.Query = []string{"page"}
	cfg.Server.Context.Cookies = []string{"session"}
	cfg.Server.Resources = map[string]config.Resource{
		"users": {Prefix: "/users", Routes: []config.Route{{Method: "GET", Path: "/:id", Handler: "getUser"}}},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = builder.GeneratePrelude(cfg)
	}
}

func BenchmarkGenerateGopressWrapper(b *testing.B) {
	cfg := config.Default()
	cfg.Framework = "gopress"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := builder.GenerateWrapper(cfg, "127.0.0.1", 3000); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateWorkerGoModGopress(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = builder.GenerateWorkerGoMod("/tmp/gode/wrapper", "/tmp/gode/app", "gopress")
	}
}
