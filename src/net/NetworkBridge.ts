import { presentBattleEnd } from "@/phaser-game/services/BattleCommandService";
import type { BattleEndOutcome } from "@/net/generated/world_api";
import { WorldSocket } from "./index";
import * as OpCodes from "./generated/opcodes";
import type { OpCode } from "./generated/opcodes";
import * as WorldTypes from "./generated/world";
import type * as ProtocolTypes from "@/net/generated/protocol";
import * as ModelTypes from "./generated/models";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import useCharacterSelectStore, {
  type CharacterSelectEntry,
} from "@/stores/CharacterSelectStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameStatusStore from "@/stores/GameStatusStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import useAudioActivityStore from "@/stores/AudioActivityStore";
import AudioManager from "@/services/audio/AudioManager";
import {
  cryPathForPokemon,
  sfxPathForConstant,
} from "@/services/audio/pokemonMusic";

/**
 * NetworkBridge acts as a central dispatcher for JSON-based network messages.
 * It observes WorldSocket.onJson and routes messages to the appropriate
 * Zustand stores or services using generated TypeScript types.
 */
export class NetworkBridge {
  private static instance: NetworkBridge;

  public static initialize() {
    if (!this.instance) {
      this.instance = new NetworkBridge();
    }
  }

  /**
   * Send a JSON message to the server via WebTransport control stream.
   */
  public static send(data: unknown, opcode: OpCode): Promise<void> {
    return WorldSocket.sendStreamJsonMessage(opcode, data);
  }

  private constructor() {
    WorldSocket.onJson = (opcode, data) => this.dispatch(opcode, data);
    console.log("[NetworkBridge] Initialized and listening for JSON messages");
  }

