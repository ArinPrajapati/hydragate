package proxy

import (
	"hydragate/internal/urlpath"
	"io"
	"net/http"
)

func Forward(reg *Registry) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		parsed, _ := urlpath.Parse(r.URL.Path)
		route, ok := reg.GetRoute(parsed.Prefix)
		if !ok {
			http.Error(w, "Route not found", http.StatusNotFound)
			return
		}

		url := route.Target + "/" + parsed.Path

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
