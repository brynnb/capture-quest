import { Scene } from "phaser";
import { PhaserActor } from "@/net/generated/world_api";
import { TILE_SIZE } from "../constants";

// Direction frame indices for sprite animation
enum DirectionFrame {
  DOWN = 0,
  UP = 1,
  LEFT = 2,
  RIGHT = 2, // Same as LEFT but flipped
  WALKING_DOWN = 3,
  WALKING_UP = 4,
  WALKING_LEFT = 5,
  WALKING_RIGHT = 5, // Same as WALKING_LEFT but flipped
}

// Movement update from server
interface MovementUpdate {
  x: number;
  y: number;
  direction: string;
  timestamp: number;
  localPath?: boolean;
  ledgeJump?: boolean;
}

// State for a tracked actor
interface ActorState {
  sprite: Phaser.GameObjects.Sprite;
  actor: PhaserActor;
  queue: MovementUpdate[];
  isAnimating: boolean;
  currentTween: Phaser.Tweens.Tween | null;
  // Track position independently since actor object may be mutated externally
  currentX: number;
  currentY: number;
  currentDirection: string;
  // Optional name label that follows the sprite
  nameLabel?: Phaser.GameObjects.Text;
  // Optional chat bubble that follows the sprite
  chatBubble?: Phaser.GameObjects.Container;
  jumpShadow?: Phaser.GameObjects.Ellipse;
  // Pending idle timer to defer STAY frame (avoids flicker during continuous walking)
  pendingIdleTimer?: ReturnType<typeof setTimeout>;
  // Step counter for alternating foot flip on UP/DOWN walking
  stepCount: number;
}

export interface ActorTilePosition {
  x: number;
  y: number;
  direction: string;
}

export interface ActorMovementOptions {
  ledgeJump?: boolean;
}

interface PathCompletion {
  x: number;
  y: number;
  resolve: () => void;
}

/**
 * ActorMovementController handles smooth movement animations for actors
 * with a queue-based system to handle network timing variations.
 */
export class ActorMovementController {
  private scene: Scene;
  private actorStates: Map<number, ActorState> = new Map();
  private pathCompletions: Map<number, PathCompletion[]> = new Map();
  private idleWaiters: Map<number, Array<() => void>> = new Map();
  private onStepComplete:
    | ((actorId: number, x: number, y: number) => void)
    | null = null;

  // Maximum queue depth before we start speeding up
  private static readonly QUEUE_THRESHOLD = 2;
  // Maximum queue depth before we just warp to the latest position
  private static readonly WARP_QUEUE_THRESHOLD = 10;
  // Speed multiplier when catching up
  private static readonly CATCHUP_SPEED_MULTIPLIER = 0.6;
  // Default move speed if not provided (ms per tile)
  private static readonly DEFAULT_MOVE_SPEED = 200;
  // Maximum age of a movement update before we just warp (ms)
  private static readonly MAX_STEP_AGE = 1500;
  private static readonly LEDGE_JUMP_LIFT_PX = 10;
  private static readonly LEDGE_JUMP_SHADOW_OFFSET_Y = 5;

  constructor(scene: Scene) {
    this.scene = scene;
  }

  /**
   * Register an actor for movement tracking
   */
  registerActor(
    actor: PhaserActor,
    sprite: Phaser.GameObjects.Sprite,
    nameLabel?: Phaser.GameObjects.Text,
    chatBubble?: Phaser.GameObjects.Container,
  ): void {
    this.actorStates.set(actor.id, {
      sprite,
      actor,
      queue: [],
      isAnimating: false,
      currentTween: null,
      // Store initial position independently
      currentX: actor.x ?? 0,
      currentY: actor.y ?? 0,
      currentDirection: actor.actionDirection || "DOWN",
      nameLabel,
      chatBubble,
      stepCount: 0,
    });

    // Set initial frame based on direction
    this.updateSpriteFrame(
      sprite,
      actor.actionType || "STAY",
      actor.actionDirection || "DOWN",
    );
  }

