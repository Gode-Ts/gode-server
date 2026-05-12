package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Path    string `json:"-"`
	RootDir string `json:"-"`

	Entry      string `json:"entry"`
	Out        string `json:"out"`
	Package    string `json:"package"`
	Framework  string `json:"framework"`
	Server     Server `json:"server"`
	Build      Build  `json:"build"`
	knownNames map[string]bool
}

type Server struct {
	Host        string              `json:"host"`
	Port        int                 `json:"port"`
	Middlewares []string            `json:"middlewares"`
	Context     Context             `json:"context"`
	Resources   map[string]Resource `json:"resources"`
}

type Context struct {
	Headers []string `json:"headers"`
	Query   []string `json:"query"`
	Cookies []string `json:"cookies"`
}

type Resource struct {
	Prefix      string   `json:"prefix"`
	Middlewares []string `json:"middlewares"`
	Routes      []Route  `json:"routes"`
}

type Route struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Handler     string   `json:"handler"`
	Middlewares []string `json:"middlewares"`
}

type FlatRoute struct {
	Resource    string
	Method      string
	Path        string
	FullPath    string
	Handler     string
	Middlewares []string
}

type Build struct {
	WorkDir         string `json:"workDir"`
	BinDir          string `json:"binDir"`
	DistDir         string `json:"distDir"`
	ReloadDebounce  string `json:"reloadDebounce"`
	StartupTimeout  string `json:"startupTimeout"`
	ShutdownTimeout string `json:"shutdownTimeout"`
}

