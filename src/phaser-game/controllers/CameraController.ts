import { Scene } from "phaser";
import { MIN_ZOOM, MAX_ZOOM, DEFAULT_ZOOM, ZOOM_STEP } from "../constants";

// Define a constant for non-overworld zoom level
const NON_OVERWORLD_ZOOM = 2.0; // Much more zoomed in than DEFAULT_ZOOM
const PAN_GESTURE_THRESHOLD = 8;

interface ActivePointerState {
  startX: number;
  startY: number;
  lastX: number;
  lastY: number;
  isPanning: boolean;
}

interface PinchState {
  midpointX: number;
  midpointY: number;
  distance: number;
}

export class CameraController {
  private scene: Scene;
  private mainCamera: Phaser.Cameras.Scene2D.Camera;
  private uiCamera!: Phaser.Cameras.Scene2D.Camera; // UI camera for HUD elements
  private zoomLevel: number = DEFAULT_ZOOM;
  private isOverworld: boolean = true; // Track whether we're in overworld mode
  private isFollowing = false;
  private readonly activePointers = new Map<number, ActivePointerState>();
  private readonly gesturePointerIds = new Set<number>();
  private pinchState: PinchState | null = null;
  private readonly pointerDownHandler = (pointer: Phaser.Input.Pointer) => {
    this.handlePointerDown(pointer);
  };
  private readonly pointerMoveHandler = (pointer: Phaser.Input.Pointer) => {
    this.handlePointerMove(pointer);
  };
  private readonly pointerUpHandler = (pointer: Phaser.Input.Pointer) => {
    this.handlePointerUp(pointer);
  };
  private readonly wheelHandler = (
    _pointer: Phaser.Input.Pointer,
    _gameObjects: Phaser.GameObjects.GameObject[],
    _deltaX: number,
    deltaY: number,
  ) => {
    if (deltaY > 0) {
      this.setZoom(Math.max(MIN_ZOOM, this.zoomLevel - ZOOM_STEP));
    } else {
      this.setZoom(Math.min(MAX_ZOOM, this.zoomLevel + ZOOM_STEP));
    }
  };

  // Store overworld camera state
  private overworldCameraState = {
    x: 0,
    y: 0,
    zoom: DEFAULT_ZOOM,
    saved: false,
  };

  constructor(scene: Scene) {
    this.scene = scene;
    this.mainCamera = scene.cameras.main;
    this.mainCamera.setBackgroundColor(0x000000);
    this.setupUiCamera();
    this.setupControls();

    // Apply initial zoom
    this.mainCamera.setZoom(this.zoomLevel);
  }

  setupUiCamera() {
    // Create a separate camera for UI elements that won't be affected by zoom
    this.uiCamera = this.scene.cameras.add(
      0,
      0,
      this.mainCamera.width,
      this.mainCamera.height
    );
    this.uiCamera.setScroll(0, 0);
    this.uiCamera.transparent = true;
    this.uiCamera.setName("UICamera");
  }

  setupControls() {
    this.scene.input.on("pointerdown", this.pointerDownHandler);
    this.scene.input.on("pointermove", this.pointerMoveHandler);
    this.scene.input.on("pointerup", this.pointerUpHandler);
    this.scene.input.on("pointerupoutside", this.pointerUpHandler);
    this.scene.input.on("wheel", this.wheelHandler);
  }

  private handlePointerDown(pointer: Phaser.Input.Pointer): void {
    // Pointer ids are reused, so discard a gesture marker left by an outside
    // release before tracking the new contact.
    this.gesturePointerIds.delete(pointer.id);
    this.activePointers.set(pointer.id, {
      startX: pointer.x,
      startY: pointer.y,
      lastX: pointer.x,
      lastY: pointer.y,
      isPanning: false,
    });

    if (this.activePointers.size >= 2) {
      this.beginPinch();
    }
  }

