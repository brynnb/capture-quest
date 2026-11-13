import {watchInteractionPosition} from "../../services/InventoryCommandService";
import { Scene } from "phaser";

import type { GameCornerPrizeListResponse } from "@/net/generated/world_api";

export class TileViewerOverlays {
  private prizeChoice: (() => void) | null = null;
  private gameCornerMenuItems: Phaser.GameObjects.GameObject[] = [];
  private safariHudContainer: Phaser.GameObjects.Container | null = null;
  private safariHudText: Phaser.GameObjects.Text | null = null;

  constructor(private readonly scene: Scene, private readonly configureUI:(elements:Phaser.GameObjects.GameObject[])=>void) {}

  // Keep the menu alive through pointer-up and consume the whole gesture.
  // Destroying it on pointer-down lets the release activate the world beneath it.
  private bindMenuChoice(item:Phaser.GameObjects.Text,choose:()=>void):void{
    item.on("pointerdown",(_pointer:Phaser.Input.Pointer,_x:number,_y:number,event:Phaser.Types.Input.EventData)=>event.stopPropagation());
    item.on("pointerup",(_pointer:Phaser.Input.Pointer,_x:number,_y:number,event:Phaser.Types.Input.EventData)=>{event.stopPropagation();choose();});
  }

  isSlotMachineTile(x: number, y: number): boolean {
    const slotPositions = new Set([
      "18,15", "18,14", "18,13", "18,12", "18,11", "18,10",
      "13,10", "13,11", "13,12", "13,13", "13,14", "13,15",
      "12,15", "12,14", "12,13", "12,12", "12,11", "12,10",
      "7,10", "7,11", "7,12", "7,13", "7,14", "7,15",
      "6,15", "6,14", "6,13", "6,12", "6,11", "6,10",
      "1,10", "1,11", "1,12", "1,13", "1,14", "1,15",
    ]);
    return slotPositions.has(`${x},${y}`);
  }

  showPrizeMenu(data: GameCornerPrizeListResponse, signal: AbortSignal): Promise<number | undefined> {
    this.closeGameCornerUI();
    return new Promise(resolve => {
      let settled = false;
      const finish = (prizeId?: number) => {
        if (settled) return;
        settled = true;
        signal.removeEventListener("abort", abort);
        this.prizeChoice = null;
        this.gameCornerMenuItems.forEach(item => item.destroy());
        this.gameCornerMenuItems = [];
        resolve(prizeId);
      };
      const abort = () => finish();
      this.prizeChoice = abort;
      signal.addEventListener("abort", abort, {once:true});
      const cam = this.scene.cameras.main;
      const prizes = data.prizes;
      const itemH = 20, w = 220, h = prizes.length * itemH + 60;
      const bg = this.scene.add.rectangle(cam.width / 2, cam.height / 2, w, h, 0x000000, 0.92)
        .setStrokeStyle(2, 0xffcc00).setDepth(9999).setScrollFactor(0).setOrigin(0.5);
      this.gameCornerMenuItems.push(bg);
      const title = this.scene.add.text(bg.x, bg.y - h / 2 + 14, `PRIZE EXCHANGE (${data.snapshot.coins} coins)`, {
        fontSize:"10px",color:"#ffcc00",fontFamily:"monospace",align:"center",
      }).setOrigin(0.5).setDepth(10000).setScrollFactor(0);
      this.gameCornerMenuItems.push(title);
      prizes.forEach((prize,i) => {
        const canAfford = data.snapshot.coins >= prize.coinCost;
        const button = this.scene.add.text(bg.x, bg.y - h / 2 + 36 + i * itemH, `${prize.name} - ${prize.coinCost}C`, {
          fontSize:"10px",color:canAfford?"#ffffff":"#666666",fontFamily:"monospace",align:"center",padding:{x:4,y:2},
        }).setOrigin(0.5).setDepth(10000).setScrollFactor(0);
        if (canAfford) {
          button.setInteractive({useHandCursor:true});
          button.on("pointerover",()=>button.setColor("#ffff00"));
          button.on("pointerout",()=>button.setColor("#ffffff"));
          this.bindMenuChoice(button,()=>finish(prize.id));
        }
        this.gameCornerMenuItems.push(button);
      });
      const cancel = this.scene.add.text(bg.x,bg.y+h/2-14,"CANCEL",{
        fontSize:"10px",color:"#ff6666",fontFamily:"monospace",align:"center",backgroundColor:"#333333",padding:{x:16,y:2},
      }).setOrigin(0.5).setDepth(10000).setScrollFactor(0).setInteractive({useHandCursor:true});
      this.bindMenuChoice(cancel,()=>finish());
      this.gameCornerMenuItems.push(cancel);
      this.configureUI(this.gameCornerMenuItems);
      if(signal.aborted)abort();
    });
  }

  closeGameCornerUI(): void {
    this.prizeChoice?.();
    this.prizeChoice = null;
    this.gameCornerMenuItems.forEach(item => item.destroy());
    this.gameCornerMenuItems = [];
  }

