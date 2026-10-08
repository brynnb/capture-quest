package protocol

type PhaserDialogueRequest struct {
	RequestID    string `json:"requestId"`
	TextConstant string `json:"textConstant"`
}

type PhaserDialogueIdentity struct {
	RequestID    string `json:"requestId"`
	CharacterID  int64  `json:"characterId"`
	TextConstant string `json:"textConstant"`
}

type PhaserDialogueEntry struct {
	Label      string  `json:"label"`
	SourceFile string  `json:"sourceFile"`
	Dialogue   string  `json:"dialogue"`
	IsTrainer  int     `json:"isTrainer"`
	MapName    *string `json:"mapName" tstype:"string | null,required"`
}

type PhaserDialogueResponse struct {
	PhaserDialogueIdentity `tstype:",extends"`
	Success                bool                  `json:"success" tstype:"true"`
	DialogueEntries        []PhaserDialogueEntry `json:"dialogueEntries"`
	HasBranching           bool                  `json:"hasBranching"`
	BranchingPrompt        *string               `json:"branchingPrompt" tstype:"string | null,required"`
}

type PhaserDialogueError struct {
	PhaserDialogueIdentity `tstype:",extends"`
	Success                bool   `json:"success" tstype:"false"`
	Error                  string `json:"error"`
}
