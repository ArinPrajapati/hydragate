package main

import (
	"fmt"
	"log"
	"net/http"

	"hydragate/internal/middleware"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello, %s!", r.URL.Path[1:])
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Alive")
}

func main() {
	http.Handle("/health", middleware.Chain(http.HandlerFunc(handlerHealth), middleware.Logger))
	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
