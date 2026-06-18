import { Scene } from "phaser";
import { PhaserActor } from "@/net/generated/world_api";
import useGameStatusStore from "@/stores/GameStatusStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import useSlotMachineStore from "@/stores/SlotMachineStore";
import { TILE_SIZE } from "../../constants";
import { CameraController } from "../../controllers/CameraController";
import { PlayerMovementController } from "../../controllers/PlayerMovementController";
import { WarpManager } from "../../managers";
import { MapRenderer } from "../../renderers/MapRenderer";
import {
  fetchDialogueWithBranching,
  parseDialogueText,
} from "../../services/DialogueService";
import {
  sendDialogueChoice,
} from "../../services/PhaserNetworkService";
import * as PhaserNet from "../../services/PhaserNetworkService";
import { TileViewerOverlays } from "./TileViewerOverlays";

type MovementDirection = "UP" | "DOWN" | "LEFT" | "RIGHT";

const POKEMON_CENTER_MAP_IDS = new Set([
  41, 58, 64, 68, 81, 89, 133, 141, 154, 171, 182,
]);
const POKEMON_CENTER_PC_ACCESS_TILE = { x: 13, y: 4 };
const POKEMON_CENTER_PC_CLICK_TILES = [
  { x: 13, y: 3 },
  POKEMON_CENTER_PC_ACCESS_TILE,
] as const;

interface TileViewerInteractionDeps {
  scene: Scene;
  mapRenderer: () => MapRenderer;
  cameraController: () => CameraController;
  playerMovementController: () => PlayerMovementController;
  warpManager: () => WarpManager;
  overlays: () => TileViewerOverlays;
  getPlayerActor: () => PhaserActor | null;
  actors: () => PhaserActor[];
  viewedMapIds: () => Set<number>;
  currentActorById: (actorId: number) => PhaserActor | null;
  isWorldInputFrozen: () => boolean;
  handleTileEditorClick: (worldX: number, worldY: number) => void;
}

export class TileViewerInteractionController {
  private cursors?: Phaser.Types.Input.Keyboard.CursorKeys;
  private wasdKeys?: {
    W: Phaser.Input.Keyboard.Key;
    A: Phaser.Input.Keyboard.Key;
    S: Phaser.Input.Keyboard.Key;
    D: Phaser.Input.Keyboard.Key;
  };
  private spaceKey?: Phaser.Input.Keyboard.Key;
  private suppressNextWorldPointerUp = false;
  private readonly pointerUpHandler = (pointer: Phaser.Input.Pointer) => {
    this.handlePointerUp(pointer);
  };
  private readonly actorClickHandler = (actor: PhaserActor) => {
    void this.handleActorClicked(actor);
  };
  private readonly mapInteractivePointerDownHandler = () => {
    this.suppressNextWorldPointerUp = true;
  };
  private readonly focusedButtonSpaceHandler = (event: KeyboardEvent) => {
    this.handleFocusedButtonSpace(event);
  };

  constructor(private readonly deps: TileViewerInteractionDeps) {}

  setup(): void {
    this.setupKeyboardInput();
    this.deps.scene.input.on("pointerup", this.pointerUpHandler);
    this.deps.scene.events.on(
      "actorClicked",
      this.actorClickHandler,
    );
    this.deps.scene.events.on(
      "mapInteractivePointerDown",
      this.mapInteractivePointerDownHandler,
    );
    window.addEventListener("keydown", this.focusedButtonSpaceHandler, true);
  }

  cleanup(): void {
    this.deps.scene.input.off("pointerup", this.pointerUpHandler);
    this.deps.scene.events.off("actorClicked", this.actorClickHandler);
    this.deps.scene.events.off(
      "mapInteractivePointerDown",
      this.mapInteractivePointerDownHandler,
    );
    window.removeEventListener("keydown", this.focusedButtonSpaceHandler, true);
    this.suppressNextWorldPointerUp = false;
  }

  update(): void {
    this.handleKeyboardInteract();

    const movement = this.deps.playerMovementController();
    if (!movement.getIsMoving()) {
      const dir = this.getKeyboardDirection();
      if (dir) {
        movement.handleKeyboardMove(dir);
      }
    }

    if (
      this.cursors &&
      !useGameStatusStore.getState().isCameraFollowEnabled &&
      !this.isTextInputFocused()
    ) {
      this.deps.cameraController().update(this.cursors);
    }
  }

