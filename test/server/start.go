package main

import (
	"fmt"
	"net/http"
	"time"

	"hydragate/test/internal/auth"
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	})

	http.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		fmt.Fprint(w, time.Now().Format("15:04:05"))
	})

	http.HandleFunc("/date", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		fmt.Fprint(w, time.Now().Format("2006-01-02"))
	})

	http.HandleFunc("/auth/login", auth.LoginHandler("my-super-secret-key-change-in-production"))

	fmt.Println("Sever Running on http://localhost:9000")

	http.ListenAndServe(":9000", nil)
}
