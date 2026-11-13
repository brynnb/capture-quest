package protocol

type FishingIdentity struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	InstanceID  int32  `json:"instanceId"`
}
type FishingResponse struct {
	FishingIdentity `tstype:",extends"`
	Success         bool   `json:"success" tstype:"true"`
	Hooked          bool   `json:"hooked"`
	Message         string `json:"message"`
}
type FishingError struct {
	FishingIdentity `tstype:",extends"`
	Success         bool   `json:"success" tstype:"false"`
	Error           string `json:"error"`
}
