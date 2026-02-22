package app

type RouteConfig struct {
	Route     string `json:"route"`
	Target    string `json:"target"`
	Protected bool   `json:"protected"`
}

type RateLimitConfig struct {
	Enabled    bool `json:"enabled"`
	Capacity   int  `json:"capacity"`
	RefillRate int  `json:"refill_rate"`
}

type GatewayConfig struct {
	JWTSecret     string            `json:"jwt_secret"`
	ForwardClaims map[string]string `json:"forward_claims"`
	APIKeys       map[string]string `json:"api_keys"` // later API will hanlded from dashbaord intead of config file
	RateLimit     RateLimitConfig   `json:"rate_limit"`
	Routes        []RouteConfig     `json:"routes"`
}
