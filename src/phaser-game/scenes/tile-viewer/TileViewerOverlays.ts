import { Scene } from "phaser";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";

type PrizeListPayload = {
  prizes?: Array<{ id: number; type: string; name: string; coinCost: number }>;
  coins: number;
  error?: string;
  message?: string;
};

export class TileViewerOverlays {
  private gameCornerMenuItems: Phaser.GameObjects.GameObject[] = [];
  private safariHudContainer: Phaser.GameObjects.Container | null = null;
  private safariHudText: Phaser.GameObjects.Text | null = null;

  constructor(private readonly scene: Scene) {}

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

  showSlotMachineUI(coins: number): void {
    this.closeGameCornerUI();
    const cam = this.scene.cameras.main;
    const w = 200;
    const h = 160;

    const bg = this.scene.add
      .rectangle(cam.width / 2, cam.height / 2, w, h, 0x000000, 0.92)
      .setStrokeStyle(2, 0xffcc00)
      .setDepth(9999)
      .setScrollFactor(0)
      .setOrigin(0.5);
    this.gameCornerMenuItems.push(bg);

    const title = this.scene.add
      .text(cam.width / 2, bg.y - h / 2 + 14, "SLOT MACHINE", {
        fontSize: "12px",
        color: "#ffcc00",
        fontFamily: "monospace",
        align: "center",
      })
      .setOrigin(0.5)
      .setDepth(10000)
      .setScrollFactor(0);
    this.gameCornerMenuItems.push(title);

    const coinText = this.scene.add
      .text(cam.width / 2, bg.y - h / 2 + 32, `Coins: ${coins}`, {
        fontSize: "10px",
        color: "#aaffaa",
        fontFamily: "monospace",
        align: "center",
      })
      .setOrigin(0.5)
      .setDepth(10000)
      .setScrollFactor(0);
    this.gameCornerMenuItems.push(coinText);

    const reelText = this.scene.add
      .text(cam.width / 2, bg.y - h / 2 + 54, "[ ? ] [ ? ] [ ? ]", {
        fontSize: "14px",
        color: "#ffffff",
        fontFamily: "monospace",
        align: "center",
      })
      .setOrigin(0.5)
      .setDepth(10000)
      .setScrollFactor(0);
    this.gameCornerMenuItems.push(reelText);

    (this.scene as unknown as Record<string, unknown>)._slotReelText =
      reelText;
    (this.scene as unknown as Record<string, unknown>)._slotCoinText =
      coinText;

    [1, 2, 3].forEach((bet) => {
      const yPos = bg.y - h / 2 + 80 + (bet - 1) * 22;
      const btn = this.scene.add
        .text(cam.width / 2, yPos, `BET ${bet}`, {
          fontSize: "11px",
          color: "#ffffff",
          fontFamily: "monospace",
          align: "center",
          backgroundColor: "#333333",
          padding: { x: 24, y: 3 },
        })
        .setOrigin(0.5)
        .setDepth(10000)
        .setScrollFactor(0)
        .setInteractive({ useHandCursor: true });
      btn.on("pointerover", () => btn.setColor("#ffff00"));
      btn.on("pointerout", () => btn.setColor("#ffffff"));
      btn.on("pointerdown", () => {
        import("@/phaser-game/services/PhaserNetworkService").then(
          (PhaserNet) => {
            PhaserNet.playSlotMachine(bet, false);
          },
        );
      });
      this.gameCornerMenuItems.push(btn);
    });

    const closeBtn = this.scene.add
      .text(cam.width / 2, bg.y + h / 2 - 14, "QUIT", {
        fontSize: "10px",
        color: "#ff6666",
        fontFamily: "monospace",
        align: "center",
        backgroundColor: "#333333",
        padding: { x: 16, y: 2 },
      })
      .setOrigin(0.5)
      .setDepth(10000)
      .setScrollFactor(0)
      .setInteractive({ useHandCursor: true });
    closeBtn.on("pointerdown", () => this.closeGameCornerUI());
    this.gameCornerMenuItems.push(closeBtn);
  }

