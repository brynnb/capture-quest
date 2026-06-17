package opcodes

type OpCode uint16

const (
	Reconnect OpCode = 0

	// Auth and character-session opcodes.
	JWTResponse             OpCode = 1
	JWTLogin                OpCode = 2
	CharacterCreate         OpCode = 3
	DeleteCharacter         OpCode = 4
	CharacterCreateResponse OpCode = 5
	EnterWorld              OpCode = 6
	PostEnterWorld          OpCode = 7
	SendCharInfo            OpCode = 8
	ValidateNameRequest     OpCode = 9
	ValidateNameResponse    OpCode = 10
	Heartbeat               OpCode = 11

	// Shared data and chat opcodes.
	GetItemRequest         OpCode = 12
	GetItemResponse        OpCode = 13
	StaticDataRequest      OpCode = 14
	StaticDataResponse     OpCode = 15
	SendChatMessage        OpCode = 16
	ChatMessageBroadcast   OpCode = 17
	GetNPCDialogueRequest  OpCode = 18
	GetNPCDialogueResponse OpCode = 19
	CharCreateDataRequest  OpCode = 20
	CharCreateDataResponse OpCode = 21
	SetOption              OpCode = 23
	CharacterData          OpCode = 24
	CharacterWallet        OpCode = 26
	CharacterBind          OpCode = 27

	// Quest opcodes.
	QuestNPCInfoRequest   OpCode = 28
	QuestNPCInfoResponse  OpCode = 29
	QuestDialogueRequest  OpCode = 30
	QuestDialogueResponse OpCode = 31
	QuestTurnInRequest    OpCode = 32
	QuestTurnInResponse   OpCode = 33

	// Phaser map/data opcodes.
	PhaserMapInfoRequest         OpCode = 34
	PhaserMapInfoResponse        OpCode = 35
	PhaserTilesRequest           OpCode = 36
	PhaserTilesResponse          OpCode = 37
	PhaserOverworldMapsRequest   OpCode = 38
	PhaserOverworldMapsResponse  OpCode = 39
	PhaserActorsRequest          OpCode = 40
	PhaserActorsResponse         OpCode = 41
	PhaserWarpsRequest           OpCode = 42
	PhaserWarpsResponse          OpCode = 43
	PhaserActorPositionUpdate    OpCode = 44
	PhaserPlayerPositionUpdate   OpCode = 45
	PhaserActorDespawn           OpCode = 47
	PhaserDialogueRequest        OpCode = 48
	PhaserDialogueResponse       OpCode = 49
	PhaserWildEncountersRequest  OpCode = 50
	PhaserWildEncountersResponse OpCode = 51
	PhaserTrainerDataRequest     OpCode = 52
	PhaserTrainerDataResponse    OpCode = 53
	PhaserPokemonDataRequest     OpCode = 54
	PhaserPokemonDataResponse    OpCode = 55
	PhaserMoveDataRequest        OpCode = 56
	PhaserMoveDataResponse       OpCode = 57
	PhaserMapScriptsRequest      OpCode = 58
	PhaserMapScriptsResponse     OpCode = 59
	PhaserLearnsetRequest        OpCode = 60
	PhaserLearnsetResponse       OpCode = 61
	PhaserItemDataRequest        OpCode = 62
	PhaserItemDataResponse       OpCode = 63
	PhaserHiddenObjectsRequest   OpCode = 64
	PhaserHiddenObjectsResponse  OpCode = 65
	PhaserMapMusicRequest        OpCode = 66
	PhaserMapMusicResponse       OpCode = 67

	// Battle and trainer opcodes.
	PokeBattleStartRequest      OpCode = 68
	PokeBattleStartResponse     OpCode = 69
	PokeBattleActionRequest     OpCode = 70
	PokeBattleActionResponse    OpCode = 71
	PokeBattleSwitchRequest     OpCode = 72
	PokeBattleSwitchResponse    OpCode = 73
	PokeBattleEndNotify         OpCode = 74
	TrainerEncounterNotify      OpCode = 75
	TrainerEncounterReady       OpCode = 76
	PokemonPartyRequest         OpCode = 77
	PokemonPartyResponse        OpCode = 78
	PokeCenterHealRequest       OpCode = 79
	PokeCenterHealResponse      OpCode = 80
	TrainerInteractRequest      OpCode = 81
	TrainerInteractResponse     OpCode = 82
	TrainerBattleStartRequest   OpCode = 83
	CQBattleItemUseRequest      OpCode = 84
	CQBattleItemUseResponse     OpCode = 85
	PokeBattleCatchAttempt      OpCode = 86
	PokeMoveLearnRequest        OpCode = 87
	PokeMoveLearnResponse       OpCode = 88
	PokeBattleCloseRequest      OpCode = 89
	PokemonPartyReorderRequest  OpCode = 90
	PokemonPartyReorderResponse OpCode = 91

	// Inventory, shops, items, and PC opcodes.
	CQInventoryRequest         OpCode = 92
	CQInventoryResponse        OpCode = 93
	CQMerchantOpenRequest      OpCode = 94
	CQMerchantOpenResponse     OpCode = 95
	CQMerchantBuyRequest       OpCode = 96
	CQMerchantBuyResponse      OpCode = 97
	CQMerchantSellRequest      OpCode = 98
	CQMerchantSellResponse     OpCode = 99
	CQItemUseRequest           OpCode = 100
	CQItemUseResponse          OpCode = 101
	PokeFishingRequest         OpCode = 102
	PokeFishingResponse        OpCode = 103
	PokeSurfingRequest         OpCode = 104
	PokeSurfingResponse        OpCode = 105
	ItemPickupRequest          OpCode = 106
	ItemPickupResponse         OpCode = 107
	PokemonPCOpenRequest       OpCode = 108
	PokemonPCOpenResponse      OpCode = 109
	PokemonPCDepositRequest    OpCode = 110
	PokemonPCDepositResponse   OpCode = 111
	PokemonPCWithdrawRequest   OpCode = 112
	PokemonPCWithdrawResponse  OpCode = 113
	PokemonPCReleaseRequest    OpCode = 114
	PokemonPCReleaseResponse   OpCode = 115
	PokemonPCSwitchBoxRequest  OpCode = 116
	PokemonPCSwitchBoxResponse OpCode = 117

	// Dialogue, cutscene, warp, and elevator opcodes.
	DialogueChoiceRequest  OpCode = 118
	DialogueChoiceResponse OpCode = 119
	CutsceneStartNotify    OpCode = 120
	CutsceneEndRequest     OpCode = 121
	WarpTileTeleportNotify OpCode = 122
	ElevatorFloorsRequest  OpCode = 123
	ElevatorFloorsResponse OpCode = 124
	ElevatorSelectRequest  OpCode = 125

	// Safari Zone opcodes.
	SafariZoneEnterRequest     OpCode = 126
	SafariZoneEnterResponse    OpCode = 127
	SafariBattleStartNotify    OpCode = 128
	SafariBattleActionRequest  OpCode = 129
	SafariBattleActionResponse OpCode = 130
	SafariZoneStepUpdate       OpCode = 131
	SafariZoneExitNotify       OpCode = 132

	// Game Corner opcodes.
	GameCornerCoinBalanceRequest  OpCode = 133
	GameCornerCoinBalanceResponse OpCode = 134
	GameCornerBuyCoinsRequest     OpCode = 135
	GameCornerSlotPlayRequest     OpCode = 136
	GameCornerSlotResultResponse  OpCode = 137
	GameCornerPrizeListRequest    OpCode = 138
	GameCornerPrizeListResponse   OpCode = 139
	GameCornerPrizeBuyRequest     OpCode = 140
	GameCornerPrizeBuyResponse    OpCode = 141
	GameCornerCoinPickupNotify    OpCode = 142

	// Repel and Pokédex UI opcodes.
	RepelUseRequest       OpCode = 143
	RepelUseResponse      OpCode = 144
	RepelWoreOffNotify    OpCode = 145
	PokedexListRequest    OpCode = 146
	PokedexListResponse   OpCode = 147
	PokedexStatusRequest  OpCode = 148
	PokedexStatusResponse OpCode = 149
	TrainerCardRequest    OpCode = 150
	TrainerCardResponse   OpCode = 151

	// Local development and tile editing opcodes.
	DebugSceneListRequest         OpCode = 152
	DebugSceneListResponse        OpCode = 153
	DebugSceneJumpRequest         OpCode = 154
	DebugSceneJumpResponse        OpCode = 155
	TileEditorPlaceRequest        OpCode = 156
	TileEditorPlaceResponse       OpCode = 157
	TileEditorEraseRequest        OpCode = 158
	TileEditorEraseResponse       OpCode = 159
	TileEditorFillRequest         OpCode = 160
	TileEditorFillResponse        OpCode = 161
	TileEditorBroadcast           OpCode = 162
	TileEditorUndoRequest         OpCode = 163
	TileEditorUndoResponse        OpCode = 164
	TilePropertiesRequest         OpCode = 165
	TilePropertiesResponse        OpCode = 166
	TilePropertyUpdateRequest     OpCode = 167
	TilePropertyUpdateResponse    OpCode = 168
	DebugGivePowerPokemonRequest  OpCode = 169
	DebugGivePowerPokemonResponse OpCode = 170

	// Scripted events and session utility opcodes.
	ScriptedEventInteractRequest  OpCode = 171
	ScriptedEventInteractResponse OpCode = 172
	WarpHomeRequest               OpCode = 173
	WarpHomeResponse              OpCode = 174
	CharacterQuitRequest          OpCode = 175
	MapChangeRequest              OpCode = 176
	FieldMoveUseRequest           OpCode = 177
	FieldMoveUseResponse          OpCode = 178
)