  private dispatch(opcode: OpCode, data: unknown) {
    switch (opcode) {
      case OpCodes.JWTResponse:
        this.handleJWTResponse(data as WorldTypes.JWTLoginResponse);
        break;

      case OpCodes.SendCharInfo:
        this.handleSendCharInfo(
          data as { characters?: CharacterSelectEntry[] },
        );
        break;

      case OpCodes.PostEnterWorld:
      case OpCodes.CharacterCreateResponse:
        this.handleSimpleSuccess(
          opcode,
          data as WorldTypes.SimpleSuccessResponse,
        );
        break;

      // New separate character data streams (replaces legacy CharacterState)
      case OpCodes.CharacterData:
        this.handleCharacterData(data as ProtocolTypes.CharacterData);
        break;

      case OpCodes.CharacterWallet:
        this.handleCharacterWalletData(data as ModelTypes.CharacterWallet);
        break;

      case OpCodes.CharacterBind:
        this.handleCharacterBindData(data as ModelTypes.CharacterBind);
        break;

      case OpCodes.SendChatMessage:
      case OpCodes.ChatMessageBroadcast:
        this.handleChatMessage(data as WorldTypes.ChatMessageBroadcast);
        break;

      // Phaser 2D game opcodes
      case OpCodes.PhaserMapInfoResponse:
      case OpCodes.PokemonPCOpenResponse:
      case OpCodes.PokemonPCDepositResponse:
      case OpCodes.PokemonPCWithdrawResponse:
      case OpCodes.PokemonPCReleaseResponse:
      case OpCodes.PokemonPCSwitchBoxResponse:
      case OpCodes.CutsceneEndResponse:
      case OpCodes.EscapeRopeUseResponse:
      case OpCodes.StaticDataResponse:
      case OpCodes.CharCreateDataResponse:
      case OpCodes.SetOption:
      case OpCodes.BicycleStateResponse:
      case OpCodes.GameplayStateResponse:
      case OpCodes.CQMerchantOpenResponse:
      case OpCodes.CQMerchantBuyResponse:
      case OpCodes.CQMerchantSellResponse:
      case OpCodes.RepelUseResponse:
      case OpCodes.PokemonPartyReorderResponse:
      case OpCodes.SafariBattleActionResponse:
      case OpCodes.PokeBattleActionResponse:
      case OpCodes.PokeBattleSwitchResponse:
      case OpCodes.CQBattleItemUseResponse:
      case OpCodes.PokeMoveLearnResponse:
      case OpCodes.PokeBattleCloseResponse:
      case OpCodes.OwnedPlayerPositionResponse:
      case OpCodes.ServerPlayerMovementNotify:
      case OpCodes.PlayerFacingResponse:
      case OpCodes.PlayerStepResponse:
      case OpCodes.PlayerStepCompleteResponse:
      case OpCodes.PhaserMapLoadResponse:
      case OpCodes.PhaserInstantWarpResponse:
      case OpCodes.PhaserWarpActivateResponse:
      case OpCodes.PhaserTilesResponse:
      case OpCodes.PhaserOverworldMapsResponse:
      case OpCodes.PhaserActorsResponse:
      case OpCodes.PhaserWarpsResponse:
      case OpCodes.PhaserActorPositionUpdate:
      case OpCodes.PhaserActorDespawn:
      case OpCodes.TrainerEncounterNotify:
        // Delegate to Phaser network service
        import("@/phaser-game/services/PhaserNetworkService").then((module) =>
          module.dispatchPhaserResponse(opcode, data),
        );
        break;

      // Tile Editor opcodes (Dynamic World)
      case OpCodes.TilePropertiesResponse:
      case OpCodes.TileEditorPlaceResponse:
      case OpCodes.TileEditorEraseResponse:
      case OpCodes.TileEditorFillResponse:
      case OpCodes.TileEditorUndoResponse:
      case OpCodes.TileEditorBroadcast:
      case OpCodes.TilePropertyUpdateResponse:
        import("@/components/TileEditor/TileEditorNetwork").then((module) =>
          module.dispatchTileEditorResponse(opcode, data),
        );
        break;

      // Pokémon Battle opcodes (Phase 4)
      case OpCodes.PokeBattleStartResponse:
        this.handlePokeBattleStart(data as Record<string, unknown>);
        break;
      case OpCodes.PokeBattleEndNotify:
        this.handlePokeBattleEnd(data as Record<string, unknown>);
        break;

      case OpCodes.ResourcesChangedNotify:
        import("@/phaser-game/services/InventoryCommandService").then(module=>module.acceptResourceChangeNotification(data));
        break;
      case OpCodes.CQItemUseResponse:
        // Correlated party replies belong exclusively to the scene coordinator.
        // Field-item effects retain their existing uncorrelated protocol.
        if (typeof (data as Record<string, unknown>).requestId === "string") {
          import("@/phaser-game/services/PhaserNetworkService").then(module => module.dispatchPhaserResponse(opcode, data));
        } else this.handleCQItemUseResponse(data as Record<string, unknown>);
        break;
      case OpCodes.PokeFishingResponse:
        this.handlePokeFishingResponse(data as Record<string, unknown>);
        break;
      case OpCodes.PokeSurfingResponse:
        this.handlePokeSurfingResponse(data as Record<string, unknown>);
        break;
      case OpCodes.FieldMoveUseResponse:
        this.handleFieldMoveUseResponse(data as Record<string, unknown>);
        break;
      case OpCodes.ItemPickupResponse:
        this.handleItemPickupResponse(data as Record<string, unknown>);
        break;

      // Pokémon PC (Phase 6.5)

      // Move learning (Phase 6.2)


      // Dialogue choice (Phase 9.6)
      case OpCodes.DialogueChoiceResponse:
        this.handleDialogueChoiceResponse(data as Record<string, unknown>);
        break;

      // Cutscene (Phase 9.4)
      case OpCodes.CutsceneStartNotify:
        this.handleCutsceneStartNotify(data as Record<string, unknown>);
        break;

      // Warp tile teleport (Phase 9.7)
      case OpCodes.WarpTileTeleportNotify:
        this.handleWarpTileTeleport(data as ProtocolTypes.WarpTileTeleportNotify);
        break;
      case OpCodes.WarpHomeResponse:
        this.handleWarpHomeResponse(data as Record<string, unknown>);
        break;

      // Elevator floor list (Phase 9.7)
      case OpCodes.ElevatorFloorsResponse:
        this.handleElevatorFloorsResponse(data as { floors: Array<{ floorMapId: number; floorLabel: string; destX: number; destY: number }>; message?: string });
        break;

      // Repel (Phase 11.2)
      case OpCodes.RepelWoreOffNotify:
        this.handleRepelEvent(data as { message?: string });
        break;

      // Game Corner (Phase 11.4)
      case OpCodes.GameCornerCoinBalanceResponse:
      case OpCodes.GameCornerSlotResultResponse:
      case OpCodes.GameCornerPrizeListResponse:
      case OpCodes.GameCornerPrizeBuyResponse:
      case OpCodes.GameCornerCoinPickupNotify:
        this.handleGameCornerEvent(opcode, data);
        break;

      // Safari Zone (Phase 11.3)
      case OpCodes.SafariZoneEnterResponse:
        this.handleSafariEvent("safariZoneEnter", data);
        break;
      case OpCodes.SafariBattleStartNotify:
        this.handleSafariBattleStart(data as Record<string, unknown>);
        break;
      case OpCodes.SafariZoneStepUpdate:
        this.handleSafariEvent("safariStepUpdate", data);
        break;
      case OpCodes.SafariZoneExitNotify:
        this.handleSafariEvent("safariZoneExit", data);
        break;

      // Pokédex & UI (Phase 10)
      case OpCodes.PokedexListResponse:
        this.handlePokedexListResponse(data as ProtocolTypes.PokedexListResponse | ProtocolTypes.ErrorResponse);
        break;
      case OpCodes.PokedexStatusResponse:
        this.handlePokedexStatusResponse(data as ProtocolTypes.PokedexStatusResponse | ProtocolTypes.ErrorResponse);
        break;
      case OpCodes.TrainerCardResponse:
        this.handleTrainerCardResponse(data as ProtocolTypes.TrainerCardResponse | ProtocolTypes.ErrorResponse);
        break;

      // Debug Scene Debugger
      case OpCodes.DebugSceneListResponse:
        this.handleDebugSceneListResponse(data as Record<string, unknown>);
        break;
      case OpCodes.DebugSceneJumpResponse:
        this.handleDebugSceneJumpResponse(data as Record<string, unknown>);
        break;
      case OpCodes.DebugGivePowerPokemonResponse:
        this.handleDebugGivePowerPokemonResponse(data as Record<string, unknown>);
        break;

      default:
        // Opcode not handled by JSON bridge
        break;
    }
  }

