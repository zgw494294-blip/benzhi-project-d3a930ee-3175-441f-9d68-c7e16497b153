package httpapi

import "net/http"

func noContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }
func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST, PATCH")
	w.WriteHeader(http.StatusMethodNotAllowed)
}
