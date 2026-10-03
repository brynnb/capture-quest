package world

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log"
	"time"

	"capturequest/internal/api"
	"capturequest/internal/api/opcodes"
	"capturequest/internal/logutil"
	"capturequest/internal/session"
)

// DatagramHandler defines the signature for handling datagrams.
type DatagramHandler func(session *session.Session, payload []byte, wh *WorldHandler) bool

// HandlerRegistry holds the handler mappings and dependencies.
type HandlerRegistry struct {
	handlers      map[opcodes.OpCode]DatagramHandler
	globalOpcodes map[opcodes.OpCode]bool // Opcodes that should be handled globally
	WH            *WorldHandler           `json:"wH,omitempty"`
}

func NewWorldOpCodeRegistry() *HandlerRegistry {
	handlers := map[opcodes.OpCode]DatagramHandler{
		opcodes.JWTLogin:         HandleJWTLogin,
		opcodes.CharacterCreate:  HandleCharacterCreate,
		opcodes.DeleteCharacter:  HandleCharacterDelete,
		opcodes.EnterWorld:       HandleEnterWorld,
		opcodes.MapChangeRequest: HandleMapChangeRequest,
		// Data query handlers
		opcodes.StaticDataRequest:     HandleStaticDataRequest,
		opcodes.SendChatMessage:       HandleSendChatMessage,
		opcodes.ValidateNameRequest:   HandleValidateNameRequest,
		opcodes.CharCreateDataRequest: HandleCharCreateDataRequest,
		opcodes.CharacterQuitRequest:  HandleCharacterQuitRequest,
		// Unified options handler
		opcodes.SetOption: HandleSetOption,
		opcodes.Heartbeat: HandleHeartbeat,
		// Phaser 2D game handlers
		opcodes.PhaserMapInfoRequest:       HandlePhaserMapInfoRequest,
		opcodes.PhaserTilesRequest:         HandlePhaserTilesRequest,
		opcodes.PhaserOverworldMapsRequest: HandlePhaserOverworldMapsRequest,
		opcodes.PhaserActorsRequest:        HandlePhaserActorsRequest,

		opcodes.PhaserWarpsRequest:         HandlePhaserWarpsRequest,
		opcodes.PhaserPlayerPositionUpdate: HandlePhaserPlayerPositionUpdate,
		// Phaser data endpoints (Phase 2.4)
		opcodes.PhaserDialogueRequest:       HandlePhaserDialogueRequest,
		opcodes.PhaserWildEncountersRequest: HandlePhaserWildEncountersRequest,
		opcodes.PhaserTrainerDataRequest:    HandlePhaserTrainerDataRequest,
		opcodes.PhaserPokemonDataRequest:    HandlePhaserPokemonDataRequest,
		opcodes.PhaserMoveDataRequest:       HandlePhaserMoveDataRequest,
		opcodes.PhaserMapScriptsRequest:     HandlePhaserMapScriptsRequest,
		opcodes.PhaserLearnsetRequest:       HandlePhaserLearnsetRequest,
		opcodes.PhaserItemDataRequest:       HandlePhaserItemDataRequest,
		opcodes.PhaserHiddenObjectsRequest:  HandlePhaserHiddenObjectsRequest,
		opcodes.PhaserMapMusicRequest:       HandlePhaserMapMusicRequest,
		// Pokémon Battle handlers (Phase 4)
		opcodes.PokeBattleStartRequest:  HandlePokeBattleStart,
		opcodes.PokeBattleActionRequest: HandlePokeBattleAction,
		opcodes.PokeBattleSwitchRequest: HandlePokeBattleSwitch,
		// Trainer Encounter handlers (Phase 4.6)
		opcodes.TrainerEncounterReady: HandleTrainerEncounterReady,
		// Pokémon Party handlers (Phase 6.1)
		opcodes.PokemonPartyRequest: HandlePokemonPartyRequest,
		// Pokémon Center healing handler (Phase 6.6)
		opcodes.PokeCenterHealRequest: HandlePokeCenterHeal,
		// Trainer click interactions
		opcodes.TrainerInteractRequest:    HandleTrainerInteractRequest,
		opcodes.TrainerBattleStartRequest: HandleTrainerBattleStartRequest,
		// CQ Item & Inventory handlers (Phase 7)
		opcodes.CQInventoryRequest:     HandleCQInventoryRequest,
		opcodes.CQMerchantOpenRequest:  HandleCQMerchantOpenRequest,
		opcodes.CQMerchantBuyRequest:   HandleCQMerchantBuyRequest,
		opcodes.CQMerchantSellRequest:  HandleCQMerchantSellRequest,
		opcodes.CQItemUseRequest:       HandleCQItemUse,
		opcodes.CQBattleItemUseRequest: HandleCQBattleItemUse,
		opcodes.PokeFishingRequest:     HandlePokeFishing,
		opcodes.PokeSurfingRequest:     HandlePokeSurfing,
		opcodes.FieldMoveUseRequest:    HandleFieldMoveUse,
		opcodes.ItemPickupRequest:      HandleItemPickup,
		// Pokémon PC handlers (Phase 6.5)
		opcodes.PokemonPCOpenRequest:      HandlePokemonPCOpen,
		opcodes.PokemonPCDepositRequest:   HandlePokemonPCDeposit,
		opcodes.PokemonPCWithdrawRequest:  HandlePokemonPCWithdraw,
		opcodes.PokemonPCReleaseRequest:   HandlePokemonPCRelease,
		opcodes.PokemonPCSwitchBoxRequest: HandlePokemonPCSwitchBox,
		// Move learning handler (Phase 6.2)
		opcodes.PokeMoveLearnRequest: HandlePokeMoveLearn,
		// Battle close handler — client confirms battle UI dismissed
		opcodes.PokeBattleCloseRequest: HandlePokeBattleClose,
		// Party reorder handler (Phase 6.1)
		opcodes.PokemonPartyReorderRequest: HandlePokemonPartyReorder,
		// Dialogue choice handler (Phase 9.6)
		opcodes.DialogueChoiceRequest: HandleDialogueChoiceRequest,
		// Cutscene handler (Phase 9.4)
		opcodes.CutsceneEndRequest: HandleCutsceneEndRequest,
		// Scripted event interaction handler
		opcodes.ScriptedEventInteractRequest: HandleScriptedEventInteract,
		// Recovery warp handler
		opcodes.WarpHomeRequest: HandleWarpHomeRequest,
		// Elevator handlers (Phase 9.7)
		opcodes.ElevatorFloorsRequest: HandleElevatorFloorsRequest,
		opcodes.ElevatorSelectRequest: HandleElevatorSelectRequest,
		// Safari Zone handlers (Phase 11.3)
		opcodes.SafariZoneEnterRequest:    HandleSafariZoneEnter,
		opcodes.SafariBattleActionRequest: HandleSafariBattleAction,
		// Game Corner handlers (Phase 11.4)
		opcodes.GameCornerCoinBalanceRequest: HandleGameCornerCoinBalance,
		opcodes.GameCornerBuyCoinsRequest:    HandleGameCornerBuyCoins,
		opcodes.GameCornerSlotPlayRequest:    HandleGameCornerSlotPlay,
		opcodes.GameCornerPrizeListRequest:   HandleGameCornerPrizeList,
		opcodes.GameCornerPrizeBuyRequest:    HandleGameCornerPrizeBuy,
		// Repel handler (Phase 11.2)
		opcodes.RepelUseRequest: HandleRepelUse,
		// Pokédex & UI handlers (Phase 10)
		opcodes.PokedexListRequest:   HandlePokedexListRequest,
		opcodes.PokedexStatusRequest: HandlePokedexStatusRequest,
		opcodes.TrainerCardRequest:   HandleTrainerCardRequest,
		// Debug Scene Debugger
		opcodes.DebugSceneListRequest:        HandleDebugSceneListRequest,
		opcodes.DebugSceneJumpRequest:        HandleDebugSceneJumpRequest,
		opcodes.DebugGivePowerPokemonRequest: HandleDebugGivePowerPokemonRequest,
		opcodes.DebugWarpProbeCasesRequest:   HandleDebugWarpProbeCasesRequest,
		// Tile Editor handlers (Dynamic World)
		opcodes.TileEditorPlaceRequest:    HandleTileEditorPlace,
		opcodes.TileEditorEraseRequest:    HandleTileEditorErase,
		opcodes.TileEditorFillRequest:     HandleTileEditorFill,
		opcodes.TileEditorUndoRequest:     HandleTileEditorUndo,
		opcodes.TilePropertiesRequest:     HandleTilePropertiesRequest,
		opcodes.TilePropertyUpdateRequest: HandleTilePropertyUpdate,
	}

	globalOpcodes := make(map[opcodes.OpCode]bool)
	for opCode := range handlers {
		globalOpcodes[opCode] = true
	}

	registry := &HandlerRegistry{
		handlers:      handlers,
		globalOpcodes: globalOpcodes,
	}

	return registry
}