  /**
   * Unregister an actor (when removed from scene)
   */
  unregisterActor(actorId: number): void {
    const state = this.actorStates.get(actorId);
    if (state) {
      if (state.currentTween) state.currentTween.stop();
      if (state.pendingIdleTimer) clearTimeout(state.pendingIdleTimer);
      this.destroyJumpShadow(state);
    }
    this.actorStates.delete(actorId);
    this.resolveIdleWaiters(actorId);
  }

  updateActorMetadata(actor: PhaserActor): void {
    const state = this.actorStates.get(actor.id);
    if (!state) return;

    state.actor.moveSpeed = actor.moveSpeed;
    state.actor.actionType = actor.actionType;
    state.actor.movementType = actor.movementType;
    state.actor.spriteName = actor.spriteName;
    state.actor.name = actor.name;
    state.actor.text = actor.text;
    state.actor.trainerClass = actor.trainerClass;
    state.actor.trainerPartyIndex = actor.trainerPartyIndex;
    state.actor.itemId = actor.itemId;
  }

  /**
   * Handle a position update from the server
   */
  handlePositionUpdate(
    actorId: number,
    newX: number,
    newY: number,
    direction: string,
    options: ActorMovementOptions = {},
  ): void {
    const state = this.actorStates.get(actorId);
    if (!state) {
      console.warn(
        `[MovementController] handlePositionUpdate: Actor ${actorId} not registered`,
      );
      return;
    }

    // If the movement is a large jump (e.g. warp), or if the queue is way behind
    const dist =
      Math.abs(newX - state.currentX) + Math.abs(newY - state.currentY);
    if (
      dist > 2 ||
      state.queue.length >= ActorMovementController.WARP_QUEUE_THRESHOLD
    ) {
      // console.log(`[MovementController] Actor ${actorId} warping to (${newX}, ${newY}). Dist: ${dist}, Queue: ${state.queue.length}`);
      this.warpActor(actorId, newX, newY, direction);
      return;
    }

    const update: MovementUpdate = {
      x: newX,
      y: newY,
      direction: (direction || "DOWN").toUpperCase(),
      timestamp: Date.now(),
      ledgeJump: options.ledgeJump === true,
    };

    // Add to queue
    state.queue.push(update);

    // If not currently animating, start processing the queue
    if (!state.isAnimating) {
      this.processQueue(actorId);
    }
  }

  /**
   * Handle a direction-only update (no position change)
   */
  handleDirectionUpdate(actorId: number, direction: string): void {
    const state = this.actorStates.get(actorId);
    if (!state) return;

    // console.log(`[MovementController] Direction update for actor ${actorId}: ${direction}`);

    const dir = (direction || "DOWN").toUpperCase();
    state.currentDirection = dir;
    state.actor.actionDirection = dir;

    // If not animating, immediately update the sprite
    if (!state.isAnimating) {
      this.updateSpriteFrame(state.sprite, "STAY", dir);
    }
  }

  getActorTilePosition(actorId: number): ActorTilePosition | null {
    const state = this.actorStates.get(actorId);
    if (!state) return null;
    return {
      x: state.currentX,
      y: state.currentY,
      direction: state.currentDirection,
    };
  }

  animateActorLocalPath(
    actorId: number,
    path: { x: number; y: number }[],
    finalDirection?: string,
  ): Promise<void> {
    const state = this.actorStates.get(actorId);
    if (!state) {
      return Promise.resolve();
    }

    const filteredPath: { x: number; y: number }[] = [];
    let fromX = state.currentX;
    let fromY = state.currentY;
    for (const step of path) {
      if (step.x === fromX && step.y === fromY) {
        continue;
      }
      filteredPath.push(step);
      fromX = step.x;
      fromY = step.y;
    }

    if (filteredPath.length === 0) {
      if (finalDirection) {
        this.handleDirectionUpdate(actorId, finalDirection);
      }
      return Promise.resolve();
    }

    return new Promise<void>((resolve) => {
      const last = filteredPath[filteredPath.length - 1];
      const completions = this.pathCompletions.get(actorId) ?? [];
      completions.push({
        x: last.x,
        y: last.y,
        resolve: () => {
          if (finalDirection) {
            this.handleDirectionUpdate(actorId, finalDirection);
          }
          resolve();
        },
      });
      this.pathCompletions.set(actorId, completions);

      fromX = state.currentX;
      fromY = state.currentY;
      for (let i = 0; i < filteredPath.length; i++) {
        const step = filteredPath[i];
        const direction = this.calculateDirection(fromX, fromY, step.x, step.y);
        state.queue.push({
          x: step.x,
          y: step.y,
          direction,
          timestamp: Date.now() + i * ActorMovementController.DEFAULT_MOVE_SPEED,
          localPath: true,
        });
        fromX = step.x;
        fromY = step.y;
      }
      if (!state.isAnimating) {
        this.processQueue(actorId);
      }
    });
  }

