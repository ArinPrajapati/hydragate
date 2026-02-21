package app

type RouteConfig struct {
	Route     string `json:"route"`
	Target    string `json:"target"`
	Protected bool   `json:"protected"`
}

type GatewayConfig struct {
	JWTSecret     string            `json:"jwt_secret"`
	ForwardClaims map[string]string `json:"forward_claims"`
	APIKeys       map[string]string `json:"api_keys"` // later API will hanlded from dashbaord intead of config file
	Routes        []RouteConfig     `json:"routes"`
}
