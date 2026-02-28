package proxy

import (
	"fmt"

	"hydragate/internal/app"
)

type Routing struct {
	id        string
	Target    string
	Protected bool
	Transform app.TransformConfig
	Cache     *app.RouteCacheConfig
	CachePaths []app.CachePathOverride
}

type Registry struct {
	Routes map[string]Routing
}

func NewRegistry() *Registry {
	return &Registry{
		Routes: make(map[string]Routing),
	}
}

func (r *Registry) AddRoute(pathPrefix string, target string, protected bool, transform app.TransformConfig) {
	r.AddRouteWithCache(pathPrefix, target, protected, transform, nil, nil)
}

func (r *Registry) AddRouteWithCache(pathPrefix string, target string, protected bool, transform app.TransformConfig, cache *app.RouteCacheConfig, cachePaths []app.CachePathOverride) {
	r.Routes[pathPrefix] = Routing{
		id:         fmt.Sprintf("%d", len(r.Routes)+1),
		Target:     target,
		Protected:  protected,
		Transform:  transform,
		Cache:      cache,
		CachePaths: cachePaths,
	}
}

func (r *Registry) GetRoute(pathPrefix string) (Routing, bool) {
	route, ok := r.Routes[pathPrefix]
	return route, ok
}

func (r *Registry) LoadRoutes(configs []app.RouteConfig) {
	for _, c := range configs {
		r.AddRouteWithCache(c.Route, c.Target, c.Protected, c.Transform, c.Cache, c.CachePaths)
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