  /**
   * Process the movement queue for an actor
   */
  private processQueue(actorId: number): void {
    const state = this.actorStates.get(actorId);
    if (!state || state.queue.length === 0) {
      if (state) {
        state.isAnimating = false;
        // Defer idle frame slightly so continuous walking doesn't flash standing sprite
        if (state.pendingIdleTimer) clearTimeout(state.pendingIdleTimer);
        state.pendingIdleTimer = setTimeout(() => {
          // Only set idle if still not animating (no new movement arrived)
          if (!state.isAnimating && state.sprite.active) {
            this.updateSpriteFrame(state.sprite, "STAY", state.currentDirection);
          }
          state.pendingIdleTimer = undefined;
        }, 50);
        this.resolveIdleWaiters(actorId);
      }
      return;
    }

    // Cancel any pending idle frame since we have more movement
    if (state.pendingIdleTimer) {
      clearTimeout(state.pendingIdleTimer);
      state.pendingIdleTimer = undefined;
    }
    this.destroyJumpShadow(state);

    state.isAnimating = true;

    // If we have a massive queue (unprocessed while backgrounded), jump to the end
    const isLocalPath = state.queue.some((update) => update.localPath);
    if (
      !isLocalPath &&
      state.queue.length >= ActorMovementController.WARP_QUEUE_THRESHOLD
    ) {
      const last = state.queue[state.queue.length - 1];
      this.warpActor(actorId, last.x, last.y, last.direction);
      return;
    }

    // Get the next update from the queue
    const update = state.queue.shift()!;

    // If this update is too old (e.g. from when the tab was hidden),
    // snap immediately to the most recent position in the queue
    const now = Date.now();
    if (
      !update.localPath &&
      now - update.timestamp > ActorMovementController.MAX_STEP_AGE
    ) {
      const lastUpdate =
        state.queue.length > 0 ? state.queue[state.queue.length - 1] : update;
      this.warpActor(actorId, lastUpdate.x, lastUpdate.y, lastUpdate.direction);
      return;
    }

    // Use internally tracked position (not externally mutated actor.x/y)
    const currentX = state.currentX;
    const currentY = state.currentY;
    const targetX = update.x;
    const targetY = update.y;
    const tileDistance =
      Math.abs(targetX - currentX) + Math.abs(targetY - currentY);
    const isLedgeJump = update.ledgeJump === true && tileDistance === 2;

    // Calculate direction from movement (override server direction if moving)
    let moveDirection = update.direction;
    if (targetX !== currentX || targetY !== currentY) {
      if (targetY < currentY) moveDirection = "UP";
      else if (targetY > currentY) moveDirection = "DOWN";
      else if (targetX < currentX) moveDirection = "LEFT";
      else if (targetX > currentX) moveDirection = "RIGHT";
    }

    // Calculate target pixel positions
    const targetPosX = targetX * TILE_SIZE + TILE_SIZE / 2;
    const targetPosY = targetY * TILE_SIZE + TILE_SIZE / 2;

    // Determine animation duration
    let duration =
      (state.actor.moveSpeed || ActorMovementController.DEFAULT_MOVE_SPEED) *
      Math.max(1, tileDistance);

    // Speed up if queue is getting deep
    if (
      !update.localPath &&
      state.queue.length > ActorMovementController.QUEUE_THRESHOLD
    ) {
      duration *= ActorMovementController.CATCHUP_SPEED_MULTIPLIER;
    }

    // If no actual movement, just update direction and process next
    if (targetX === currentX && targetY === currentY) {
      state.currentDirection = moveDirection;
      this.updateSpriteFrame(state.sprite, "STAY", moveDirection);
      this.processQueue(actorId);
      return;
    }

    // Increment step counter for UP/DOWN foot alternation between tiles
    state.stepCount++;
    const alternateFlip = state.stepCount % 2 === 1;

    // Start with standing frame facing the move direction
    this.updateSpriteFrame(state.sprite, "STAY", moveDirection);
    const jumpShadow = isLedgeJump
      ? this.createJumpShadow(state, currentX, currentY)
      : null;

    // Create the movement tween
    state.currentTween = this.scene.tweens.add({
      targets: state.sprite,
      x: targetPosX,
      y: targetPosY,
      duration: duration,
      ease: "Linear",
      onUpdate: (tween) => {
        // Walk cycle: 2 phases per tile
        //   First half (0-50%):  Standing frame
        //   Second half (50-100%): Walking frame (leg out)
        // For UP/DOWN, alternateFlip toggles between tiles for foot swap
        const progress = tween.progress;
        const groundX = state.sprite.x;
        const groundY = state.sprite.y;
        if (progress >= 0.5) {
          this.updateSpriteFrame(state.sprite, "WALK", moveDirection, alternateFlip);
        } else {
          this.updateSpriteFrame(state.sprite, "STAY", moveDirection);
        }
        if (jumpShadow) {
          jumpShadow.setPosition(
            groundX,
            groundY + ActorMovementController.LEDGE_JUMP_SHADOW_OFFSET_Y,
          );
          state.sprite.setY(
            groundY - ActorMovementController.ledgeJumpYOffset(progress),
          );
        }

        // Sync attached objects to sprite position
        this.syncAttachedObjects(state);
      },
      onComplete: () => {
        // Update our tracked position
        state.currentX = targetX;
        state.currentY = targetY;
        state.currentDirection = moveDirection;
        state.actor.x = targetX;
        state.actor.y = targetY;
        state.actor.actionDirection = moveDirection;
        state.currentTween = null;
        state.sprite.setY(targetPosY);
        this.destroyJumpShadow(state);

        // Sync attached objects to final position
        this.syncAttachedObjects(state);

        // Process next item in queue
        this.processQueue(actorId);

        // Notify completion
        if (this.onStepComplete) {
          this.onStepComplete(actorId, targetX, targetY);
        }
        this.resolvePathCompletions(actorId);
      },
    });
  }