  private handleJWTResponse(data: WorldTypes.JWTLoginResponse) {
    console.log("[NetworkBridge] JWT Response:", data);
  }

  private handleSendCharInfo(data: { characters?: CharacterSelectEntry[] }) {
    useCharacterSelectStore.getState().setCharacters(data.characters || []);
    useCharacterSelectStore.getState().setIsLoading(false);

    const currentScreen = useGameScreenStore.getState().currentScreen;
    if (
      currentScreen === "title" ||
      currentScreen === "login" ||
      currentScreen === "register"
    ) {
      useGameScreenStore.getState().setScreen("characterSelect");
    }
  }

  private handleSimpleSuccess(
    opcode: OpCode,
    data: WorldTypes.SimpleSuccessResponse,
  ) {
    console.log(`[NetworkBridge] Success for opcode ${opcode}:`, data.value);
  }

  // New separate handlers for character data streams
  private handleCharacterData(data: ProtocolTypes.CharacterData) {
    usePlayerCharacterStore.getState().handleCharacterData(data);
  }

  private handleCharacterWalletData(data: ModelTypes.CharacterWallet) {
    usePlayerCharacterStore.getState().handleCharacterWalletData(data);
  }

  private handleCharacterBindData(data: ModelTypes.CharacterBind) {
    usePlayerCharacterStore.getState().handleCharacterBindData(data);
  }

