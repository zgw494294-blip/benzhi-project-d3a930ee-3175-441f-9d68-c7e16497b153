package httpapi

import "net/http"

func requestID(r *http.Request) string {
	if v := r.Header.Get("X-Request-ID"); v != "" {
		return v
	}
	return r.Header.Get("Idempotency-Key")
}
func setResponseHeaders(w http.ResponseWriter, r *http.Request) {
	if id := requestID(r); id != "" {
		w.Header().Set("X-Request-ID", id)
	}
	w.Header().Set("Cache-Control", "no-store")
}