func (r *HandlerRegistry) ShouldHandleGlobally(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	op := binary.LittleEndian.Uint16(data[:2])
	return r.globalOpcodes[(opcodes.OpCode)(op)]
}

func (r *HandlerRegistry) HandleWorldPacket(ses *session.Session, data []byte) bool {
	if ses == nil || len(data) < 2 || len(data) > api.MaxClientPacketSize {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result bool
	err := ses.ExecuteCommand(ctx, func() { result = r.handleWorldPacket(ses, data) })
	if err != nil && !errors.Is(err, session.ErrSessionClosed) && !ses.IsClosed() {
		log.Printf("[HandlerRegistry] Session %d command rejected: %v", ses.SessionID, err)
		ses.Close()
	}
	return result
}

func (r *HandlerRegistry) handleWorldPacket(ses *session.Session, data []byte) bool {
	if ses == nil || ses.IsClosed() || len(data) < 2 || len(data) > api.MaxClientPacketSize {
		return false
	}
	op := binary.LittleEndian.Uint16(data[:2])
	payload := bytes.TrimSpace(data[2:])
	// Every client command currently has a JSON object payload. In particular,
	// JSON null must not decode as a successful, zero-valued gameplay command.
	if len(payload) == 0 || payload[0] != '{' || !json.Valid(payload) {
		return false
	}
	if !sessionAllowsOpcode(ses, opcodes.OpCode(op)) {
		logutil.Debugf("[HandlerRegistry] Rejected opcode %d in current session state", op)
		return false
	}

	if logutil.DebugEnabled() && opcodes.OpCode(op) != opcodes.Heartbeat {
		logutil.Debugf("[HandlerRegistry] Received opcode %d from session %d (payload length: %d)", op, ses.SessionID, len(payload))
	}

	forwardToZone := false
	if h, ok := r.handlers[(opcodes.OpCode)(op)]; ok {
		forwardToZone = h(ses, payload, r.WH)
	} else {
		log.Printf("no handler for opcode %d from session %d", op, ses.SessionID)
	}
	return forwardToZone
}

// New gameplay opcodes require a selected character by default. Only explicit
// connection/account operations may run before selection. Battle-specific
// rules and GM authorization remain in their authoritative gameplay handlers.
func sessionAllowsOpcode(ses *session.Session, op opcodes.OpCode) bool {
	if op == opcodes.Heartbeat {
		return true // The login screen also keeps its connection alive.
	}
	if op == opcodes.JWTLogin {
		// Account changes at character select include guest-account registration.
		return !ses.HasValidClient()
	}
	if !ses.Authenticated {
		return false
	}
	switch op {
	case opcodes.CharacterCreate, opcodes.DeleteCharacter, opcodes.EnterWorld:
		return !ses.HasValidClient()
	case opcodes.StaticDataRequest, opcodes.CharCreateDataRequest, opcodes.ValidateNameRequest:
		return true
	default:
		return ses.HasValidClient()
	}
}

func NewZoneOpCodeRegistry(zoneID int) *HandlerRegistry {
	registry := &HandlerRegistry{
		handlers:      map[opcodes.OpCode]DatagramHandler{},
		globalOpcodes: map[opcodes.OpCode]bool{},
	}

	return registry
}
