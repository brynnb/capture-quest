import {bindSafariVisitView} from "../../services/SafariVisitService";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import { Scene } from "phaser";
import { PhaserActor } from "@/net/generated/world_api";
import { TileViewerOverlays } from "./TileViewerOverlays";
import { MapRenderer } from "../../renderers/MapRenderer";

interface TileViewerEventBridgeDeps {
  scene: Scene;
  overlays: TileViewerOverlays;
  mapRenderer: () => MapRenderer | null;
  actors: () => PhaserActor[];
  resetScene: (resetCamera: boolean) => void;
}

export class TileViewerEventBridge {
  private safariUnsubscribe:(()=>void)|null=null;
  private elevatorFloorsHandler: ((e: Event) => void) | null = null;



  private gameCornerPrizeHandler: ((e: Event) => void) | null = null;
  private gameCornerBuyHandler: ((e: Event) => void) | null = null;
  private gameCornerCoinHandler: ((e: Event) => void) | null = null;
  private chatBubbleHandler: ((e: Event) => void) | null = null;

  constructor(private readonly deps: TileViewerEventBridgeDeps) {}

  register(): void {
    this.elevatorFloorsHandler = (event: Event) => {
      const floors = (event as CustomEvent).detail as Array<{
        floorMapId: number;
        floorLabel: string;
      }>;
      console.log(`[Elevator] Received ${floors.length} floors`);
      this.deps.overlays.showElevatorMenu(floors);
    };
    window.addEventListener("elevatorFloors", this.elevatorFloorsHandler);

 this.safariUnsubscribe=bindSafariVisitView(view=>{if(view?.active)this.deps.overlays.updateSafariHUD(view.stepsLeft,view.ballsLeft);else this.deps.overlays.destroySafariHUD();});

    this.gameCornerPrizeHandler = (event: Event) => {
      const data = (event as CustomEvent).detail;
      console.log("[GameCorner] Prize list:", data);
      this.deps.overlays.showPrizeMenu(data);
    };
    window.addEventListener("gameCornerPrizeList", this.gameCornerPrizeHandler);

    this.gameCornerBuyHandler = (event: Event) => {
      const data = (event as CustomEvent).detail as {
        success: boolean;
        coins: number;
        prizeName?: string;
        message?: string;
        error?: string;
      };
      console.log("[GameCorner] Prize buy:", data);
      this.deps.overlays.closeGameCornerUI();
      usePokemonDialogueStore
        .getState()
        .openDialogue([data.message || data.error || "Done!"]);
    };
    window.addEventListener("gameCornerPrizeBuy", this.gameCornerBuyHandler);

    this.gameCornerCoinHandler = (event: Event) => {
      const data = (event as CustomEvent).detail as {
        coins: number;
        coinAmount?: number;
        message?: string;
      };
      if (data.message) {
        usePokemonDialogueStore.getState().openDialogue([data.message]);
      }
    };
    window.addEventListener("gameCornerCoinBalance", this.gameCornerCoinHandler);
    window.addEventListener("gameCornerCoinPickup", this.gameCornerCoinHandler);

    this.chatBubbleHandler = (event: Event) => {
      const renderer = this.deps.mapRenderer();
      if (!renderer) return;

      const { senderId, text } = (event as CustomEvent).detail as {
        senderId: number;
        senderName: string;
        text: string;
      };
      const actor = this.deps
        .actors()
        .find(
          (candidate) =>
            candidate.objectType === "player" &&
            candidate.internalId === senderId,
        );
      if (actor) {
        renderer.showChatBubble(actor.id, text);
      }
    };
    window.addEventListener("playerChatBubble", this.chatBubbleHandler);
  }

  cleanup(): void {
    if (this.elevatorFloorsHandler) {
      window.removeEventListener("elevatorFloors", this.elevatorFloorsHandler);
      this.elevatorFloorsHandler = null;
    }
 this.safariUnsubscribe?.();this.safariUnsubscribe=null;
    if (this.gameCornerPrizeHandler) {
      window.removeEventListener("gameCornerPrizeList", this.gameCornerPrizeHandler);
      this.gameCornerPrizeHandler = null;
    }
    if (this.gameCornerBuyHandler) {
      window.removeEventListener("gameCornerPrizeBuy", this.gameCornerBuyHandler);
      this.gameCornerBuyHandler = null;
    }
    if (this.gameCornerCoinHandler) {
      window.removeEventListener("gameCornerCoinBalance", this.gameCornerCoinHandler);
      window.removeEventListener("gameCornerCoinPickup", this.gameCornerCoinHandler);
      this.gameCornerCoinHandler = null;
    }
    if (this.chatBubbleHandler) {
      window.removeEventListener("playerChatBubble", this.chatBubbleHandler);
      this.chatBubbleHandler = null;
    }
  }
}
