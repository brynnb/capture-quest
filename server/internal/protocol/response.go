package protocol

// ErrorResponse is the shared unsuccessful query response. The generated literal
// discriminator lets clients narrow a success/error union without field casts.
type ErrorResponse struct {
	Success bool   `json:"success" tstype:"false"`
	Error   string `json:"error"`
}