  private createJumpShadow(
    state: ActorState,
    x: number,
    y: number,
  ): Phaser.GameObjects.Ellipse | null {
    const shadow = this.scene.add.ellipse(
      x * TILE_SIZE + TILE_SIZE / 2,
      y * TILE_SIZE +
        TILE_SIZE / 2 +
        ActorMovementController.LEDGE_JUMP_SHADOW_OFFSET_Y,
      TILE_SIZE * 0.75,
      TILE_SIZE * 0.25,
      0x000000,
      0.65,
    );
    const spriteDepth = Number.isFinite(state.sprite.depth)
      ? state.sprite.depth
      : 0;
    shadow.setDepth(spriteDepth - 0.1);
    const parentContainer = state.sprite.parentContainer;
    if (parentContainer) {
      parentContainer.add(shadow);
      const orderedParent = parentContainer as unknown as {
        moveBelow?: (
          child: Phaser.GameObjects.GameObject,
          below: Phaser.GameObjects.GameObject,
        ) => void;
        bringToTop?: (child: Phaser.GameObjects.GameObject) => void;
      };
      if (orderedParent.moveBelow) {
        orderedParent.moveBelow(shadow, state.sprite);
      } else {
        orderedParent.bringToTop?.(state.sprite);
      }
    }
    state.jumpShadow = shadow;
    return shadow;
  }

