package app

type RouteConfig struct {
	Route     string `json:"route"`
	Target    string `json:"target"`
	Protected bool   `json:"protected"`
}

type GatewayConfig struct {
	JWTSecret     string            `json:"jwt_secret"`
	ForwardClaims map[string]string `json:"forward_claims"`
	Routes        []RouteConfig     `json:"routes"`
}
