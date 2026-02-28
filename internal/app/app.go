package app

type TransformConfig struct {
	AddHeaders    map[string]string `json:"add_headers,omitempty"`
	RemoveHeaders []string          `json:"remove_headers,omitempty"`
	PathRewrite   string            `json:"path_rewrite,omitempty"`
}

type RouteConfig struct {
	Route     string          `json:"route"`
	Target    string          `json:"target"`
	Protected bool            `json:"protected"`
	Transform TransformConfig `json:"transform"`
}

type RateLimitConfig struct {
	Enabled    bool `json:"enabled"`
	Capacity   int  `json:"capacity"`
	RefillRate int  `json:"refill_rate"`
}

type GatewayConfig struct {
	JWTSecret     string            `json:"jwt_secret"`
	ForwardClaims map[string]string `json:"forward_claims"`
	APIKeys       map[string]string `json:"api_keys"`
	RateLimit     RateLimitConfig   `json:"rate_limit"`
	Routes        []RouteConfig     `json:"routes"`
}
