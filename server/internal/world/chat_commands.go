package world

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

// ChatCommand defines a slash command handler.
type ChatCommand struct {
	// Name is the command name without the leading slash (e.g. "help").
	Name string
	// Description is a short help string shown in /help output.
	Description string
	// MinStatus is the minimum account status required (0 = everyone, 255 = Lead GM).
	MinStatus int32
	// Handler executes the command. Args is the trimmed text after the command name.
	Handler func(ses *session.Session, args string, wh *WorldHandler)
}

// chatCommandRegistry holds all registered slash commands.
var chatCommandRegistry = map[string]*ChatCommand{}

// RegisterChatCommand adds a command to the registry.
func RegisterChatCommand(cmd *ChatCommand) {
	chatCommandRegistry[strings.ToLower(cmd.Name)] = cmd
}

// getAccountStatus shares the caller's lifetime and runtime database. A failed
// read is not a zero-status account and must not execute a privileged command.
func getAccountStatus(ctx context.Context, database *sql.DB, accountID int64) (int32, error) {
	if database == nil {
		return 0, fmt.Errorf("account status database is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var status int32
	if err := database.QueryRowContext(ctx, "SELECT status FROM account WHERE id=$1", accountID).Scan(&status); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return 0, fmt.Errorf("read account %d status: %w", accountID, err)
	}
	return status, nil
}

// HandleChatCommand attempts to parse and execute a slash command.
// Returns true if the input was a command (even if it failed), false if it's regular chat.
func HandleChatCommand(ses *session.Session, text string, wh *WorldHandler) bool {
	if !strings.HasPrefix(text, "/") {
		return false
	}
	if ses.IsClosed() || ses.CommandContext().Err() != nil {
		return true
	}

	// Parse: "/commandName arg1 arg2 ..."
	trimmed := strings.TrimPrefix(text, "/")
	parts := strings.SplitN(trimmed, " ", 2)
	cmdName := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	cmd, ok := chatCommandRegistry[cmdName]
	if !ok {
		sendCommandError(ses, fmt.Sprintf("Unknown command: /%s", cmdName))
		return true
	}

	// Permission check
	if cmd.MinStatus > 0 {
		status, err := getAccountStatus(ses.CommandContext(), wh.database, ses.AccountID)
		if err != nil {
			sendCommandError(ses, "Account permissions unavailable.")
			return true
		}
		if status < cmd.MinStatus {
			sendCommandError(ses, "You don't have permission to use that command.")
			return true
		}
	}

	if ses.IsClosed() || ses.CommandContext().Err() != nil {
		return true
	}
	cmd.Handler(ses, args, wh)
	return true
}

// sendCommandError sends a system error message back to the command sender only.
func sendCommandError(ses *session.Session, msg string) {
	ses.SendStreamJSON(ChatMessageBroadcast{
		Text:        msg,
		MessageType: "system",
	}, opcodes.ChatMessageBroadcast)
}

// sendCommandResponse sends a system message back to the command sender only.
func sendCommandResponse(ses *session.Session, msg string) {
	ses.SendStreamJSON(ChatMessageBroadcast{
		Text:        msg,
		MessageType: "system",
	}, opcodes.ChatMessageBroadcast)
}

func init() {
	// /help — list available commands
	RegisterChatCommand(&ChatCommand{
		Name:        "help",
		Description: "List available commands",
		MinStatus:   0,
		Handler: func(ses *session.Session, args string, wh *WorldHandler) {
			accountStatus, err := getAccountStatus(ses.CommandContext(), wh.database, ses.AccountID)
			if err != nil {
				sendCommandError(ses, "Account permissions unavailable.")
				return
			}
			lines := []string{"Available commands:"}
			for _, cmd := range chatCommandRegistry {
				if cmd.MinStatus <= accountStatus {
					lines = append(lines, fmt.Sprintf("  /%s — %s", cmd.Name, cmd.Description))
				}
			}
			sendCommandResponse(ses, strings.Join(lines, "\n"))
		},
	})
}
