import { WorldSocket } from "./index";
import * as OpCodes from "./generated/opcodes";
import type { OpCode } from "./generated/opcodes";
import * as WorldTypes from "./generated/world";
import * as ModelTypes from "./generated/models";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import useCharacterSelectStore, {
  type CharacterSelectEntry,
} from "@/stores/CharacterSelectStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import usePokemonPartyStore from "@/stores/PokemonPartyStore";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import usePokemonPCStore from "@/stores/PokemonPCStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import useAudioActivityStore from "@/stores/AudioActivityStore";
import AudioManager, { type GeneratedSFXName } from "@/services/audio/AudioManager";
import {
  cryPathForPokemon,
  sfxPathForConstant,
  victoryMusicTrackForState,
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
  public static send(data: unknown, opcode: OpCode): void {
    WorldSocket.sendStreamJsonMessage(opcode, data);
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
        this.handleCharacterData(data as ModelTypes.CharacterData);
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
      case OpCodes.PhaserTilesResponse:
      case OpCodes.PhaserOverworldMapsResponse:
      case OpCodes.PhaserActorsResponse:
      case OpCodes.PhaserWarpsResponse:
      case OpCodes.PhaserActorPositionUpdate:
      case OpCodes.PhaserActorDespawn:
      case OpCodes.PhaserMapMusicResponse:
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
      case OpCodes.PokeBattleActionResponse:
      case OpCodes.CQBattleItemUseResponse:
      case OpCodes.PokeBattleSwitchResponse:
        this.handlePokeBattleUpdate(data as Record<string, unknown>);
        break;
      case OpCodes.PokeBattleEndNotify:
        this.handlePokeBattleEnd(data as Record<string, unknown>);
        break;

      // Pokémon Party opcodes (Phase 6.1)
      case OpCodes.PokemonPartyResponse:
        this.handlePokemonPartyResponse(data as Record<string, unknown>);
        break;

      // Pokémon Center healing (Phase 6.6)
      case OpCodes.PokeCenterHealResponse:
        this.handlePokeCenterHealResponse(data as Record<string, unknown>);
        break;

      // CQ Inventory & Merchant (Phase 7)
      case OpCodes.CQInventoryResponse:
        this.handleCQInventoryResponse(data as Record<string, unknown>);
        break;
      case OpCodes.CQMerchantOpenResponse:
        this.handleCQMerchantOpenResponse(data as Record<string, unknown>);
        break;
      case OpCodes.CQMerchantBuyResponse:
        this.handleCQMerchantBuyResponse(data as Record<string, unknown>);
        break;
      case OpCodes.CQMerchantSellResponse:
        this.handleCQMerchantSellResponse(data as Record<string, unknown>);
        break;
      case OpCodes.CQItemUseResponse:
        this.handleCQItemUseResponse(data as Record<string, unknown>);
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
      case OpCodes.PokemonPCOpenResponse:
        this.handlePokemonPCOpenResponse(data as Record<string, unknown>);
        break;
      case OpCodes.PokemonPCDepositResponse:
        this.handlePokemonPCDepositResponse(data as Record<string, unknown>);
        break;
      case OpCodes.PokemonPCWithdrawResponse:
        this.handlePokemonPCWithdrawResponse(data as Record<string, unknown>);
        break;
      case OpCodes.PokemonPCReleaseResponse:
        this.handlePokemonPCReleaseResponse(data as Record<string, unknown>);
        break;
      case OpCodes.PokemonPCSwitchBoxResponse:
        this.handlePokemonPCSwitchBoxResponse(data as Record<string, unknown>);
        break;

      // Move learning (Phase 6.2)
      case OpCodes.PokeMoveLearnResponse:
        this.handlePokeMoveLearnResponse(data as Record<string, unknown>);
        break;

      // Party reorder (Phase 6.1)
      case OpCodes.PokemonPartyReorderResponse:
        this.handlePokemonPartyReorderResponse(data as Record<string, unknown>);
        break;

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
        this.handleWarpTileTeleport(
          data as {
            mapId: number;
            x: number;
            y: number;
            direction?: string;
            animateExitStep?: boolean;
            animationStartX?: number;
            animationStartY?: number;
          },
        );
        break;
      case OpCodes.WarpHomeResponse:
        this.handleWarpHomeResponse(data as Record<string, unknown>);
        break;

      // Elevator floor list (Phase 9.7)
      case OpCodes.ElevatorFloorsResponse:
        this.handleElevatorFloorsResponse(data as { floors: Array<{ floorMapId: number; floorLabel: string; destX: number; destY: number }>; message?: string });
        break;

      // Repel (Phase 11.2)
      case OpCodes.RepelUseResponse:
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
      case OpCodes.SafariBattleActionResponse:
        this.handleSafariBattleAction(data as Record<string, unknown>);
        break;
      case OpCodes.SafariZoneStepUpdate:
        this.handleSafariEvent("safariStepUpdate", data);
        break;
      case OpCodes.SafariZoneExitNotify:
        this.handleSafariEvent("safariZoneExit", data);
        break;

      // Pokédex & UI (Phase 10)
      case OpCodes.PokedexListResponse:
        this.handlePokedexListResponse(data as Record<string, unknown>);
        break;
      case OpCodes.PokedexStatusResponse:
        this.handlePokedexStatusResponse(data as Record<string, unknown>);
        break;
      case OpCodes.TrainerCardResponse:
        this.handleTrainerCardResponse(data as Record<string, unknown>);
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
  private handleCharacterData(data: ModelTypes.CharacterData) {
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
    AudioManager.playGeneratedSFX("battleStart", 0.9);
    useAudioActivityStore.getState().setBattleVictoryTrack(null);
    usePokeBattleStore.getState().startBattle({
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

  private playSourceSFX(
    sfxConstant: string,
    volume: number,
    fallback?: GeneratedSFXName,
  ) {
    const path = sfxPathForConstant(sfxConstant);
    if (path) {
      void AudioManager.playSFX(path, volume);
      return;
    }
    if (fallback) {
      void AudioManager.playGeneratedSFX(fallback, volume);
    }
  }

  private handlePokeBattleUpdate(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Battle action failed:", data.error);
      return;
    }
    usePokeBattleStore.getState().updateBattleState({
      playerPokemon: data.playerPokemon as PokeBattlePokemonDTO,
      enemyPokemon: data.enemyPokemon as PokeBattlePokemonDTO,
      phase: data.phase as string,
      turnNumber: data.turnNumber as number,
      events: (data.events || []) as BattleEventDTO[],
      playerParty: data.playerParty as PokeBattlePokemonDTO[] | undefined,
      playerActive: data.playerActive as number | undefined,
      battleType: data.battleType as string | undefined,
      allowedActions: data.allowedActions as string[] | undefined,
      guaranteedCatch: data.guaranteedCatch as boolean | undefined,
    });
  }

  private handlePokeBattleEnd(data: Record<string, unknown>) {
    const playerWon = data.playerWon as boolean;
    if (playerWon) {
      const battleState = usePokeBattleStore.getState();
      useAudioActivityStore
        .getState()
        .setBattleVictoryTrack(
          victoryMusicTrackForState(
            battleState.battleType,
            battleState.trainerClass,
          ),
        );
    }
    const blackoutWarp = !playerWon && data.blackoutMapId
      ? {
          mapId: data.blackoutMapId as number,
          x: data.blackoutX as number,
          y: data.blackoutY as number,
        }
      : undefined;
    const sentToPC = data.sentToPC as boolean | undefined;
    const sentToPCBox = data.pcBox as number | undefined;
    const lossMessage = data.lossMessage as string | undefined;
    usePokeBattleStore.getState().endBattle(playerWon, blackoutWarp, sentToPC, sentToPCBox, lossMessage);
  }

  private handlePokeCenterHealResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Pokémon Center heal failed:", data.error);
      return;
    }
    this.playSourceSFX("SFX_HEALING_MACHINE", 0.9, "heal");
    // Don't update party store here — the client will request fresh party data
    // after the Nurse Joy dialogue finishes (via the onClose callback).
    console.log("[NetworkBridge] Pokémon Center heal confirmed by server");
  }

  private handlePokemonPartyResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Party request failed:", data.error);
      return;
    }
    const party = (data.party || []) as Parameters<
      ReturnType<typeof usePokemonPartyStore.getState>["setParty"]
    >[0];
    usePokemonPartyStore.getState().setParty(party);
  }

  private handleCQInventoryResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Inventory request failed:", data.error);
      return;
    }
    const items = (data.items || []) as Parameters<
      ReturnType<typeof useCQInventoryStore.getState>["setInventory"]
    >[0];
    const money = (data.money || 0) as number;
    useCQInventoryStore.getState().setInventory(items, money);
  }

  private handleCQMerchantOpenResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Merchant open failed:", data.error);
      return;
    }
    const merchantId = data.merchantId as number;
    const name = data.name as string;
    const items = (data.items || []) as Parameters<
      ReturnType<typeof useCQInventoryStore.getState>["openShop"]
    >[2];
    const money = (data.money || 0) as number;
    useCQInventoryStore.getState().openShop(merchantId, name, items, money);
  }

  private handleCQMerchantBuyResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Buy failed:", data.error);
      return;
    }
    this.playSourceSFX("SFX_PURCHASE", 0.8, "confirm");
    useCQInventoryStore.getState().updateAfterBuy(
      data.itemId as number,
      data.quantity as number,
      data.instanceId as number,
      data.money as number,
      data.item as Parameters<
        ReturnType<typeof useCQInventoryStore.getState>["updateAfterBuy"]
      >[4],
    );
    console.log("[NetworkBridge] Bought item, money:", data.money);
  }

  private handleCQMerchantSellResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Sell failed:", data.error);
      return;
    }
    this.playSourceSFX("SFX_PURCHASE", 0.8, "confirm");
    useCQInventoryStore.getState().updateAfterSell(
      data.instanceId as number,
      data.money as number,
    );
    console.log("[NetworkBridge] Sold", data.itemName, "for", data.sellPrice);
  }

  private handleCQItemUseResponse(data: Record<string, unknown>) {
    if (!data.success) {
      const error = String(data.error || "It won't have any effect");
      useChatStore.getState().addMessage(error, MessageType.SYSTEM);
      console.warn("[NetworkBridge] Item use failed:", data.error);
      this.playSourceSFX("SFX_DENIED", 0.8, "error");
      return;
    }

    // TM/HM needs move slot selection — signal the UI via store
    if (data.needsMoveSlot) {
      const instanceId = Number(data.instanceId);
      const partySlot = Number(data.partySlot);
      if (!Number.isInteger(instanceId) || !Number.isInteger(partySlot)) {
        const error = "Unable to choose a move for that TM/HM. Please try again.";
        useChatStore.getState().addMessage(error, MessageType.SYSTEM);
        console.warn("[NetworkBridge] TM/HM move-slot response missing context:", data);
        this.playSourceSFX("SFX_DENIED", 0.8, "error");
        return;
      }
      const store = useCQInventoryStore.getState();
      const inv = store.items.find((i) => i.instance.id === instanceId);
      const moveId = Number(data.moveId);
      const moveName = typeof data.moveName === "string" ? data.moveName : undefined;
      store.setPendingTMHM({
        instanceId,
        partySlot,
        itemName: inv?.item.name || "TM/HM",
        moveId: Number.isInteger(moveId) ? moveId : undefined,
        moveName,
        message: data.message as string,
      });
      console.log("[NetworkBridge] TM/HM needs move slot:", data.message);
      this.playSourceSFX("SFX_PRESS_AB", 0.6, "confirm");
      return;
    }

    // Update inventory quantity (or remove if depleted)
    const instanceId = data.instanceId as number;
    const newQty = data.newQty as number;
    const store = useCQInventoryStore.getState();
    if (newQty !== undefined && newQty <= 0) {
      store.updateAfterSell(instanceId, store.money); // reuse removal logic
    } else if (newQty !== undefined) {
      // Update quantity on the existing item
      const items = store.items.map((i) =>
        i.instance.id === instanceId
          ? { ...i, instance: { ...i.instance, quantity: newQty } }
          : i,
      );
      useCQInventoryStore.setState({ items });
    }

    // Clear any pending TM/HM state on success
    store.setPendingTMHM(null);

    const bicycle = data.bicycle as
      | {
          wantsRiding?: boolean;
          activeRiding?: boolean;
          forcedRiding?: boolean;
          WantsRiding?: boolean;
          ActiveRiding?: boolean;
          ForcedRiding?: boolean;
        }
      | undefined;
    if (bicycle) {
      useAudioActivityStore
        .getState()
        .setBicycleState({
          wantsRiding: Boolean(bicycle.wantsRiding ?? bicycle.WantsRiding),
          activeRiding: Boolean(bicycle.activeRiding ?? bicycle.ActiveRiding),
          forcedRiding: Boolean(bicycle.forcedRiding ?? bicycle.ForcedRiding),
        });
    }

    if (data.message) {
      useChatStore.getState().addMessage(String(data.message), MessageType.SYSTEM);
    }

    this.playSourceSFX("SFX_PRESS_AB", 0.75, "confirm");
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
      this.playSourceSFX("SFX_DENIED", 0.8, "error");
      return;
    }
    if (message) {
      usePokemonDialogueStore.getState().openDialogue([message]);
      useChatStore.getState().addMessage(message, MessageType.SYSTEM);
    }
    if (data.hooked) {
      console.log("[NetworkBridge] Fishing: hooked a Pokémon!");
      this.playSourceSFX("SFX_PRESS_AB", 0.75, "confirm");
      // Battle start will arrive via PokeBattleStartResponse
    } else {
      console.log("[NetworkBridge] Fishing:", data.message);
      this.playSourceSFX("SFX_DENIED", 0.6, "error");
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
        }
      | undefined;
    if (tile) {
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

  private handlePokemonPartyReorderResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Party reorder failed:", data.error);
      return;
    }
    const party = (data.party || []) as Parameters<
      ReturnType<typeof usePokemonPartyStore.getState>["setParty"]
    >[0];
    usePokemonPartyStore.getState().setParty(party);
    this.playSourceSFX("SFX_PRESS_AB", 0.55, "confirm");
    console.log("[NetworkBridge] Party reordered successfully");
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
    this.playSourceSFX("SFX_GET_ITEM_1", 0.9, "itemPickup");

    // Notify Phaser scene to remove the actor sprite
    window.dispatchEvent(
      new CustomEvent("itemPickedUp", { detail: { actorId, itemName } }),
    );
  }

  private handlePokemonPCOpenResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] PC open failed:", data.error);
      return;
    }
    const pcData = data as {
      currentBox: number;
      boxCount: number;
      boxSize: number;
      box: PCPokemonDTO[];
      party: PCPokemonDTO[];
    };
    usePokemonPCStore.getState().openPC(pcData);
    this.playSourceSFX("SFX_TURN_ON_PC", 0.7, "confirm");
    console.log(`[NetworkBridge] PC opened, box ${pcData.currentBox} with ${pcData.box.length} Pokémon`);
  }

  private handlePokemonPCDepositResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] PC deposit failed:", data.error);
      return;
    }
    console.log("[NetworkBridge] Pokémon deposited to PC");
  }

  private handlePokemonPCWithdrawResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] PC withdraw failed:", data.error);
      return;
    }
    console.log("[NetworkBridge] Pokémon withdrawn from PC");
  }

  private handlePokemonPCReleaseResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] PC release failed:", data.error);
      return;
    }
    console.log("[NetworkBridge] Pokémon released from PC");
  }

  private handlePokemonPCSwitchBoxResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] PC switch box failed:", data.error);
      return;
    }
    const currentBox = data.currentBox as number;
    const box = (data.box || []) as PCPokemonDTO[];
    const party = data.party as PCPokemonDTO[] | undefined;
    if (party) {
      // After deposit/withdraw: update box, party, and currentBox together
      usePokemonPCStore.setState({ currentBox, boxPokemon: box, party });
    } else {
      // Normal box switch: only update box contents
      usePokemonPCStore.getState().setBox(currentBox, box);
    }
    console.log(`[NetworkBridge] Switched to box ${currentBox} with ${box.length} Pokémon`);
  }

  private handlePokeMoveLearnResponse(data: Record<string, unknown>) {
    if (!data.success) {
      console.warn("[NetworkBridge] Move learn failed:", data.error);
      return;
    }
    this.playSourceSFX("SFX_GET_ITEM_1", 0.75, "itemPickup");
    const message = data.message as string;
    const skipped = data.skipped as boolean;
    console.log("[NetworkBridge] Move learn:", skipped ? "skipped" : "learned", message);

    // Build the event list: move learn confirmation message + any post-events
    // (trainer dialogue, prize money) that were deferred.
    const allEvents: BattleEventDTO[] = [{ type: "message" as const, message }];
    const postEvents = data.postEvents as BattleEventDTO[] | undefined;
    if (postEvents && postEvents.length > 0) {
      allEvents.push(...postEvents);
    }

    // Show events via the normal animation flow, then transition to battle_end.
    const store = usePokeBattleStore.getState();
    store.updateBattleState({
      playerPokemon: (data.updatedPokemon as PokeBattlePokemonDTO) || store.playerPokemon!,
      enemyPokemon: store.enemyPokemon!,
      phase: "battle_end",
      turnNumber: store.turnNumber,
      events: allEvents,
    });
    // The move learn prompt only fires when the player won, so always set "win".
    store.endBattle(true);
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
      ballsLeft: data.ballsLeft as number,
      stepsLeft: data.stepsLeft as number,
    });
  }

  private handleSafariBattleAction(data: Record<string, unknown>) {
    console.log("[NetworkBridge] Safari battle action:", data);
    usePokeBattleStore.getState().updateSafariState({
      events: (data.events || []) as BattleEventDTO[],
      ballsLeft: data.ballsLeft as number,
      stepsLeft: data.stepsLeft as number,
      isOver: data.isOver as boolean,
      caught: data.caught as boolean,
      fled: data.fled as boolean,
      caughtPokemon: data.caughtPokemon as { name: string } | undefined,
      sentToPC: data.sentToPC as boolean | undefined,
      pcBox: data.pcBox as number | undefined,
    });
  }

  private handleWarpTileTeleport(data: {
    mapId: number;
    x: number;
    y: number;
    direction?: string;
    animateExitStep?: boolean;
    animationStartX?: number;
    animationStartY?: number;
  }) {
    console.log("[NetworkBridge] Warp tile teleport:", data);
    // Dispatch a custom event that TileViewer listens for
    window.dispatchEvent(
      new CustomEvent("warpTileTeleport", { detail: data })
    );
  }

  private handleWarpHomeResponse(data: Record<string, unknown>) {
    const chat = useChatStore.getState();
    if (data.success) {
      this.playSourceSFX("SFX_GO_OUTSIDE", 0.85, "warp");
      const battleStore = usePokeBattleStore.getState();
      if (battleStore.isInBattle) {
        battleStore.closeBattle();
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
      this.playSourceSFX("SFX_PRESS_AB", 0.45, "confirm");
      handleCutsceneStart(data as unknown as import("@/phaser-game/services/CutsceneService").CutsceneStartPayload);
    });
  }

  private handlePokedexListResponse(data: Record<string, unknown>) {
    if (!data.success) return;
    import("@/stores/PokedexStore").then(({ default: usePokedexStore }) => {
      const species = (data.species || []) as Array<{
        id: number; name: string; type1: string; type2?: string;
        pokedexType?: string; height?: string; weight?: number;
        pokedexText?: string; iconImage?: string;
        crySfx?: string; cryPitch?: number; cryLength?: number;
      }>;
      const status = (data.status || []) as Array<{
        pokemonId: number; seen: boolean; caught: boolean;
      }>;
      usePokedexStore.getState().setSpecies(species);
      usePokedexStore.getState().setStatus(status);
    });
  }

  private handlePokedexStatusResponse(data: Record<string, unknown>) {
    if (!data.success) return;
    import("@/stores/PokedexStore").then(({ default: usePokedexStore }) => {
      const status = (data.status || []) as Array<{
        pokemonId: number; seen: boolean; caught: boolean;
      }>;
      usePokedexStore.getState().setStatus(status);
    });
  }

  private handleTrainerCardResponse(data: Record<string, unknown>) {
    if (!data.success) return;
    import("@/stores/PokedexStore").then(({ default: usePokedexStore }) => {
      usePokedexStore.getState().setTrainerCard({
        name: data.name as string,
        money: data.money as number,
        timePlayed: data.timePlayed as number,
        badges: (data.badges || []) as string[],
        badgeCount: data.badgeCount as number,
        pokedexSeen: data.pokedexSeen as number,
        pokedexCaught: data.pokedexCaught as number,
      });
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
        }>) || []
      );
    });
  }

  private handleDebugSceneJumpResponse(data: Record<string, unknown>) {
    if (data.success) {
      const battleStore = usePokeBattleStore.getState();
      if (battleStore.isInBattle) {
        battleStore.closeBattle();
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

type PCPokemonDTO = Parameters<
  ReturnType<typeof usePokemonPCStore.getState>["openPC"]
>[0]["box"][number];
