package builder

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/Gode-Ts/gode-server/internal/config"
)

func GenerateWrapper(cfg config.Config, host string, port int) (string, error) {
	if host == "" {
		host = cfg.Server.Host
	}
	if port == 0 {
		port = cfg.Server.Port
	}
	if cfg.Framework == "gopress" {
		return generateGopressWrapper(host, port)
	}
	var b bytes.Buffer
	w := func(format string, args ...any) {
		if len(args) == 0 {
			b.WriteString(format)
		} else {
			fmt.Fprintf(&b, format, args...)
		}
		b.WriteByte('\n')
	}
	w("package main")
	w("")
	w("import (")
	w("%q", "net/http")
	w("runtime %q", "github.com/Gode-Ts/gode-runtime")
	w(")")
	w("")
	w("type GodeContext struct {")
	w("runtime.BaseContext")
	for _, field := range contextFieldNames(cfg) {
		w("%s string", tsFieldToGo(field))
	}
	w("}")
	w("")
	w("type GodeResponse = runtime.GodeResponse")
	w("type GodeMiddlewareResult = runtime.MiddlewareResultFor[GodeContext]")
	w("")
	w("var routes = []runtime.RouteFor[GodeContext]{")
	for _, route := range cfg.FlatRoutes() {
		middlewares := make([]string, 0, len(route.Middlewares))
		for _, middleware := range route.Middlewares {
			middlewares = append(middlewares, functionName(middleware))
		}
		w("{Method: %s, Pattern: %s, Handler: %s, Middlewares: []runtime.MiddlewareFuncFor[GodeContext]{%s}},", quote(route.Method), quote(route.FullPath), functionName(route.Handler), strings.Join(middlewares, ", "))
	}
	w("}")
	w("")
	w("func main() {")
	w("if err := runtime.ListenAndServeFor(runtime.ServerConfigFor[GodeContext]{")
	w("Host: %s,", quote(host))
	w("Port: %d,", port)
	w("BuildContext: buildGodeContext,")
	w("Routes: routes,")
	w("}); err != nil {")
	w("panic(err)")
	w("}")
	w("}")
	w("")
	w("func buildGodeContext(request *http.Request, params map[string]string) GodeContext {")
	w("base := runtime.BuildBaseContextWithOptions(request, params, runtime.ContextOptions{")
	w("RequestID: true,")
	w("Body: true,")
	if containsString(cfg.Server.Context.Query, "page") {
		w("QueryPage: true,")
	}
	if containsStringFold(cfg.Server.Context.Headers, "authorization") {
		w("HeaderAuthorization: true,")
	}
	if containsString(cfg.Server.Context.Cookies, "session") {
		w("CookieSession: true,")
	}
	w("})")
	w("ctx := GodeContext{BaseContext: base}")
	for _, route := range cfg.FlatRoutes() {
		for _, part := range strings.Split(route.FullPath, "/") {
			if strings.HasPrefix(part, ":") {
				name := strings.TrimPrefix(part, ":")
				if isBaseContextParam(name) {
					continue
				}
				w("ctx.%s = params[%q]", tsFieldToGo(cfg.ContextFieldForParam(name)), name)
			}
		}
	}
	if hasNonBaseQuery(cfg.Server.Context.Query) {
		w("query := request.URL.Query()")
	}
	for _, name := range cfg.Server.Context.Query {
		if isBaseContextQuery(name) {
			continue
		}
		w("ctx.%s = query.Get(%q)", tsFieldToGo(cfg.ContextFieldForQuery(name)), name)
	}
	for _, name := range cfg.Server.Context.Headers {
		if isBaseContextHeader(name) {
			continue
		}
		w("ctx.%s = request.Header.Get(%q)", tsFieldToGo(cfg.ContextFieldForHeader(name)), name)
	}
	nonBaseCookies := nonBaseCookieCount(cfg.Server.Context.Cookies)
	if nonBaseCookies > 1 {
		w("cookies := map[string]string{}")
		w("for _, cookie := range request.Cookies() { cookies[cookie.Name] = cookie.Value }")
	}
	for _, name := range cfg.Server.Context.Cookies {
		if isBaseContextCookie(name) {
			continue
		}
		if nonBaseCookies > 1 {
			w("ctx.%s = cookies[%q]", tsFieldToGo(cfg.ContextFieldForCookie(name)), name)
			continue
		}
		w("if cookie, err := request.Cookie(%q); err == nil { ctx.%s = cookie.Value }", name, tsFieldToGo(cfg.ContextFieldForCookie(name)))
	}
	w("return ctx")
	w("}")
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return b.String(), err
	}
	return string(formatted), nil
}

func generateGopressWrapper(host string, port int) (string, error) {
	var b bytes.Buffer
	w := func(format string, args ...any) {
		if len(args) == 0 {
			b.WriteString(format)
		} else {
			fmt.Fprintf(&b, format, args...)
		}
		b.WriteByte('\n')
	}
	w("package main")
	w("")
	w("import %q", "github.com/Gode-Ts/gopress")
	w("")
	w("func main() {")
	w("if err := gopress.ListenAndServe(gopress.ServerConfig{")
	w("Host: %s,", quote(host))
	w("Port: %d,", port)
	w("App: BuildGopressApp(),")
	w("}); err != nil {")
	w("panic(err)")
	w("}")
	w("}")
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return b.String(), err
	}
	return string(formatted), nil
}

func MiddlewareFunctionNames(cfg config.Config) []string {
	seen := map[string]bool{}
	for _, route := range cfg.FlatRoutes() {
		for _, name := range route.Middlewares {
			seen[name] = true
		}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsStringFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func isBaseContextParam(name string) bool {
	return name == "id"
}

func isBaseContextQuery(name string) bool {
	return name == "page"
}

func isBaseContextHeader(name string) bool {
	return strings.EqualFold(name, "authorization")
}

func isBaseContextCookie(name string) bool {
	return name == "session"
}

func hasNonBaseQuery(values []string) bool {
	for _, value := range values {
		if !isBaseContextQuery(value) {
			return true
		}
	}
	return false
}

func nonBaseCookieCount(values []string) int {
	count := 0
	for _, value := range values {
		if !isBaseContextCookie(value) {
			count++
		}
	}
	return count
}