  getKeyboardDirection(): MovementDirection | null {
    if (!this.deps.scene.input.keyboard) return null;
    if (this.isTextInputFocused()) return null;
    if (this.deps.isWorldInputFrozen()) return null;

    if (this.wasdKeys) {
      if (this.wasdKeys.W.isDown) return "UP";
      if (this.wasdKeys.S.isDown) return "DOWN";
      if (this.wasdKeys.A.isDown) return "LEFT";
      if (this.wasdKeys.D.isDown) return "RIGHT";
    }

    if (this.cursors && useGameStatusStore.getState().isCameraFollowEnabled) {
      if (this.cursors.up.isDown) return "UP";
      if (this.cursors.down.isDown) return "DOWN";
      if (this.cursors.left.isDown) return "LEFT";
      if (this.cursors.right.isDown) return "RIGHT";
    }

    return null;
  }

  private setupKeyboardInput(): void {
    if (!this.deps.scene.input.keyboard) return;

    this.cursors = this.deps.scene.input.keyboard.createCursorKeys();
    this.wasdKeys = {
      W: this.deps.scene.input.keyboard.addKey(Phaser.Input.Keyboard.KeyCodes.W, false),
      A: this.deps.scene.input.keyboard.addKey(Phaser.Input.Keyboard.KeyCodes.A, false),
      S: this.deps.scene.input.keyboard.addKey(Phaser.Input.Keyboard.KeyCodes.S, false),
      D: this.deps.scene.input.keyboard.addKey(Phaser.Input.Keyboard.KeyCodes.D, false),
    };
    this.spaceKey = this.deps.scene.input.keyboard.addKey(
      Phaser.Input.Keyboard.KeyCodes.SPACE,
      false,
    );
    this.deps.scene.input.keyboard.removeCapture([
      Phaser.Input.Keyboard.KeyCodes.UP,
      Phaser.Input.Keyboard.KeyCodes.DOWN,
      Phaser.Input.Keyboard.KeyCodes.LEFT,
      Phaser.Input.Keyboard.KeyCodes.RIGHT,
      Phaser.Input.Keyboard.KeyCodes.SPACE,
    ]);
  }

  private handlePointerUp(pointer: Phaser.Input.Pointer): void {
    if (this.suppressNextWorldPointerUp) {
      this.suppressNextWorldPointerUp = false;
      return;
    }
    if (this.deps.isWorldInputFrozen()) return;
    if (pointer.getDistance() >= 10) return;

    const worldPoint = this.deps.scene.cameras.main.getWorldPoint(
      pointer.x,
      pointer.y,
    );

    if (useGameStatusStore.getState().isWarpMode) {
      this.handleWarpModeClick(worldPoint.x, worldPoint.y);
      return;
    }

    if (useGameStatusStore.getState().isTileManagerOpen) {
      this.deps.handleTileEditorClick(worldPoint.x, worldPoint.y);
      return;
    }

    const clickTileX = Math.floor(worldPoint.x / TILE_SIZE);
    const clickTileY = Math.floor(worldPoint.y / TILE_SIZE);
    if (this.isPokemonCenterPCClickTile(clickTileX, clickTileY)) {
      this.requestPokemonPCInteraction();
      return;
    }

    const gameCornerMapId = 135;
    if (
      this.deps.viewedMapIds().has(gameCornerMapId) &&
      this.deps.overlays().isSlotMachineTile(clickTileX, clickTileY)
    ) {
      const playerPos = this.deps
        .playerMovementController()
        .getCurrentPosition();
      const dist =
        Math.abs(playerPos.x - clickTileX) + Math.abs(playerPos.y - clickTileY);
      if (dist <= 2) {
        const isLucky = Math.random() < 0.125;
        useSlotMachineStore.getState().openSlotMachine(isLucky);
        return;
      }
    }

    const clickedWarp = this.deps.warpManager().getWarpAt(clickTileX, clickTileY);
    if (clickedWarp) {
      this.deps.scene.events.emit("warpClicked", clickedWarp);
      return;
    }

    this.deps.playerMovementController().handleTileClick(
      worldPoint.x,
      worldPoint.y,
    );
  }