  handleSlotResult(data: {
    success: boolean;
    reels?: string[][];
    payout?: number;
    coins?: number;
    error?: string;
  }): void {
    const reelText = (this.scene as unknown as Record<string, unknown>)
      ._slotReelText as Phaser.GameObjects.Text | null;
    const coinText = (this.scene as unknown as Record<string, unknown>)
      ._slotCoinText as Phaser.GameObjects.Text | null;
    if (coinText && data.coins !== undefined) {
      coinText.setText(`Coins: ${data.coins}`);
    }
    if (reelText && data.reels) {
      reelText.setText("[ ? ] [ ? ] [ ? ]");
    }
  }

  showPrizeMenu(data: PrizeListPayload): void {
    this.closeGameCornerUI();
    if (data.error || data.message) {
      usePokemonDialogueStore
        .getState()
        .openDialogue([data.error || data.message || "Done!"]);
      return;
    }

    const cam = this.scene.cameras.main;
    const prizes = data.prizes || [];
    const itemH = 20;
    const w = 220;
    const h = prizes.length * itemH + 60;

    const bg = this.scene.add
      .rectangle(cam.width / 2, cam.height / 2, w, h, 0x000000, 0.92)
      .setStrokeStyle(2, 0xffcc00)
      .setDepth(9999)
      .setScrollFactor(0)
      .setOrigin(0.5);
    this.gameCornerMenuItems.push(bg);

    const title = this.scene.add
      .text(bg.x, bg.y - h / 2 + 14, `PRIZE EXCHANGE (${data.coins} coins)`, {
        fontSize: "10px",
        color: "#ffcc00",
        fontFamily: "monospace",
        align: "center",
      })
      .setOrigin(0.5)
      .setDepth(10000)
      .setScrollFactor(0);
    this.gameCornerMenuItems.push(title);

    prizes.forEach((prize, i) => {
      const yPos = bg.y - h / 2 + 36 + i * itemH;
      const canAfford = data.coins >= prize.coinCost;
      const label = `${prize.name} - ${prize.coinCost}C`;
      const btn = this.scene.add
        .text(bg.x, yPos, label, {
          fontSize: "10px",
          color: canAfford ? "#ffffff" : "#666666",
          fontFamily: "monospace",
          align: "center",
          padding: { x: 4, y: 2 },
        })
        .setOrigin(0.5)
        .setDepth(10000)
        .setScrollFactor(0);
      if (canAfford) {
        btn.setInteractive({ useHandCursor: true });
        btn.on("pointerover", () => btn.setColor("#ffff00"));
        btn.on("pointerout", () => btn.setColor("#ffffff"));
        btn.on("pointerdown", () => {
          import("@/phaser-game/services/PhaserNetworkService").then(
            (PhaserNet) => {
              PhaserNet.buyPrize(prize.id);
            },
          );
        });
      }
      this.gameCornerMenuItems.push(btn);
    });

    const closeBtn = this.scene.add
      .text(bg.x, bg.y + h / 2 - 14, "CANCEL", {
        fontSize: "10px",
        color: "#ff6666",
        fontFamily: "monospace",
        align: "center",
        backgroundColor: "#333333",
        padding: { x: 16, y: 2 },
      })
      .setOrigin(0.5)
      .setDepth(10000)
      .setScrollFactor(0)
      .setInteractive({ useHandCursor: true });
    closeBtn.on("pointerdown", () => this.closeGameCornerUI());
    this.gameCornerMenuItems.push(closeBtn);
  }

  closeGameCornerUI(): void {
    this.gameCornerMenuItems.forEach((item) => item.destroy());
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
  ): void {
    const cam = this.scene.cameras.main;
    const menuWidth = 120;
    const itemHeight = 24;
    const padding = 8;
    const menuHeight = floors.length * itemHeight + padding * 2 + itemHeight;
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
      label.on("pointerdown", () => {
        import("@/phaser-game/services/PhaserNetworkService").then(
          (PhaserNet) => {
            PhaserNet.selectElevatorFloor(floor.floorMapId);
          },
        );
        menuItems.forEach((item) => item.destroy());
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
    cancel.on("pointerdown", () => {
      menuItems.forEach((item) => item.destroy());
    });
    menuItems.push(cancel);
  }

  cleanup(): void {
    this.destroySafariHUD();
    this.closeGameCornerUI();
  }
}
