package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAdminChatsRejectsInvalidLimit(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/admin/chats?limit=101", nil)
	response := httptest.NewRecorder()
	handleAdminChats(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}
