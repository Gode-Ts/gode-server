package builder

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Gode-Ts/gode-server/internal/config"
)

func GeneratePrelude(cfg config.Config) string {
	fields := []string{
		"requestId: string",
		"method: string",
		"path: string",
		"body: string",
	}
	seen := map[string]bool{}
	add := func(field string) {
		if seen[field] {
			return
		}
		seen[field] = true
		fields = append(fields, field+": string")
	}
	for _, route := range cfg.FlatRoutes() {
		for _, part := range strings.Split(route.FullPath, "/") {
			if strings.HasPrefix(part, ":") {
				add(cfg.ContextFieldForParam(strings.TrimPrefix(part, ":")))
			}
		}
	}
	for _, name := range cfg.Server.Context.Query {
		add(cfg.ContextFieldForQuery(name))
	}
	for _, name := range cfg.Server.Context.Headers {
		add(cfg.ContextFieldForHeader(name))
	}
	for _, name := range cfg.Server.Context.Cookies {
		add(cfg.ContextFieldForCookie(name))
	}
	var b strings.Builder
	b.WriteString("declare type GodeContext = {\n")
	for _, field := range fields {
		b.WriteString("  ")
		b.WriteString(field)
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
	b.WriteString(`declare type GodeResponse = {
  status: number
  body: string
  contentType: string
}

declare type GodeMiddlewareResult = {
  continue: boolean
  ctx: GodeContext
  status: number
  body: string
  contentType: string
}
`)
	return b.String()
}

func contextFieldNames(cfg config.Config) []string {
	var fields []string
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			fields = append(fields, name)
			seen[name] = true
		}
	}
	for _, route := range cfg.FlatRoutes() {
		for _, part := range strings.Split(route.FullPath, "/") {
			if strings.HasPrefix(part, ":") {
				add(cfg.ContextFieldForParam(strings.TrimPrefix(part, ":")))
			}
		}
	}
	for _, name := range cfg.Server.Context.Query {
		add(cfg.ContextFieldForQuery(name))
	}
	for _, name := range cfg.Server.Context.Headers {
		add(cfg.ContextFieldForHeader(name))
	}
	for _, name := range cfg.Server.Context.Cookies {
		add(cfg.ContextFieldForCookie(name))
	}
	sort.Strings(fields)
	return fields
}

func tsFieldToGo(name string) string {
	if name == "" {
		return name
	}
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	if len(parts) == 0 {
		parts = []string{name}
	}
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func functionName(name string) string {
	return tsFieldToGo(name)
}

func quote(value string) string {
	return fmt.Sprintf("%q", value)
}
