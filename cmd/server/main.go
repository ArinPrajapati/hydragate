package main

import (
	"fmt"
	"log"
	"net/http"

	"hydragate/internal/middleware"
	"hydragate/internal/proxy"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello, %s!", r.URL.Path[1:])
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func main() {
	reg := proxy.NewRegistry()
	reg.AddRoute("api", "http://localhost:9000")
	http.Handle("/health", middleware.Chain(http.HandlerFunc(handlerHealth), middleware.Logger))
	http.Handle("/", middleware.Chain(http.HandlerFunc(proxy.Forward(reg)), middleware.Logger))
	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