  private destroyJumpShadow(state: ActorState): void {
    if (!state.jumpShadow) return;
    state.jumpShadow.destroy();
    state.jumpShadow = undefined;
  }

  private static ledgeJumpYOffset(progress: number): number {
    return (
      Math.sin(Math.PI * progress) *
      ActorMovementController.LEDGE_JUMP_LIFT_PX
    );
  }

  private calculateDirection(
    fromX: number,
    fromY: number,
    toX: number,
    toY: number,
  ): string {
    if (toY < fromY) return "UP";
    if (toY > fromY) return "DOWN";
    if (toX < fromX) return "LEFT";
    if (toX > fromX) return "RIGHT";
    return "DOWN";
  }

  /**
   * Set callback for when an actor completes a step
   */
  setOnStepComplete(
    callback: (actorId: number, x: number, y: number) => void,
  ): void {
    this.onStepComplete = callback;
  }

  /**
   * Update sprite frame based on action type and direction
   */
  private updateSpriteFrame(
    sprite: Phaser.GameObjects.Sprite,
    actionType: string,
    direction: string,
    alternateFlip: boolean = false,
  ): void {
    // Safety check: ensure sprite is still valid and active
    if (!sprite || !sprite.active || !sprite.scene) {
      return;
    }

    const isWalking = actionType.toUpperCase() === "WALK";
    const dir = (direction || "DOWN").toUpperCase();

    let frame: number;
    let flipX = false;

    switch (dir) {
      case "UP":
        frame = isWalking ? DirectionFrame.WALKING_UP : DirectionFrame.UP;
        // Alternate flip on odd steps to simulate foot alternation
        if (isWalking && alternateFlip) flipX = true;
        break;
      case "DOWN":
        frame = isWalking ? DirectionFrame.WALKING_DOWN : DirectionFrame.DOWN;
        // Alternate flip on odd steps to simulate foot alternation
        if (isWalking && alternateFlip) flipX = true;
        break;
      case "LEFT":
        frame = isWalking ? DirectionFrame.WALKING_LEFT : DirectionFrame.LEFT;
        flipX = false;
        break;
      case "RIGHT":
        frame = isWalking ? DirectionFrame.WALKING_RIGHT : DirectionFrame.RIGHT;
        flipX = true;
        break;
      default:
        frame = DirectionFrame.DOWN;
    }

    // Safety check: ensure requested frame exists in texture
    const texture = sprite.texture;
    const frameStr = frame.toString();

    // If the texture is missing or destroyed, don't try to set frame
    if (!texture || texture.key === "__MISSING") {
      return;
    }

    if (texture.has(frameStr) || texture.has(frame as any)) {
      sprite.setFrame(frame);
    } else if (
      texture.has("0") ||
      texture.has(0 as any) ||
      texture.frameTotal > 0
    ) {
      // Fallback to frame 0 if requested frame doesn't exist
      sprite.setFrame(0);
    }

    sprite.setFlipX(flipX);
  }

  /**
   * Instantly warp an actor to a position and clear their queue
   */
  private warpActor(
    actorId: number,
    x: number,
    y: number,
    direction: string,
  ): void {
    const state = this.actorStates.get(actorId);
    if (!state) return;

    // Stop current animation
    if (state.currentTween) {
      state.currentTween.stop();
      state.currentTween = null;
    }
    this.destroyJumpShadow(state);
    if (state.pendingIdleTimer) {
      clearTimeout(state.pendingIdleTimer);
      state.pendingIdleTimer = undefined;
    }

    // Clear queue
    state.queue = [];

    // Internal snap
    state.currentX = x;
    state.currentY = y;
    state.currentDirection = (direction || "DOWN").toUpperCase();
    state.actor.x = x;
    state.actor.y = y;
    state.actor.actionDirection = state.currentDirection;
    state.isAnimating = false;

    // Visual snap
    const snapX = x * TILE_SIZE + TILE_SIZE / 2;
    const snapY = y * TILE_SIZE + TILE_SIZE / 2;
    state.sprite.setX(snapX);
    state.sprite.setY(snapY);
    this.syncAttachedObjects(state);
    this.updateSpriteFrame(state.sprite, "STAY", state.currentDirection);

    // Notify completion
    if (this.onStepComplete) {
      this.onStepComplete(actorId, x, y);
    }
    this.resolvePathCompletions(actorId);
    this.resolveIdleWaiters(actorId);
  }

