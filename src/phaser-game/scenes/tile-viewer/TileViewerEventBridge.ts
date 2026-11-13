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



  private gameCornerCoinHandler: ((e: Event) => void) | null = null;
  private chatBubbleHandler: ((e: Event) => void) | null = null;

  constructor(private readonly deps: TileViewerEventBridgeDeps) {}

  register(): void {
 this.safariUnsubscribe=bindSafariVisitView(view=>{if(view?.active)this.deps.overlays.updateSafariHUD(view.stepsLeft,view.ballsLeft);else this.deps.overlays.destroySafariHUD();});

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
 this.safariUnsubscribe?.();this.safariUnsubscribe=null;
    if (this.gameCornerCoinHandler) {
      window.removeEventListener("gameCornerCoinPickup", this.gameCornerCoinHandler);
      this.gameCornerCoinHandler = null;
    }
    if (this.chatBubbleHandler) {
      window.removeEventListener("playerChatBubble", this.chatBubbleHandler);
      this.chatBubbleHandler = null;
    }
  }
}