  private handleWarpModeClick(worldX: number, worldY: number): void {
    const tileX = Math.floor(worldX / TILE_SIZE);
    const tileY = Math.floor(worldY / TILE_SIZE);
    const playerActor = this.deps.getPlayerActor();
    const mapId = playerActor?.mapId ?? 0;
    const playerId = playerActor?.id;
    const movement = this.deps.playerMovementController();
    const oldPos = movement.getCurrentPosition();

    PhaserNet.sendPlayerPosition(tileX, tileY, mapId);
    movement.stopMovement();
    movement.syncPosition(tileX, tileY);
    if (playerId != null) {
      this.deps.mapRenderer().updateActorPosition(
        playerId,
        oldPos.x,
        oldPos.y,
        tileX,
        tileY,
        "DOWN",
      );
    }
    const gameStatus = useGameStatusStore.getState();
    gameStatus.setWarpMode(false);
    gameStatus.setCameraFollowEnabled(true);
  }

  private async handleActorClicked(actor: PhaserActor): Promise<void> {
    if (this.deps.isWorldInputFrozen()) return;
    if (!(await this.ensureActorInteractionReachable(actor))) return;
    await this.performActorInteraction(actor);
  }

  private async performActorInteraction(actor: PhaserActor): Promise<void> {
    if (actor.spriteName === "SPRITE_NURSE") {
      console.log(
        `[TileViewer] Nurse Joy clicked on map ${actor.mapId}, triggering heal`,
      );
      PhaserNet.sendPokeCenterHeal(actor.mapId);
      const healLines = [
        "Welcome to our POKéMON CENTER!",
        "We'll restore your POKéMON to full health.",
        "...",
        "Thank you for waiting.\nYour POKéMON are fully healed!",
      ];
      usePokemonDialogueStore
        .getState()
        .openDialogue(healLines, "NURSE JOY", actor.id, () => {
          PhaserNet.sendPokemonPartyRequest();
        });
      return;
    }

    if (actor.objectType === "pc") {
      console.log(`[TileViewer] PC clicked on map ${actor.mapId}`);
      PhaserNet.sendPokemonPCOpen();
      return;
    }

    if (this.isBikeShopClerk(actor)) {
      const startedScript = await PhaserNet.tryScriptedEventInteraction(actor.id);
      if (startedScript) return;

      console.warn("[TileViewer] Bike Shop clerk has no eligible scripted event");
      return;
    }

    if (actor.spriteName === "SPRITE_CLERK") {
      const startedScript = await PhaserNet.tryScriptedEventInteraction(actor.id);
      if (startedScript) return;

      console.log(`[TileViewer] Clerk clicked on map ${actor.mapId}, opening shop`);
      PhaserNet.sendCQMerchantOpenByMap(actor.mapId);
      return;
    }

    if (actor.mapId === 135 && actor.objectType === "sign") {
      console.log("[TileViewer] Game Corner slot machine clicked");
      void import("@/phaser-game/services/PhaserNetworkService").then(
        (network) => {
          network.requestCoinBalance();
        },
      );
      const onBalance = (event: Event) => {
        const data = (event as CustomEvent).detail as { coins: number };
        window.removeEventListener("gameCornerCoinBalance", onBalance);
        this.deps.overlays().showSlotMachineUI(data.coins);
      };
      window.addEventListener("gameCornerCoinBalance", onBalance);
      return;
    }

    const elevatorMapIds = [127, 203, 236];
    if (actor.objectType === "sign" && elevatorMapIds.includes(actor.mapId)) {
      console.log(`[TileViewer] Elevator panel clicked on map ${actor.mapId}`);
      PhaserNet.requestElevatorFloors(actor.mapId);
      return;
    }

    const startedScript = await PhaserNet.tryScriptedEventInteraction(actor.id);
    if (startedScript) return;

    if (actor.trainerClass && actor.trainerPartyIndex !== undefined) {
      const trainer = await PhaserNet.requestTrainerInteraction(actor.id);
      if (trainer?.success) {
        const lines = trainer.dialogue
          ? parseDialogueText(trainer.dialogue)
          : [];
        const startBattle = () => {
          if (trainer.shouldBattle) {
            PhaserNet.sendTrainerBattleStart(actor.id);
          }
        };
        if (lines.length > 0) {
          usePokemonDialogueStore
            .getState()
            .openDialogue(lines, null, actor.id, startBattle);
        } else {
          startBattle();
        }
        return;
      }
    }

    if (actor.objectType === "item") {
      console.log(
        `[TileViewer] Item ball clicked: ${actor.name} (actor ${actor.id})`,
      );
      PhaserNet.sendItemPickup(actor.id);
      return;
    }

    if (!actor.text) {
      console.log(`[TileViewer] Actor ${actor.name} has no text constant`);
      return;
    }

    console.log(
      `[TileViewer] Actor clicked: ${actor.name} (${actor.objectType}), text: ${actor.text}`,
    );

    const result = await fetchDialogueWithBranching(actor.text);
    if (result.lines.length === 0) {
      console.warn(`[TileViewer] No dialogue found for ${actor.text}`);
      return;
    }

    const speakerName = actor.objectType === "sign" ? null : actor.name || null;
    if (result.hasBranching && result.branchingPrompt) {
      const textConstant = actor.text;
      const actorId = actor.id;
      const prompt = result.branchingPrompt;
      usePokemonDialogueStore
        .getState()
        .openDialogue(result.lines, speakerName, actor.id, () => {
          usePokemonDialogueStore.getState().showChoice(prompt, (yes) => {
            sendDialogueChoice(textConstant, yes, actorId);
          });
        });
      return;
    }

    usePokemonDialogueStore
      .getState()
      .openDialogue(result.lines, speakerName, actor.id);
  }

