package runtime

import (
	"net/http"
	"strings"

	"github.com/Gode-Ts/gode-server/internal/config"
)

type Match struct {
	Route  config.FlatRoute
	Params map[string]string
}

type Router struct {
	routes []config.FlatRoute
}

func NewRouter(routes []config.FlatRoute) Router {
	return Router{routes: append([]config.FlatRoute{}, routes...)}
}

func (r Router) Match(req *http.Request) (Match, bool) {
	for _, route := range r.routes {
		if route.Method != req.Method {
			continue
		}
		params, ok := matchPattern(route.FullPath, req.URL.Path)
		if ok {
			return Match{Route: route, Params: params}, true
		}
	}
	return Match{}, false
}

func matchPattern(pattern, path string) (map[string]string, bool) {
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)
	if len(patternParts) != len(pathParts) {
		return nil, false
	}
	params := map[string]string{}
	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			params[strings.TrimPrefix(part, ":")] = pathParts[i]
			continue
		}
		if part != pathParts[i] {
			return nil, false
		}
	}
	return params, true
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
