package proxy

import (
	"fmt"

	"hydragate/internal/app"
)

type Routing struct {
	id        string
	Target    string
	Protected bool
}

type Registry struct {
	Routes map[string]Routing
}

func NewRegistry() *Registry {
	return &Registry{
		Routes: make(map[string]Routing),
	}
}

func (r *Registry) AddRoute(pathPrefix string, target string, protected bool) {
	r.Routes[pathPrefix] = Routing{
		id:        fmt.Sprintf("%d", len(r.Routes)+1),
		Target:    target,
		Protected: protected,
	}
}

func (r *Registry) GetRoute(pathPrefix string) (Routing, bool) {
	route, ok := r.Routes[pathPrefix]
	return route, ok
}

func (r *Registry) LoadRoutes(configs []app.RouteConfig) {
	for _, c := range configs {
		r.AddRoute(c.Route, c.Target, c.Protected)
	}
}

func (r *Registry) ProtectedRoutes() map[string]bool {
	result := make(map[string]bool)
	for path, route := range r.Routes {
		if route.Protected {
			result[path] = true
		}
	}
	return result
}
