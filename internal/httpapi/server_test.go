package httpapi

import (
	"biocuration/internal/application"
	"biocuration/internal/repository"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMissingIdempotency(t *testing.T) {
	s, _ := repository.Open(":memory:")
	defer s.Close()
	h := New(application.New(s)).Handler()
	r := httptest.NewRequest("POST", "/v1/trees", bytes.NewBufferString(`{"treeID":"t"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("状态码 %d", w.Code)
	}
}
func TestHealth(t *testing.T) {
	s, _ := repository.Open(":memory:")
	defer s.Close()
	h := New(application.New(s)).Handler()
	r := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var v map[string]string
	_ = json.NewDecoder(w.Body).Decode(&v)
	if v["status"] != "ok" {
		t.Fatal(v)
	}
	_ = context.Background()
	_ = http.MethodGet
}
