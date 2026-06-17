package world

import (
	"fmt"
	"log"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
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

// getAccountStatus fetches the account's status (GM level) from the database.
func getAccountStatus(accountID int64) int32 {
	myDB := db.GlobalWorldDB.DB
	if myDB == nil {
		return 0
	}
	var status int32
	err := myDB.QueryRow("SELECT status FROM account WHERE id = $1", accountID).Scan(&status)
	if err != nil {
		log.Printf("[ChatCmd] failed to fetch account status for %d: %v", accountID, err)
		return 0
	}
	return status
}

// HandleChatCommand attempts to parse and execute a slash command.
// Returns true if the input was a command (even if it failed), false if it's regular chat.
func HandleChatCommand(ses *session.Session, text string, wh *WorldHandler) bool {
	if !strings.HasPrefix(text, "/") {
		return false
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
		status := getAccountStatus(ses.AccountID)
		if status < cmd.MinStatus {
			sendCommandError(ses, "You don't have permission to use that command.")
			return true
		}
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
			accountStatus := getAccountStatus(ses.AccountID)
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