func Default() Config {
	return Config{
		Entry:   "./src",
		Out:     "./.gode/gen",
		Package: "app",
		Server: Server{
			Host:      "127.0.0.1",
			Port:      3000,
			Context:   Context{},
			Resources: map[string]Resource{},
		},
		Build: Build{
			WorkDir:         "./.gode/work",
			BinDir:          "./.gode/bin",
			DistDir:         "./dist",
			ReloadDebounce:  "250ms",
			StartupTimeout:  "5s",
			ShutdownTimeout: "10s",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = "gode.json"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.Path = abs
	cfg.RootDir = filepath.Dir(abs)
	cfg.applyDefaults()
	cfg.knownNames = scanExportedFunctionNames(filepath.Join(cfg.RootDir, cfg.Entry))
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	d := Default()
	if c.Entry == "" {
		c.Entry = d.Entry
	}
	if c.Out == "" {
		c.Out = d.Out
	}
	if c.Package == "" {
		c.Package = d.Package
	}
	if c.Server.Host == "" {
		c.Server.Host = d.Server.Host
	}
	if c.Server.Port == 0 {
		c.Server.Port = d.Server.Port
	}
	if c.Server.Resources == nil {
		c.Server.Resources = map[string]Resource{}
	}
	if c.Build.WorkDir == "" {
		c.Build.WorkDir = d.Build.WorkDir
	}
	if c.Build.BinDir == "" {
		c.Build.BinDir = d.Build.BinDir
	}
	if c.Build.DistDir == "" {
		c.Build.DistDir = d.Build.DistDir
	}
	if c.Build.ReloadDebounce == "" {
		c.Build.ReloadDebounce = d.Build.ReloadDebounce
	}
	if c.Build.StartupTimeout == "" {
		c.Build.StartupTimeout = d.Build.StartupTimeout
	}
	if c.Build.ShutdownTimeout == "" {
		c.Build.ShutdownTimeout = d.Build.ShutdownTimeout
	}
}

func (c Config) Validate() error {
	var errs []string
	if c.Entry == "" {
		errs = append(errs, "entry is required")
	}
	if c.Framework != "" && c.Framework != "gode" && c.Framework != "gopress" {
		errs = append(errs, "framework must be one of: gode, gopress")
	}
	if c.Server.Port < 0 || c.Server.Port > 65535 {
		errs = append(errs, "server.port must be between 0 and 65535")
	}
	if _, err := time.ParseDuration(c.Build.ReloadDebounce); err != nil {
		errs = append(errs, "build.reloadDebounce must be a duration")
	}
	if _, err := time.ParseDuration(c.Build.StartupTimeout); err != nil {
		errs = append(errs, "build.startupTimeout must be a duration")
	}
	if _, err := time.ParseDuration(c.Build.ShutdownTimeout); err != nil {
		errs = append(errs, "build.shutdownTimeout must be a duration")
	}
	if c.Framework == "gopress" {
		if len(errs) > 0 {
			sort.Strings(errs)
			return errors.New(strings.Join(errs, "; "))
		}
		return nil
	}
	seenRoutes := map[string]bool{}
	for resourceName, resource := range c.Server.Resources {
		if !strings.HasPrefix(resource.Prefix, "/") {
			errs = append(errs, fmt.Sprintf("resource %q prefix must start with /", resourceName))
		}
		for _, route := range resource.Routes {
			method := strings.ToUpper(route.Method)
			if !validHTTPMethods[method] {
				errs = append(errs, fmt.Sprintf("route %s %s has invalid method", route.Method, route.Path))
			}
			if route.Handler == "" {
				errs = append(errs, fmt.Sprintf("route %s %s missing handler", route.Method, route.Path))
			}
			if !isIdentifier(route.Handler) {
				errs = append(errs, fmt.Sprintf("route %s %s handler %q is not a valid identifier", route.Method, route.Path, route.Handler))
			}
			if len(c.knownNames) > 0 && !c.knownNames[route.Handler] {
				errs = append(errs, fmt.Sprintf("missing handler %q", route.Handler))
			}
			key := method + " " + joinPath(resource.Prefix, route.Path)
			if seenRoutes[key] {
				errs = append(errs, "duplicate route "+key)
			}
			seenRoutes[key] = true
			for _, name := range append(append([]string{}, c.Server.Middlewares...), append(resource.Middlewares, route.Middlewares...)...) {
				if !isIdentifier(name) {
					errs = append(errs, fmt.Sprintf("middleware %q is not a valid identifier", name))
				}
				if len(c.knownNames) > 0 && !c.knownNames[name] {
					errs = append(errs, "unknown middleware "+name)
				}
			}
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (c Config) WithKnownNames(names ...string) Config {
	c.knownNames = map[string]bool{}
	for _, name := range names {
		c.knownNames[name] = true
	}
	return c
}

func (c Config) FlatRoutes() []FlatRoute {
	var routes []FlatRoute
	resourceNames := make([]string, 0, len(c.Server.Resources))
	for name := range c.Server.Resources {
		resourceNames = append(resourceNames, name)
	}
	sort.Strings(resourceNames)
	for _, name := range resourceNames {
		resource := c.Server.Resources[name]
		for _, route := range resource.Routes {
			middlewares := append([]string{}, c.Server.Middlewares...)
			middlewares = append(middlewares, resource.Middlewares...)
			middlewares = append(middlewares, route.Middlewares...)
			routes = append(routes, FlatRoute{
				Resource:    name,
				Method:      strings.ToUpper(route.Method),
				Path:        route.Path,
				FullPath:    joinPath(resource.Prefix, route.Path),
				Handler:     route.Handler,
				Middlewares: middlewares,
			})
		}
	}
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].FullPath != routes[j].FullPath {
			return routes[i].FullPath < routes[j].FullPath
		}
		return routes[i].Method < routes[j].Method
	})
	return routes
}

func (c Config) ContextFieldForHeader(name string) string {
	return "header" + exportedName(canonicalKey(name))
}

func (c Config) ContextFieldForQuery(name string) string {
	return "query" + exportedName(canonicalKey(name))
}

func (c Config) ContextFieldForCookie(name string) string {
	return "cookie" + exportedName(canonicalKey(name))
}

func (c Config) ContextFieldForParam(name string) string {
	return "param" + exportedName(canonicalKey(name))
}

func (c Config) Abs(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	root := c.RootDir
	if root == "" {
		root = "."
	}
	return filepath.Join(root, path)
}

func (c Config) ReloadDebounce() time.Duration {
	d, err := time.ParseDuration(c.Build.ReloadDebounce)
	if err != nil {
		return 250 * time.Millisecond
	}
	return d
}

func (c Config) StartupTimeout() time.Duration {
	d, err := time.ParseDuration(c.Build.StartupTimeout)
	if err != nil {
		return 5 * time.Second
	}
	return d
}

func (c Config) ShutdownTimeout() time.Duration {
	d, err := time.ParseDuration(c.Build.ShutdownTimeout)
	if err != nil {
		return 10 * time.Second
	}
	return d
}

func joinPath(prefix, path string) string {
	prefix = "/" + strings.Trim(prefix, "/")
	path = "/" + strings.Trim(path, "/")
	full := strings.TrimRight(prefix, "/") + path
	if full == "" {
		return "/"
	}
	return strings.ReplaceAll(full, "//", "/")
}

var validHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true, "HEAD": true,
}

var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func isIdentifier(value string) bool {
	return identRE.MatchString(value)
}

func canonicalKey(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == ' ' || r == ':' || r == '/'
	})
	for i, part := range parts {
		parts[i] = strings.ToLower(part)
	}
	return strings.Join(parts, "_")
}

func exportedName(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func scanExportedFunctionNames(entry string) map[string]bool {
	names := map[string]bool{}
	info, err := os.Stat(entry)
	if err != nil {
		return names
	}
	visit := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		re := regexp.MustCompile(`export\s+(?:async\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)`)
		for _, match := range re.FindAllStringSubmatch(string(data), -1) {
			names[match[1]] = true
		}
	}
	if !info.IsDir() {
		visit(entry)
		return names
	}
	_ = filepath.WalkDir(entry, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".ts") {
			return nil
		}
		visit(path)
		return nil
	})
	return names
}
