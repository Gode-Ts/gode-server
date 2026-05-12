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
	for _, imp := range []string{"context", "crypto/rand", "encoding/hex", "io", "net", "net/http", "os", "os/signal", "strconv", "strings", "syscall", "time"} {
		w("%q", imp)
	}
	w(")")
	w("")
	w("type handlerFunc func(context.Context, GodeContext) (GodeResponse, error)")
	w("type middlewareFunc func(context.Context, GodeContext) (GodeMiddlewareResult, error)")
	w("type routeSpec struct { Method string; Pattern string; Handler handlerFunc; Middlewares []middlewareFunc }")
	w("type routeMatch struct { Route routeSpec; Params map[string]string }")
	w("")
	w("var routes = []routeSpec{")
	for _, route := range cfg.FlatRoutes() {
		middlewares := make([]string, 0, len(route.Middlewares))
		for _, middleware := range route.Middlewares {
			middlewares = append(middlewares, functionName(middleware))
		}
		w("{Method: %s, Pattern: %s, Handler: %s, Middlewares: []middlewareFunc{%s}},", quote(route.Method), quote(route.FullPath), functionName(route.Handler), strings.Join(middlewares, ", "))
	}
	w("}")
	w("")
	w("func main() {")
	w("addr := os.Getenv(%q)", "GODE_WORKER_ADDR")
	w("if addr == \"\" {")
	w("host := envOr(%q, %s)", "GODE_HOST", quote(host))
	w("port := envOr(%q, %s)", "GODE_PORT", quote(fmt.Sprintf("%d", port)))
	w("addr = host + \":\" + port")
	w("}")
	w("listener, err := net.Listen(\"tcp\", addr)")
	w("if err != nil { panic(err) }")
	w("server := &http.Server{Handler: http.HandlerFunc(handleRequest), ReadHeaderTimeout: 10 * time.Second}")
	w("if handshake := os.Getenv(%q); handshake != \"\" { _ = os.WriteFile(handshake, []byte(listener.Addr().String()), 0o644) }", "GODE_WORKER_HANDSHAKE")
	w("go func() {")
	w("sigCh := make(chan os.Signal, 1)")
	w("signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)")
	w("<-sigCh")
	w("ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)")
	w("defer cancel()")
	w("_ = server.Shutdown(ctx)")
	w("}()")
	w("if err := server.Serve(listener); err != nil && err != http.ErrServerClosed { panic(err) }")
	w("}")
	w("")
	w(runtimeSupportSource(cfg))
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return b.String(), err
	}
	return string(formatted), nil
}

func runtimeSupportSource(cfg config.Config) string {
	var b strings.Builder
	w := func(format string, args ...any) {
		if len(args) == 0 {
			b.WriteString(format)
		} else {
			fmt.Fprintf(&b, format, args...)
		}
		b.WriteByte('\n')
	}
	w("func handleRequest(writer http.ResponseWriter, request *http.Request) {")
	w("if request.URL.Path == %q { writer.WriteHeader(http.StatusOK); _, _ = writer.Write([]byte(%q)); return }", "/__gode/ready", "ok")
	w("match, ok := matchRoute(request.Method, request.URL.Path)")
	w("if !ok { http.NotFound(writer, request); return }")
	w("godeCtx := buildContext(request, match.Params)")
	w("for _, middleware := range match.Route.Middlewares {")
	w("result, err := middleware(request.Context(), godeCtx)")
	w("if err != nil { writeError(writer, err); return }")
	w("if result.Ctx.RequestId != \"\" { godeCtx = result.Ctx }")
	w("if !result.Continue { writeGodeResponse(writer, GodeResponse{Status: result.Status, Body: result.Body, ContentType: result.ContentType}); return }")
	w("}")
	w("response, err := match.Route.Handler(request.Context(), godeCtx)")
	w("if err != nil { writeError(writer, err); return }")
	w("writeGodeResponse(writer, response)")
	w("}")
	w("")
	w("func matchRoute(method string, path string) (routeMatch, bool) {")
	w("for _, route := range routes {")
	w("if route.Method != method { continue }")
	w("params, ok := matchPattern(route.Pattern, path)")
	w("if ok { return routeMatch{Route: route, Params: params}, true }")
	w("}")
	w("return routeMatch{}, false")
	w("}")
	w("")
	w("func matchPattern(pattern string, path string) (map[string]string, bool) {")
	w("patternParts := splitPath(pattern)")
	w("pathParts := splitPath(path)")
	w("if len(patternParts) != len(pathParts) { return nil, false }")
	w("params := map[string]string{}")
	w("for i, part := range patternParts {")
	w("if strings.HasPrefix(part, \":\") { params[strings.TrimPrefix(part, \":\")] = pathParts[i]; continue }")
	w("if part != pathParts[i] { return nil, false }")
	w("}")
	w("return params, true")
	w("}")
	w("")
	w("func splitPath(path string) []string {")
	w("path = strings.Trim(path, \"/\")")
	w("if path == \"\" { return nil }")
	w("return strings.Split(path, \"/\")")
	w("}")
	w("")
	w("func buildContext(request *http.Request, params map[string]string) GodeContext {")
	w("body, _ := io.ReadAll(request.Body)")
	w("ctx := GodeContext{RequestId: newRequestID(), Method: request.Method, Path: request.URL.Path, Body: string(body)}")
	for _, route := range cfg.FlatRoutes() {
		for _, part := range strings.Split(route.FullPath, "/") {
			if strings.HasPrefix(part, ":") {
				name := strings.TrimPrefix(part, ":")
				w("ctx.%s = params[%q]", tsFieldToGo(cfg.ContextFieldForParam(name)), name)
			}
		}
	}
	for _, name := range cfg.Server.Context.Query {
		w("ctx.%s = request.URL.Query().Get(%q)", tsFieldToGo(cfg.ContextFieldForQuery(name)), name)
	}
	for _, name := range cfg.Server.Context.Headers {
		w("ctx.%s = request.Header.Get(%q)", tsFieldToGo(cfg.ContextFieldForHeader(name)), name)
	}
	for _, name := range cfg.Server.Context.Cookies {
		w("if cookie, err := request.Cookie(%q); err == nil { ctx.%s = cookie.Value }", name, tsFieldToGo(cfg.ContextFieldForCookie(name)))
	}
	w("return ctx")
	w("}")
	w("")
	w("func writeGodeResponse(writer http.ResponseWriter, response GodeResponse) {")
	w("status := int(response.Status)")
	w("if status == 0 { status = http.StatusOK }")
	w("contentType := response.ContentType")
	w("if contentType == \"\" { contentType = \"text/plain\" }")
	w("writer.Header().Set(\"Content-Type\", contentType)")
	w("writer.WriteHeader(status)")
	w("_, _ = writer.Write([]byte(response.Body))")
	w("}")
	w("")
	w("func writeError(writer http.ResponseWriter, err error) {")
	w("writer.Header().Set(\"Content-Type\", \"text/plain\")")
	w("writer.WriteHeader(http.StatusInternalServerError)")
	w("_, _ = writer.Write([]byte(err.Error()))")
	w("}")
	w("")
	w("func newRequestID() string {")
	w("var buf [8]byte")
	w("if _, err := rand.Read(buf[:]); err != nil { return strconv.FormatInt(time.Now().UnixNano(), 10) }")
	w("return hex.EncodeToString(buf[:])")
	w("}")
	w("")
	w("func envOr(name string, fallback string) string {")
	w("if value := os.Getenv(name); value != \"\" { return value }")
	w("return fallback")
	w("}")
	return b.String()
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