  private handlePointerMove(pointer: Phaser.Input.Pointer): void {
    let trackedPointer = this.activePointers.get(pointer.id);
    if (!trackedPointer && pointer.isDown) {
      // A Game Object can stop the Scene-level pointerdown event. Recover on
      // the first move so a pan that begins over an NPC or warp still works.
      this.gesturePointerIds.delete(pointer.id);
      trackedPointer = {
        startX: pointer.downX,
        startY: pointer.downY,
        lastX: pointer.x,
        lastY: pointer.y,
        isPanning: false,
      };
      this.activePointers.set(pointer.id, trackedPointer);
      if (this.activePointers.size >= 2) {
        this.beginPinch();
      }
    }
    if (!trackedPointer) return;

    const previousX = trackedPointer.lastX;
    const previousY = trackedPointer.lastY;
    trackedPointer.lastX = pointer.x;
    trackedPointer.lastY = pointer.y;

    if (this.activePointers.size >= 2) {
      this.updatePinch();
      return;
    }

    const distanceFromStart = Math.hypot(
      pointer.x - trackedPointer.startX,
      pointer.y - trackedPointer.startY,
    );
    if (!trackedPointer.isPanning) {
      if (distanceFromStart < PAN_GESTURE_THRESHOLD) return;

      trackedPointer.isPanning = true;
      this.gesturePointerIds.add(pointer.id);
      if (!this.isFollowing) {
        this.panByScreenDelta(
          pointer.x - trackedPointer.startX,
          pointer.y - trackedPointer.startY,
        );
      }
      return;
    }

    if (!this.isFollowing) {
      this.panByScreenDelta(
        pointer.x - previousX,
        pointer.y - previousY,
      );
    }
  }

  private handlePointerUp(pointer: Phaser.Input.Pointer): void {
    this.activePointers.delete(pointer.id);

    if (this.activePointers.size >= 2) {
      this.beginPinch();
      return;
    }

    this.pinchState = null;

    // Continue smoothly as a one-finger pan after one finger of a pinch lifts.
    // Resetting the origin prevents the earlier pinch distance from causing a
    // jump on the next move.
    const remainingPointer = this.activePointers.values().next().value as
      | ActivePointerState
      | undefined;
    if (remainingPointer) {
      remainingPointer.startX = remainingPointer.lastX;
      remainingPointer.startY = remainingPointer.lastY;
      remainingPointer.isPanning = true;
    }
  }

  private beginPinch(): void {
    for (const pointerId of this.activePointers.keys()) {
      this.gesturePointerIds.add(pointerId);
    }
    this.pinchState = this.currentPinchState();
  }

  private updatePinch(): void {
    const nextPinchState = this.currentPinchState();
    if (!nextPinchState) {
      this.pinchState = null;
      return;
    }

    for (const pointerId of this.activePointers.keys()) {
      this.gesturePointerIds.add(pointerId);
    }

    const previousPinchState = this.pinchState;
    this.pinchState = nextPinchState;
    if (!previousPinchState || this.isFollowing) return;

    const distanceRatio = previousPinchState.distance > 0
      ? nextPinchState.distance / previousPinchState.distance
      : 1;
    const nextZoom = Math.min(
      MAX_ZOOM,
      Math.max(MIN_ZOOM, this.zoomLevel * distanceRatio),
    );

    // Keep the world point beneath the previous midpoint beneath the new
    // midpoint. This combines two-finger panning and focal-point zooming in a
    // single transform, avoiding the map sliding away during a pinch.
    const anchor = this.worldPointAt(
      previousPinchState.midpointX,
      previousPinchState.midpointY,
      this.zoomLevel,
    );
    this.zoomLevel = nextZoom;
    this.mainCamera.setZoom(nextZoom);
    this.scrollWorldPointToScreen(
      anchor.x,
      anchor.y,
      nextPinchState.midpointX,
      nextPinchState.midpointY,
      nextZoom,
    );
  }

  private currentPinchState(): PinchState | null {
    const pointers = Array.from(this.activePointers.values());
    if (pointers.length < 2) return null;

    const first = pointers[0];
    const second = pointers[1];
    return {
      midpointX: (first.lastX + second.lastX) / 2,
      midpointY: (first.lastY + second.lastY) / 2,
      distance: Math.hypot(
        second.lastX - first.lastX,
        second.lastY - first.lastY,
      ),
    };
  }

  private panByScreenDelta(deltaX: number, deltaY: number): void {
    this.mainCamera.scrollX -= deltaX / this.zoomLevel;
    this.mainCamera.scrollY -= deltaY / this.zoomLevel;
  }