  private handleChatMessage(data: WorldTypes.ChatMessageBroadcast) {
    useChatStore.getState().handleChatMessage(data);

    // Emit a chat bubble event for general player chat.
    if (
      data.senderId &&
      (data.messageType || "").toLowerCase() === "general"
    ) {
      window.dispatchEvent(
        new CustomEvent("playerChatBubble", {
          detail: {
            senderId: data.senderId,
            senderName: data.senderName,
            text: data.text,
          },
        }),
      );
    }
  }

  private handlePokeBattleStart(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Battle start failed:", data.error);
      return;
    }
    useAudioActivityStore.getState().setBattleVictoryTrack(null);
    usePokeBattleStore.getState().startBattle({
      battleId: data.battleId as string,
      revision: data.revision as number,
      playerPokemon: data.playerPokemon as PokeBattlePokemonDTO,
      enemyPokemon: data.enemyPokemon as PokeBattlePokemonDTO,
      phase: data.phase as string,
      turnNumber: data.turnNumber as number,
      events: (data.events || []) as BattleEventDTO[],
      trainerClass: (data.trainerClass as string) || undefined,
      playerParty: data.playerParty as PokeBattlePokemonDTO[] | undefined,
      playerActive: data.playerActive as number | undefined,
      battleType: data.battleType as string | undefined,
      allowedActions: data.allowedActions as string[] | undefined,
      guaranteedCatch: data.guaranteedCatch as boolean | undefined,
    });
    this.playPokemonCry(data.enemyPokemon as PokeBattlePokemonDTO | undefined);
  }

  private playPokemonCry(pokemon?: PokeBattlePokemonDTO) {
    const path = cryPathForPokemon(pokemon?.name, pokemon?.crySfx);
    if (path) {
      void AudioManager.playSFX(path, 0.8);
    }
  }

  private playSourceSFX(sfxConstant: string, volume: number) {
    const path = sfxPathForConstant(sfxConstant);
    if (path) {
      void AudioManager.playSFX(path, volume);
    }
  }

  private handlePokeBattleEnd(data: Record<string, unknown>) {
    // Only standalone battle-start blackout still uses this unsolicited opcode.
    // Ordinary command end outcomes travel inside their correlated reply.
    const end = data as unknown as BattleEndOutcome;
    if (!end.playerWon && end.blackoutMapId > 0 && !usePokeBattleStore.getState().isInBattle) {
      useGameStatusStore.getState().triggerBlackoutWarp(end.blackoutMapId, end.blackoutX, end.blackoutY);
      return;
    }
    presentBattleEnd(end);
  }

  private handleCQItemUseResponse(data: Record<string, unknown>) {
    if (!data.success) {
      const error = String(data.error || "It won't have any effect");
      useChatStore.getState().addMessage(error, MessageType.SYSTEM);
      console.warn("[NetworkBridge] Item use failed:", data.error);
      this.playSourceSFX("SFX_DENIED", 0.8);
      return;
    }

    // Update inventory quantity (or remove if depleted)
    const instanceId = data.instanceId as number;
    const newQty = data.newQty as number;
    const store = useCQInventoryStore.getState();
    if (newQty !== undefined && newQty <= 0) {
      store.setInventory(store.items.filter((item) => item.instance.id !== instanceId), store.money);
    } else if (newQty !== undefined) {
      // Update quantity on the existing item
      const items = store.items.map((i) =>
        i.instance.id === instanceId
          ? { ...i, instance: { ...i.instance, quantity: newQty } }
          : i,
      );
      useCQInventoryStore.setState({ items });
    }

    if (data.message) {
      useChatStore.getState().addMessage(String(data.message), MessageType.SYSTEM);
    }

    this.playSourceSFX("SFX_PRESS_AB", 0.75);
    console.log("[NetworkBridge] Used item:", data.message);
  }

  private handlePokeFishingResponse(data: Record<string, unknown>) {
    const message = String(data.error || data.message || "");
    if (!data.success) {
      console.warn("[NetworkBridge] Fishing failed:", data.error);
      if (message) {
        usePokemonDialogueStore.getState().openDialogue([message]);
        useChatStore.getState().addMessage(message, MessageType.SYSTEM);
      }
      this.playSourceSFX("SFX_DENIED", 0.8);
      return;
    }
    if (message) {
      usePokemonDialogueStore.getState().openDialogue([message]);
      useChatStore.getState().addMessage(message, MessageType.SYSTEM);
    }
    if (data.hooked) {
      console.log("[NetworkBridge] Fishing: hooked a Pokémon!");
      this.playSourceSFX("SFX_PRESS_AB", 0.75);
      // Battle start will arrive via PokeBattleStartResponse
    } else {
      console.log("[NetworkBridge] Fishing:", data.message);
      this.playSourceSFX("SFX_DENIED", 0.6);
    }
  }

  private handlePokeSurfingResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Surfing failed:", data.error);
      if (data.error) {
        usePokemonDialogueStore.getState().openDialogue([String(data.error)]);
      }
      return;
    }
    if (data.encounter) {
      console.log("[NetworkBridge] Surfing: wild Pokémon appeared!");
      // Battle start will arrive via PokeBattleStartResponse
    } else {
      console.log("[NetworkBridge] Surfing:", data.message || "No encounter");
    }

    // A committed Surf entry can end in blackout. Its recovery notification
    // owns presentation; the water-entry animation must not overwrite it.
    if (data.blackout === true) return;

    const x = Number(data.x);
    const y = Number(data.y);
    const mapId = Number(data.mapId);
    if (Number.isFinite(x) && Number.isFinite(y) && Number.isFinite(mapId)) {
      window.dispatchEvent(
        new CustomEvent("pokeSurfingSuccess", {
          detail: {
            x,
            y,
            mapId,
            direction:
              typeof data.direction === "string" ? data.direction : undefined,
          },
        }),
      );
    }
  }

  private handleFieldMoveUseResponse(data: Record<string, unknown>) {
    if (!data.success) {
      const error = String(data.error || "That move can't be used here.");
      console.warn("[NetworkBridge] Field move failed:", error);
      usePokemonDialogueStore.getState().openDialogue([error]);
      return;
    }

    const tile = data.tile as
      | {
          x: number;
          y: number;
          tileImageId: number;
          collisionType: number;
          rawFootTileId?: number;
          talkOverTile?: boolean;
          erased?: boolean;
        }
      | undefined;
    if (tile) {
      window.dispatchEvent(
        new CustomEvent("worldTileUpdate", {
          detail: {
            mapId: data.mapId,
            tiles: [tile],
          },
        }),
      );
      window.dispatchEvent(
        new CustomEvent("tileEditorBroadcast", {
          detail: {
            mapId: data.mapId,
            tiles: [tile],
          },
        }),
      );
    }

    if (data.message) {
      usePokemonDialogueStore.getState().openDialogue([String(data.message)]);
    }
  }

  private handleItemPickupResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Item pickup failed:", data.error);
      return;
    }
    const actorId = data.actorId as number;
    const itemName = data.itemName as string;
    const message =
      typeof data.message === "string" && data.message.length > 0
        ? data.message
        : `Picked up ${itemName || "item"}.`;
    console.log(`[NetworkBridge] Picked up ${itemName} (actor ${actorId})`);
    useChatStore.getState().addMessage(message, MessageType.LOOT);
    this.playSourceSFX("SFX_GET_ITEM_1", 0.9);

    // Notify Phaser scene to remove the actor sprite
    window.dispatchEvent(
      new CustomEvent("itemPickedUp", { detail: { actorId, itemName } }),
    );
  }

  private handleDialogueChoiceResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Dialogue choice failed:", data.error);
      return;
    }

    const followUpDialogue = data.followUpDialogue as string | undefined;
    if (followUpDialogue && followUpDialogue.length > 0) {
      // Parse and display follow-up dialogue
      import("@/phaser-game/services/DialogueService").then(({ parseDialogueText }) => {
        const lines = parseDialogueText(followUpDialogue);
        if (lines.length > 0) {
          const dialogueStore = usePokemonDialogueStore.getState();
          dialogueStore.openDialogue(lines);
        }
      });
    }

    console.log("[NetworkBridge] Dialogue choice response:", data.choice ? "YES" : "NO");
  }

  private handleElevatorFloorsResponse(data: { floors: Array<{ floorMapId: number; floorLabel: string; destX: number; destY: number }>; message?: string }) {
    console.log("[NetworkBridge] Elevator floors:", data);
    if (data.message) {
      // No accessible floors (e.g. missing LIFT KEY) — show message as dialogue
      const dialogueStore = usePokemonDialogueStore.getState();
      dialogueStore.openDialogue([data.message]);
      return;
    }
    // Dispatch elevator floor list for the UI to display
    window.dispatchEvent(
      new CustomEvent("elevatorFloors", { detail: data.floors })
    );
  }

  private handleRepelEvent(data: { message?: string }) {
    if (data.message) {
      window.dispatchEvent(new CustomEvent("repelMessage", { detail: data }));
    }
  }

  private handleGameCornerEvent(opcode: number, data: unknown) {
    const eventMap: Record<number, string> = {
      [OpCodes.GameCornerCoinBalanceResponse]: "gameCornerCoinBalance",
      [OpCodes.GameCornerSlotResultResponse]: "gameCornerSlotResult",
      [OpCodes.GameCornerPrizeListResponse]: "gameCornerPrizeList",
      [OpCodes.GameCornerPrizeBuyResponse]: "gameCornerPrizeBuy",
      [OpCodes.GameCornerCoinPickupNotify]: "gameCornerCoinPickup",
    };
    const eventName = eventMap[opcode] || "gameCornerUnknown";
    console.log(`[NetworkBridge] Game Corner event: ${eventName}`, data);
    window.dispatchEvent(new CustomEvent(eventName, { detail: data }));
  }

  private handleSafariEvent(eventName: string, data: unknown) {
    console.log(`[NetworkBridge] Safari event: ${eventName}`, data);
    window.dispatchEvent(
      new CustomEvent(eventName, { detail: data })
    );
  }

  private handleSafariBattleStart(data: Record<string, unknown>) {
    console.log("[NetworkBridge] Safari battle start:", data);
    const pokemon = data.pokemon as { id: number; name: string; level: number; hp: number; maxHp: number };
    usePokeBattleStore.getState().startSafariBattle({
      pokemon,
      battleId: data.battleId as string, revision: data.revision as number,
      ballsLeft: data.ballsLeft as number,
      stepsLeft: data.stepsLeft as number,
    });
  }

  private handleWarpTileTeleport(data: ProtocolTypes.WarpTileTeleportNotify) {
    // This opcode follows a committed server destination. Scene loading reads
    // that owned location; it must not submit another destination write.
    window.dispatchEvent(
      new CustomEvent("warpTileTeleport", { detail: { ...data, serverCommitted: true } })
    );
  }

  private handleWarpHomeResponse(data: Record<string, unknown>) {
    const chat = useChatStore.getState();
    if (data.success) {
      this.playSourceSFX("SFX_GO_OUTSIDE", 0.85);
      const battleStore = usePokeBattleStore.getState();
      if (battleStore.isInBattle) {
        battleStore.retireBattle();
      }
      chat.addMessage((data.message as string) || "Warped home.", MessageType.SYSTEM);
    } else {
      chat.addMessage(
        `Warp home failed: ${(data.error as string) || "unknown error"}`,
        MessageType.SYSTEM_ERROR,
      );
    }
  }

  private handleCutsceneStartNotify(data: Record<string, unknown>) {
    import("@/phaser-game/services/CutsceneService").then(({ handleCutsceneStart }) => {
      this.playSourceSFX("SFX_PRESS_AB", 0.45);
      handleCutsceneStart(data as unknown as import("@/phaser-game/services/CutsceneService").CutsceneStartPayload);
    });
  }

  private handlePokedexListResponse(data: ProtocolTypes.PokedexListResponse | ProtocolTypes.ErrorResponse) {
    if (!data.success) return;
    import("@/stores/PokedexStore").then(({ default: usePokedexStore }) => {
      usePokedexStore.getState().setSpecies(data.species);
      usePokedexStore.getState().setStatus(data.status);
    });
  }

  private handlePokedexStatusResponse(data: ProtocolTypes.PokedexStatusResponse | ProtocolTypes.ErrorResponse) {
    if (!data.success) return;
    import("@/stores/PokedexStore").then(({ default: usePokedexStore }) => {
      usePokedexStore.getState().setStatus(data.status);
    });
  }

  private handleTrainerCardResponse(data: ProtocolTypes.TrainerCardResponse | ProtocolTypes.ErrorResponse) {
    if (!data.success) return;
    import("@/stores/PokedexStore").then(({ default: usePokedexStore }) => {
      usePokedexStore.getState().setTrainerCard(data);
    });
  }

  private handleDebugSceneListResponse(data: Record<string, unknown>) {
    import("@/stores/DebugSceneStore").then(({ default: useDebugSceneStore }) => {
      useDebugSceneStore.getState().setScenes(
        (data.scenes as Array<{
          seqNum: number;
          label: string;
          description: string;
          scenarioName: string;
          scenarioJson?: string;
          triggerType: string;
          mapName: string;
          scriptLabel?: string;
          category?: string;
          storyChapter?: string;
          storyOrder?: number;
          storyKind?: string;
          e2eMode?: string;
          driver?: string;
        }>) || []
      );
    });
  }

  private handleDebugSceneJumpResponse(data: Record<string, unknown>) {
    if (data.success) {
      const battleStore = usePokeBattleStore.getState();
      if (battleStore.isInBattle) {
        battleStore.retireBattle();
      }
      import("@/stores/DebugSceneStore").then(({ default: useDebugSceneStore }) => {
        useDebugSceneStore.getState().setLastAppliedScenario({
          label: (data.label as string) || null,
          scenarioName: (data.scenarioName as string) || null,
          scriptLabel: (data.scriptLabel as string) || null,
        });
      });
      console.log(`[DebugScene] Applied scenario: ${data.label}`);
    } else {
      console.error(`[DebugScene] Jump failed: ${data.error}`);
    }
  }

  private handleDebugGivePowerPokemonResponse(data: Record<string, unknown>) {
    import("@/stores/DebugSceneStore").then(({ default: useDebugSceneStore }) => {
      if (data.success) {
        useDebugSceneStore.getState().setPowerPokemonMessage(
          (data.message as string) || "Added power Pokémon."
        );
      } else {
        useDebugSceneStore.getState().setPowerPokemonMessage(
          `Power Pokémon failed: ${(data.error as string) || "unknown error"}`
        );
      }
    });
  }
}

type PokeBattlePokemonDTO = Parameters<
  ReturnType<typeof usePokeBattleStore.getState>["startBattle"]
>[0]["playerPokemon"];

type BattleEventDTO = Parameters<
  ReturnType<typeof usePokeBattleStore.getState>["updateBattleState"]
>[0]["events"][number];
