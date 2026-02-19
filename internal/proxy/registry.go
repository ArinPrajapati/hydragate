package proxy

import "fmt"

type Routing struct {
	id     string
	Target string
}

type Registry struct {
	Routes map[string]Routing
}

func NewRegistry() *Registry {
	return &Registry{
		Routes: make(map[string]Routing),
	}
}

func (r *Registry) AddRoute(pathPrefix string, target string) {
	r.Routes[pathPrefix] = Routing{
		id:     fmt.Sprintf("%d", len(r.Routes)+1),
		Target: target,
	}
}

func (r *Registry) GetRoute(pathPrefix string) (Routing, bool) {
	route, ok := r.Routes[pathPrefix]
	return route, ok
}