  private worldPointAt(
    screenX: number,
    screenY: number,
    zoom: number,
  ): { x: number; y: number } {
    const originX = this.mainCamera.width * this.mainCamera.originX;
    const originY = this.mainCamera.height * this.mainCamera.originY;
    const localX = screenX - this.mainCamera.x;
    const localY = screenY - this.mainCamera.y;
    return {
      x:
        this.mainCamera.scrollX +
        originX +
        (localX - originX) / zoom,
      y:
        this.mainCamera.scrollY +
        originY +
        (localY - originY) / zoom,
    };
  }

  private scrollWorldPointToScreen(
    worldX: number,
    worldY: number,
    screenX: number,
    screenY: number,
    zoom: number,
  ): void {
    const originX = this.mainCamera.width * this.mainCamera.originX;
    const originY = this.mainCamera.height * this.mainCamera.originY;
    const localX = screenX - this.mainCamera.x;
    const localY = screenY - this.mainCamera.y;
    this.mainCamera.scrollX =
      worldX - originX - (localX - originX) / zoom;
    this.mainCamera.scrollY =
      worldY - originY - (localY - originY) / zoom;
  }

  handleResize() {
    // Resize the UI camera to match the main camera
    this.uiCamera.setSize(this.mainCamera.width, this.mainCamera.height);
  }

  update(cursors: Phaser.Types.Input.Keyboard.CursorKeys) {
    if (!cursors) return;

    // Skip keyboard movement if we're following a player
    if (this.isFollowing) {
      return;
    }

    const cameraMoveSpeed = 10 / this.zoomLevel; // Adjust speed based on zoom

    if (cursors.left.isDown) {
      this.mainCamera.scrollX -= cameraMoveSpeed;
    } else if (cursors.right.isDown) {
      this.mainCamera.scrollX += cameraMoveSpeed;
    }

    if (cursors.up.isDown) {
      this.mainCamera.scrollY -= cameraMoveSpeed;
    } else if (cursors.down.isDown) {
      this.mainCamera.scrollY += cameraMoveSpeed;
    }
  }

  centerOnMap(centerX: number, centerY: number) {
    this.mainCamera.centerOn(centerX, centerY);
  }

  follow(target: Phaser.GameObjects.GameObject) {
    this.isFollowing = true;
    this.mainCamera.startFollow(target, true, 0.5, 0.5);
  }

  stopFollow() {
    this.isFollowing = false;
    this.mainCamera.stopFollow();
  }

  setZoom(zoom: number) {
    this.zoomLevel = zoom;
    this.mainCamera.setZoom(zoom);
  }

  getZoom() {
    return this.zoomLevel;
  }

  getWorldPoint(x: number, y: number) {
    return this.mainCamera.getWorldPoint(x, y);
  }

  configureIgnoreElements(
    uiElements: Phaser.GameObjects.GameObject[],
    mapContainer: Phaser.GameObjects.Container,
    worldOverlayElements: Phaser.GameObjects.GameObject[] = [],
  ) {
    // Main camera should show the map container but ignore UI elements
    this.mainCamera.ignore(uiElements);

    // UI camera should only show UI elements and ignore the map/world overlays.
    this.uiCamera.ignore([mapContainer, ...worldOverlayElements]);
  }

  cleanup() {
    this.scene.input.off("pointerdown", this.pointerDownHandler);
    this.scene.input.off("pointermove", this.pointerMoveHandler);
    this.scene.input.off("pointerup", this.pointerUpHandler);
    this.scene.input.off("pointerupoutside", this.pointerUpHandler);
    this.scene.input.off("wheel", this.wheelHandler);
    this.resetPointerState();

    // Remove the UI camera
    this.scene.cameras.remove(this.uiCamera);
  }

  isDragging() {
    return this.activePointers.size > 0;
  }

  /**
   * Consume whether a pointer participated in camera navigation.
   *
   * Tile and actor click handlers should call this on pointer-up before
   * activating their target. In particular, both fingers of a pinch are
   * marked even if one finger barely moved.
   */
  consumePointerGesture(pointerId: number): boolean {
    return this.gesturePointerIds.delete(pointerId);
  }

