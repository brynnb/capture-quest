package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTileArtStudioRoutesAreNotRegisteredInProduction(t *testing.T) {
	mux := http.NewServeMux()
	server := &Server{debugMode: false}
	server.registerTileArtStudioRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/tiles/stamps", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("production tile-art route status = %d, want 404", response.Code)
	}
}

func TestTileArtStudioRoutesAreRegisteredLocally(t *testing.T) {
	mux := http.NewServeMux()
	server := &Server{debugMode: true}
	server.registerTileArtStudioRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/tiles/replace", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code == http.StatusNotFound {
		t.Fatal("local tile-art route was not registered")
	}
}