  private handleKeyboardInteract(): void {
    if (!this.spaceKey) return;
    if (!Phaser.Input.Keyboard.JustDown(this.spaceKey)) return;
    if (this.isTextInputFocused()) return;
    if (this.deps.isWorldInputFrozen()) return;
    if (this.deps.playerMovementController().getIsMoving()) return;

    if (this.isStandingOnPokemonCenterPCAccessTile()) {
      PhaserNet.sendPokemonPCOpen();
      return;
    }

    const actor = this.findInteractableActorInFront();
    if (!actor) {
      this.deps.playerMovementController().handleFieldMoveInteractionInFront();
      return;
    }

    this.deps.scene.events.emit("actorClicked", actor);
  }

  private handleFocusedButtonSpace(event: KeyboardEvent): void {
    if (event.key !== " " && event.code !== "Space") return;

    const activeEl = document.activeElement;
    if (!(activeEl instanceof HTMLElement)) return;
    if (!this.isButtonLikeElement(activeEl)) return;

    event.preventDefault();
    activeEl.blur();
  }

  private isTextInputFocused(): boolean {
    const activeEl = document.activeElement;
    if (!activeEl) return false;
    return (
      activeEl.tagName === "INPUT" ||
      activeEl.tagName === "TEXTAREA" ||
      (activeEl instanceof HTMLElement && activeEl.isContentEditable)
    );
  }

  private isButtonLikeElement(element: HTMLElement): boolean {
    if (element.tagName === "BUTTON") return true;
    if (element.getAttribute("role") === "button") return true;
    if (element.tagName !== "INPUT") return false;

    const type = (element.getAttribute("type") || "text").toLowerCase();
    return ["button", "submit", "reset", "image"].includes(type);
  }

  private isBikeShopClerk(actor: PhaserActor): boolean {
    return (
      actor.spriteName === "SPRITE_BIKE_SHOP_CLERK" ||
      actor.text === "TEXT_BIKESHOP_CLERK" ||
      actor.name === "BikeShop_NPC_1"
    );
  }

  private isPokemonCenterMap(): boolean {
    const mapId = this.deps.getPlayerActor()?.mapId ?? null;
    if (mapId != null && POKEMON_CENTER_MAP_IDS.has(mapId)) return true;

    for (const viewedMapId of this.deps.viewedMapIds()) {
      if (POKEMON_CENTER_MAP_IDS.has(viewedMapId)) return true;
    }
    return false;
  }

  private isPokemonCenterPCClickTile(x: number, y: number): boolean {
    return (
      this.isPokemonCenterMap() &&
      POKEMON_CENTER_PC_CLICK_TILES.some((tile) => tile.x === x && tile.y === y)
    );
  }

