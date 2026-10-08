package protocol

type TrainerInteractRequest struct {
	RequestID string `json:"requestId"`
	ActorID   int    `json:"actorId"`
}

type TrainerInteractIdentity struct {
	RequestID      string `json:"requestId"`
	CharacterID    int64  `json:"characterId"`
	TrainerActorID int    `json:"trainerActorId"`
}

type TrainerInteractResponse struct {
	TrainerInteractIdentity `tstype:",extends"`
	Success                 bool   `json:"success" tstype:"true"`
	TrainerName             string `json:"trainerName"`
	TrainerClass            string `json:"trainerClass"`
	Dialogue                string `json:"dialogue"`
	ShouldBattle            bool   `json:"shouldBattle"`
	Defeated                bool   `json:"defeated"`
}

type TrainerInteractError struct {
	TrainerInteractIdentity `tstype:",extends"`
	Success                 bool   `json:"success" tstype:"false"`
	Error                   string `json:"error"`
}