  /**
   * Save the current camera position and zoom for the overworld view
   */
  saveOverworldCameraState() {
    if (this.isOverworld) {
      const cameraState = {
        x: this.mainCamera.scrollX,
        y: this.mainCamera.scrollY,
        zoom: this.zoomLevel,
        saved: true,
        timestamp: Date.now(),
      };

      // Save in memory
      this.overworldCameraState = cameraState;

      // Also save to Phaser registry for persistence between scene transitions
      try {
        if (this.scene && this.scene.game && this.scene.game.registry) {
          this.scene.game.registry.set("overworldCameraState", cameraState);
          console.log("Saved camera state:", {
            x: cameraState.x.toFixed(2),
            y: cameraState.y.toFixed(2),
            zoom: cameraState.zoom,
          });
        }
      } catch (error) {
        console.error("Error saving camera state:", error);
      }
    }
  }

  /**
   * Restore the saved overworld camera position and zoom
   * @returns True if a saved state was restored, false otherwise
   */
  restoreOverworldCameraState(): boolean {
    try {
      // First try to get from global Phaser registry (persists between scene transitions)
      const registryState = this.scene.game.registry.get(
        "overworldCameraState"
      );

      if (registryState && registryState.saved) {
        // Set zoom first to ensure proper positioning
        this.setZoom(registryState.zoom);

        // Then set scroll position
        this.mainCamera.setScroll(registryState.x, registryState.y);

        // Update memory state
        this.overworldCameraState = registryState;

        // Make sure we're in overworld mode
        this.isOverworld = true;

        console.log("Restored camera state:", {
          x: registryState.x.toFixed(2),
          y: registryState.y.toFixed(2),
          zoom: registryState.zoom,
        });
        return true;
      }

      // Fall back to memory if registry failed
      if (this.overworldCameraState.saved) {
        // Set zoom first to ensure proper positioning
        this.setZoom(this.overworldCameraState.zoom);

        // Then set scroll position
        this.mainCamera.setScroll(
          this.overworldCameraState.x,
          this.overworldCameraState.y
        );

        // Make sure we're in overworld mode
        this.isOverworld = true;

        console.log("Restored camera state from memory");
        return true;
      }

      return false;
    } catch (error) {
      console.error("Error restoring camera state:", error);
      return false;
    }
  }

  clearOverworldCameraState() {
    this.overworldCameraState = {
      x: 0,
      y: 0,
      zoom: DEFAULT_ZOOM,
      saved: false,
    };
    this.scene.game.registry.remove("overworldCameraState");
  }

  /**
   * Set the view mode to overworld or non-overworld and adjust zoom accordingly
   * @param isOverworld Whether the current view is the overworld
   */
  setViewMode(isOverworld: boolean) {
    // Only save the camera state if we're switching from overworld to map view
    if (this.isOverworld && !isOverworld) {
      // Check if we already have a saved state in the registry
      const existingState = this.scene.game.registry.get(
        "overworldCameraState"
      );

      // Only save if we don't have an existing state or we're not at default position
      if (
        !existingState ||
        !existingState.saved ||
        this.mainCamera.scrollX !== 0 ||
        this.mainCamera.scrollY !== 0 ||
        this.zoomLevel !== DEFAULT_ZOOM
      ) {
        this.saveOverworldCameraState();
      }
    }

    // Update the mode
    this.isOverworld = isOverworld;

    // Set appropriate zoom level based on view mode
    if (isOverworld) {
      // Don't set zoom here as we'll restore it in loadOverworldData
    } else {
      // Always use the default zoom for map views
      this.setZoom(NON_OVERWORLD_ZOOM);
      // Reset camera position for map views
      this.mainCamera.setScroll(0, 0);
    }
  }

  /**
   * Get the current view mode
   * @returns Whether the current view is in overworld mode
   */
  isInOverworldMode(): boolean {
    return this.isOverworld;
  }

  resetCamera() {
    // Reset zoom to appropriate default based on current view mode
    if (this.isOverworld) {
      this.setZoom(DEFAULT_ZOOM);
      this.clearOverworldCameraState();
    } else {
      // In map view, just reset the zoom but DON'T clear the saved overworld state
      this.setZoom(NON_OVERWORLD_ZOOM);
    }

    // Reset camera position
    this.mainCamera.setScroll(0, 0);

    this.resetPointerState();
  }

  private resetPointerState(): void {
    this.activePointers.clear();
    this.gesturePointerIds.clear();
    this.pinchState = null;
  }
}
