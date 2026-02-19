package proxy

import (
	"io"
	"net/http"
	"strings"
)

func Forward(reg *Registry) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// divide the path in by /
		first, rest, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
		route, ok := reg.GetRoute(first)
		if !ok {
			http.Error(w, "Route not found", http.StatusNotFound)
			return
		}

		url := route.Target + "/" + rest

		sendRequest(w, r, url)
	}
}

func sendRequest(w http.ResponseWriter, r *http.Request, url string) {
	req, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header = r.Header.Clone()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
