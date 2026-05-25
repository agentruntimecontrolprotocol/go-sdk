package chi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	chir "github.com/go-chi/chi/v5"
)

func TestMountDefaultsPath(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	r := chir.NewRouter()
	h := Mount(r, srv, Options{AllowedHosts: []string{"example.test"}})
	if h == nil {
		t.Fatal("Mount must return a non-nil handler")
	}
	// A GET to "/arcp" should reach the upgrader (host check fails for
	// localhost so we should see 421, not 404).
	req := httptest.NewRequest(http.MethodGet, "/arcp", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("Mount did not register /arcp; got 404")
	}
}

func TestMountCustomPath(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	r := chir.NewRouter()
	_ = Mount(r, srv, Options{Path: "/custom"})
	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("Mount did not register /custom; got 404")
	}
}