  updateSafariHUD(stepsLeft: number, ballsLeft: number): void {
    if (!this.safariHudContainer || !this.safariHudText) {
      const width = 148;
      const height = 58;
      const background = this.scene.add.graphics();
      background.fillStyle(0xf8f8f8, 0.96);
      background.fillRoundedRect(0, 0, width, height, 6);
      background.lineStyle(4, 0x383838, 1);
      background.strokeRoundedRect(0, 0, width, height, 6);
      background.lineStyle(1, 0xc8c8c8, 1);
      background.strokeRoundedRect(7, 7, width - 14, height - 14, 3);

      this.safariHudText = this.scene.add.text(14, 9, "", {
        fontSize: "9px",
        color: "#383838",
        fontFamily: "\"Pokemon GB\", \"Press Start 2P\", monospace",
        lineSpacing: 5,
      });

      this.safariHudContainer = this.scene.add
        .container(0, 0, [background, this.safariHudText])
        .setDepth(9998)
        .setScrollFactor(0)
        .setSize(width, height);
      this.configureUI([this.safariHudContainer]);
    }
    this.safariHudText.setText(`SAFARI\nSTEPS ${stepsLeft}\nBALLS  ${ballsLeft}`);
    this.positionSafariHUD();
  }

  positionSafariHUD(): void {
    if (!this.safariHudContainer) return;

    const cam = this.scene.cameras.main;
    const margin = 12;
    const x = Math.min(
      margin,
      Math.max(0, cam.width - this.safariHudContainer.width),
    );
    this.safariHudContainer.setPosition(x, margin);
  }

  destroySafariHUD(): void {
    if (this.safariHudContainer) {
      this.safariHudContainer.destroy(true);
      this.safariHudContainer = null;
      this.safariHudText = null;
      return;
    }
    if (this.safariHudText) {
      this.safariHudText.destroy();
      this.safariHudText = null;
    }
  }

  showElevatorMenu(
    floors: Array<{ floorMapId: number; floorLabel: string }>,
    signal:AbortSignal,
  ): Promise<number|undefined> {
    return new Promise(resolve=>{
    const cam = this.scene.cameras.main;
    const menuWidth = 120;
    const itemHeight = 24;
    const padding = 8;
    // Include both the header and cancel row in the frame.
    const menuHeight = (floors.length + 2) * itemHeight + padding * 2;
    const bg = this.scene.add
      .rectangle(cam.width / 2, cam.height / 2, menuWidth, menuHeight, 0x000000, 0.9)
      .setStrokeStyle(2, 0xffffff)
      .setDepth(9999)
      .setScrollFactor(0)
      .setOrigin(0.5);
    const menuItems: Phaser.GameObjects.GameObject[] = [bg];

    const headerY = bg.y - menuHeight / 2 + padding + itemHeight / 2;
    const header = this.scene.add
      .text(bg.x, headerY, "Which floor?", {
        fontSize: "12px",
        color: "#ffffff",
        fontFamily: "monospace",
        align: "center",
      })
      .setOrigin(0.5)
      .setDepth(10000)
      .setScrollFactor(0);
    menuItems.push(header);

    let settled=false;
    const finish=(floor?:number)=>{if(settled)return;settled=true;stopPosition();signal.removeEventListener("abort",abort);menuItems.forEach(item=>item.destroy());resolve(floor);};
    const abort=()=>finish();const stopPosition=watchInteractionPosition(abort,true);
    signal.addEventListener("abort",abort,{once:true});
    floors.forEach((floor, i) => {
      const yPos = headerY + (i + 1) * itemHeight;
      const label = this.scene.add
        .text(bg.x, yPos, floor.floorLabel, {
          fontSize: "12px",
          color: "#ffffff",
          fontFamily: "monospace",
          align: "center",
          backgroundColor: "#333333",
          padding: { x: 16, y: 4 },
        })
        .setOrigin(0.5)
        .setDepth(10000)
        .setScrollFactor(0)
        .setInteractive({ useHandCursor: true });

      label.on("pointerover", () => label.setColor("#ffff00"));
      label.on("pointerout", () => label.setColor("#ffffff"));
      this.bindMenuChoice(label, () => {
        finish(floor.floorMapId);
      });
      menuItems.push(label);
    });

    const cancelY = headerY + (floors.length + 1) * itemHeight;
    const cancel = this.scene.add
      .text(bg.x, cancelY, "CANCEL", {
        fontSize: "12px",
        color: "#aaaaaa",
        fontFamily: "monospace",
        align: "center",
        padding: { x: 16, y: 4 },
      })
      .setOrigin(0.5)
      .setDepth(10000)
      .setScrollFactor(0)
      .setInteractive({ useHandCursor: true });
    cancel.on("pointerover", () => cancel.setColor("#ffff00"));
    cancel.on("pointerout", () => cancel.setColor("#aaaaaa"));
    this.bindMenuChoice(cancel, () => {
      finish();
    });
    menuItems.push(cancel);
    this.configureUI(menuItems);
    if(signal.aborted)abort();
    });
  }

  cleanup(): void {
    this.destroySafariHUD();
    this.closeGameCornerUI();
  }
}
