package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminRequestMustBeDirectLoopback(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       bool
	}{
		{name: "IPv4 loopback", remoteAddr: "127.0.0.1:1234", want: true},
		{name: "IPv6 loopback", remoteAddr: "[::1]:1234", want: true},
		{name: "remote", remoteAddr: "203.0.113.10:1234"},
		{name: "reverse proxied", remoteAddr: "127.0.0.1:1234", headers: map[string]string{"X-Forwarded-For": "203.0.113.10"}},
		{name: "real IP", remoteAddr: "127.0.0.1:1234", headers: map[string]string{"X-Real-IP": "203.0.113.10"}},
		{name: "forwarded", remoteAddr: "127.0.0.1:1234", headers: map[string]string{"Forwarded": "for=203.0.113.10"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
			request.RemoteAddr = test.remoteAddr
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			if got := isDirectLoopbackRequest(request); got != test.want {
				t.Fatalf("isDirectLoopbackRequest() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAdminMiddlewareHidesRouteFromPublicRequests(t *testing.T) {
	handlerCalled := false
	handler := adminAuthMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10")
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if handlerCalled {
		t.Fatal("admin handler ran for a public request")
	}
	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("admin response unexpectedly enabled CORS: %q", origin)
	}
}

func TestAdminRoutesDoNotExposeDatabaseConsole(t *testing.T) {
	mux := http.NewServeMux()
	server := &Server{}
	server.registerAdminRoutes(mux)

	for _, path := range []string{"/api/admin/db/query", "/api/admin/db/tables"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.RemoteAddr = "127.0.0.1:1234"
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, response.Code)
		}
	}
}

func TestAdminMethodsAreExplicit(t *testing.T) {
	called := false
	handler := requireAdminMethod(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/api/admin/stats", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
	if response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", response.Header().Get("Allow"))
	}
	if called {
		t.Fatal("admin handler ran for the wrong method")
	}
}
