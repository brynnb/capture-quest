package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

type JWTLoginRequest struct {
	Token      string `json:"token"`
	GuestToken string `json:"guestToken,omitempty"`
}

type JWTLoginResponse struct {
	Status  int32  `json:"status"`
	Message string `json:"message,omitempty"`
}

func HandleJWTLogin(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	ctx := context.Background()
	var req JWTLoginRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("failed to unmarshal JWTLogin JSON: %v", err)
		return false
	}

	token := req.Token
	var accountID int64

	// Check for local authentication formats first (guest, register, or email:password)
	// These formats are used by the client when not using third-party OAuth.
	isLocalFormat := strings.HasPrefix(token, "guest:") ||
		strings.HasPrefix(token, "register:") ||
		token == "local" ||
		(strings.Contains(token, ":") && !strings.HasPrefix(token, "ey"))

	if isLocalFormat {
		var err error
		accountID, err = handleLocalAuth(ctx, token, req.GuestToken)
		if err != nil {
			log.Printf("local auth failed: %v", err)
			ses.SendStreamJSON(JWTLoginResponse{Status: -1, Message: err.Error()}, opcodes.JWTResponse)
			return false
		}
	} else {
		// Non-local formats (like external JWTs) are not yet supported for this project's auth.
		log.Printf("unsupported auth format: %q", token)
		ses.SendStreamJSON(JWTLoginResponse{Status: -1, Message: "Unsupported authentication format"}, opcodes.JWTResponse)
		return false
	}

	ses.AccountID = accountID
	ses.Authenticated = true
	log.Printf("[AUTH] Session %d authenticated as account %d", ses.SessionID, accountID)
	ses.SendStreamJSON(JWTLoginResponse{Status: int32(ses.SessionID)}, opcodes.JWTResponse)

	// Record login IP
	if err := LoginIP(ctx, accountID, ses.IP); err != nil {
		log.Printf("[AUTH] Failed to record login IP for account %d: %v", accountID, err)
	}

	sendCharInfo(ses, accountID)
	return false
}

// handleLocalAuth handles authentication for local development mode.
// Returns the account ID on success, or an error on failure.
func handleLocalAuth(ctx context.Context, token, guestToken string) (int64, error) {
	// Legacy guest login with "local" token (backwards compatibility)
	if token == "local" {
		return 1, nil // Legacy guest account
	}

	// Check for guest token format: "guest:<uuid>"
	// Each browser gets a unique persistent guest account
	if strings.HasPrefix(token, "guest:") {
		guestToken := strings.TrimPrefix(token, "guest:")
		if guestToken == "" {
			return 0, fmt.Errorf("invalid guest token")
		}
		return GetOrCreateGuestAccount(ctx, guestToken)
	}

	// Check for registration format: "register:email:password"
	if strings.HasPrefix(token, "register:") {
		parts := strings.SplitN(token, ":", 3)
		if len(parts) != 3 {
			return 0, fmt.Errorf("invalid registration format")
		}
		email := parts[1]
		password := parts[2]
		return RegisterLocalAccount(ctx, email, password, guestToken)
	}

	// Regular login format: "email:password"
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid credentials format")
	}
	email := parts[0]
	password := parts[1]

	return LoginLocalAccount(ctx, email, password)
}