  private isStandingOnPokemonCenterPCAccessTile(): boolean {
    if (!this.isPokemonCenterMap()) return false;

    const movement = this.deps.playerMovementController();
    const position = movement.getCurrentPosition();
    return (
      position.x === POKEMON_CENTER_PC_ACCESS_TILE.x &&
      position.y === POKEMON_CENTER_PC_ACCESS_TILE.y
    );
  }

  private requestPokemonPCInteraction(): void {
    const movement = this.deps.playerMovementController();
    const openPC = () => {
      movement.faceTile(
        POKEMON_CENTER_PC_ACCESS_TILE.x,
        POKEMON_CENTER_PC_ACCESS_TILE.y - 1,
      );
      PhaserNet.sendPokemonPCOpen();
    };

    const player = movement.getCurrentPosition();
    if (
      player.x === POKEMON_CENTER_PC_ACCESS_TILE.x &&
      player.y === POKEMON_CENTER_PC_ACCESS_TILE.y
    ) {
      openPC();
      return;
    }

    const pathing = movement.requestPathToTile(
      POKEMON_CENTER_PC_ACCESS_TILE.x,
      POKEMON_CENTER_PC_ACCESS_TILE.y,
      openPC,
    );

    if (!pathing) {
      console.warn("[TileViewer] Pokemon Center PC is not reachable from here");
    }
  }

  private findInteractableActorInFront(): PhaserActor | null {
    const movement = this.deps.playerMovementController();
    const position = movement.getCurrentPosition();
    const direction = movement.getCurrentDirection();
    const delta = this.directionDelta(direction);
    if (!delta) return null;

    const targetX = position.x + delta.dx;
    const targetY = position.y + delta.dy;
    const actorTiles = [{ x: targetX, y: targetY }];
    if (movement.isTalkOverTile(targetX, targetY)) {
      actorTiles.push({ x: targetX + delta.dx, y: targetY + delta.dy });
    }

    return (
      this.deps.actors().find((actor) => {
        if (!this.shouldPathBeforeActorInteraction(actor)) return false;
        const actorPosition = this.deps
          .mapRenderer()
          .getActorTilePosition(actor.id);
        const actorX = actorPosition?.x ?? actor.x;
        const actorY = actorPosition?.y ?? actor.y;
        return actorTiles.some((tile) => actorX === tile.x && actorY === tile.y);
      }) ?? null
    );
  }

  private directionDelta(direction: string): { dx: number; dy: number } | null {
    switch (direction.toUpperCase()) {
      case "UP":
        return { dx: 0, dy: -1 };
      case "DOWN":
        return { dx: 0, dy: 1 };
      case "LEFT":
        return { dx: -1, dy: 0 };
      case "RIGHT":
        return { dx: 1, dy: 0 };
      default:
        return null;
    }
  }

  private async ensureActorInteractionReachable(actor: PhaserActor): Promise<boolean> {
    if (!this.shouldPathBeforeActorInteraction(actor)) {
      return true;
    }

    const actorId = actor.id;
    const currentActor = this.deps.currentActorById(actorId) ?? actor;
    if (currentActor.x == null || currentActor.y == null) {
      return true;
    }

    const movement = this.deps.playerMovementController();
    if (movement.canInteractWithTile(currentActor.x, currentActor.y)) {
      movement.faceInteractionTarget(currentActor.x, currentActor.y);
      return true;
    }

    movement.requestInteractionPathToMovingTarget(
      () => {
        const latestActor = this.deps.currentActorById(actorId) ?? actor;
        if (latestActor.x == null || latestActor.y == null) {
          return null;
        }
        return { x: latestActor.x, y: latestActor.y };
      },
      () => {
        void this.performActorInteraction(
          this.deps.currentActorById(actorId) ?? actor,
        );
      },
    );
    return false;
  }

  private shouldPathBeforeActorInteraction(actor: PhaserActor): boolean {
    if (actor.objectType === "player") return false;

    return Boolean(
      actor.objectType === "item" ||
        actor.objectType === "npc" ||
        actor.objectType === "sign" ||
        actor.objectType === "pc" ||
        actor.text ||
        actor.trainerClass,
    );
  }
}