  private resolvePathCompletions(actorId: number): void {
    const state = this.actorStates.get(actorId);
    const completions = this.pathCompletions.get(actorId);
    if (!state || !completions || completions.length === 0) {
      return;
    }
    const remaining: PathCompletion[] = [];
    for (const completion of completions) {
      const atTarget =
        completion.x === state.currentX && completion.y === state.currentY;
      const idleAtTarget = atTarget && state.queue.length === 0 && !state.isAnimating;
      if (idleAtTarget) {
        completion.resolve();
      } else {
        remaining.push(completion);
      }
    }
    if (remaining.length > 0) {
      this.pathCompletions.set(actorId, remaining);
    } else {
      this.pathCompletions.delete(actorId);
    }
  }

  /**
   * Sync name label and chat bubble positions to the sprite's current position
   */
  private syncAttachedObjects(state: ActorState): void {
    const spriteX = state.sprite.x;
    const spriteY = state.sprite.y;
    if (state.nameLabel && state.nameLabel.active) {
      state.nameLabel.setPosition(spriteX, spriteY + TILE_SIZE / 2 + 2);
    }
    if (state.chatBubble && state.chatBubble.active) {
      state.chatBubble.setPosition(spriteX, spriteY - TILE_SIZE / 2 - 2);
    }
  }

  /**
   * Update the name label reference for an actor
   */
  setNameLabel(actorId: number, label: Phaser.GameObjects.Text): void {
    const state = this.actorStates.get(actorId);
    if (state) {
      state.nameLabel = label;
      this.syncAttachedObjects(state);
    }
  }

  /**
   * Update the chat bubble reference for an actor
   */
  setChatBubble(actorId: number, bubble: Phaser.GameObjects.Container | undefined): void {
    const state = this.actorStates.get(actorId);
    if (state) {
      state.chatBubble = bubble;
      if (bubble) {
        this.syncAttachedObjects(state);
      }
    }
  }

  /**
   * Get the current state of an actor
   */
  getActorState(actorId: number): ActorState | undefined {
    return this.actorStates.get(actorId);
  }

  waitForActorIdle(actorId: number): Promise<void> {
    const state = this.actorStates.get(actorId);
    if (!state || (!state.isAnimating && state.queue.length === 0)) {
      return Promise.resolve();
    }

    return new Promise<void>((resolve) => {
      const waiters = this.idleWaiters.get(actorId) ?? [];
      waiters.push(resolve);
      this.idleWaiters.set(actorId, waiters);
    });
  }

  snapActorToPosition(
    actorId: number,
    x: number,
    y: number,
    direction: string,
  ): void {
    this.warpActor(actorId, x, y, direction);
  }

  private resolveIdleWaiters(actorId: number): void {
    const waiters = this.idleWaiters.get(actorId);
    if (!waiters || waiters.length === 0) return;

    const state = this.actorStates.get(actorId);
    if (state && (state.isAnimating || state.queue.length > 0)) {
      return;
    }

    this.idleWaiters.delete(actorId);
    for (const resolve of waiters) {
      resolve();
    }
  }

  /**
   * Clear all tracked actors except players
   */
  clear(): void {
    for (const [id, state] of this.actorStates.entries()) {
      // Keep player states to avoid jumping during transitions
      if (state.actor && state.actor.objectType === "player") {
        continue;
      }

      if (state.currentTween) {
        state.currentTween.stop();
      }
      if (state.pendingIdleTimer) {
        clearTimeout(state.pendingIdleTimer);
      }
      this.actorStates.delete(id);
    }
  }
}
