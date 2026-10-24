package api

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type routeOperation struct {
	method string
	path   string
}

func TestOpenAPICoversRegisteredRoutes(t *testing.T) {
	registered := make(map[routeOperation]struct{})
	for _, source := range []struct {
		path    string
		methods []string
	}{
		{path: "api.go", methods: []string{"HandleFunc"}},
		{path: "../../cmd/autograder/main.go", methods: []string{"Handle"}},
	} {
		for route := range registeredRoutes(t, source.path, source.methods...) {
			registered[route] = struct{}{}
		}
	}

	document, err := os.ReadFile("../../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI document: %v", err)
	}
	var spec struct {
		Servers []struct {
			URL string `yaml:"url"`
		} `yaml:"servers"`
		Paths map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(document, &spec); err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}

	documented := make(map[routeOperation]struct{})
	for routePath, item := range spec.Paths {
		if item.Kind != yaml.MappingNode {
			t.Fatalf("OpenAPI path %q is not an object", routePath)
		}
		for i := 0; i < len(item.Content); i += 2 {
			method := strings.ToUpper(item.Content[i].Value)
			if !isOpenAPIMethod(method) {
				continue
			}
			var operation struct {
				Servers []struct {
					URL string `yaml:"url"`
				} `yaml:"servers"`
			}
			if err := item.Content[i+1].Decode(&operation); err != nil {
				t.Fatalf("parse %s %s operation: %v", method, routePath, err)
			}
			servers := spec.Servers
			if len(operation.Servers) != 0 {
				servers = operation.Servers
			}
			if len(servers) != 1 {
				t.Fatalf("%s %s must resolve through exactly one server", method, routePath)
			}
			fullPath, err := joinServerPath(servers[0].URL, routePath)
			if err != nil {
				t.Fatalf("resolve %s %s server: %v", method, routePath, err)
			}
			route := routeOperation{method: method, path: fullPath}
			if _, exists := documented[route]; exists {
				t.Fatalf("duplicate OpenAPI operation %s %s", method, fullPath)
			}
			documented[route] = struct{}{}
		}
	}

	var missing, extra []string
	for route := range registered {
		if _, exists := documented[route]; !exists {
			missing = append(missing, fmt.Sprintf("%s %s", route.method, route.path))
		}
	}
	for route := range documented {
		if _, exists := registered[route]; !exists {
			extra = append(extra, fmt.Sprintf("%s %s", route.method, route.path))
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) != 0 || len(extra) != 0 {
		t.Fatalf("OpenAPI/router operation mismatch: router=%d OpenAPI=%d; missing=%v extra=%v",
			len(registered), len(documented), missing, extra)
	}
}

func registeredRoutes(t *testing.T, filename string, methods ...string) map[routeOperation]struct{} {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	if err != nil {
		t.Fatalf("parse router source %q: %v", filename, err)
	}
	selected := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		selected[method] = struct{}{}
	}
	routes := make(map[routeOperation]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if _, ok := selected[selector.Sel.Name]; !ok {
			return true
		}
		pattern, ok := call.Args[0].(*ast.BasicLit)
		if !ok || pattern.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(pattern.Value)
		if err != nil {
			t.Errorf("unquote router pattern %s: %v", pattern.Value, err)
			return true
		}
		fields := strings.Fields(value)
		if len(fields) != 2 || !isOpenAPIMethod(fields[0]) {
			return true
		}
		routes[routeOperation{method: fields[0], path: fields[1]}] = struct{}{}
		return true
	})
	return routes
}

func isOpenAPIMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE":
		return true
	default:
		return false
	}
}

func joinServerPath(serverURL, routePath string) (string, error) {
	server, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(server.Path, "/")
	return base + "/" + strings.TrimLeft(routePath, "/"), nil
}
