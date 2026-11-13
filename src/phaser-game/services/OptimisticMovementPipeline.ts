import type { PlayerStepCompleteResponse, PlayerStepResponse } from "@/net/generated/protocol";
import { readOwnedPlayerPosition, submitPredictedPlayerStep } from "./PlayerMovementService";

export interface PredictedPlayerStep {
  sequence: number;
  mapId: number;
  sourceX: number;
  sourceY: number;
  x: number;
  y: number;
  direction: string;
  ledgeJump?: boolean;
}

type StartedStep = PredictedPlayerStep & {
  commandToken: string;
  abort: AbortController;
};

interface MovementPipelineCallbacks {
  confirmed: (step: PredictedPlayerStep, result: PlayerStepCompleteResponse) => void;
  boundary: (step: PredictedPlayerStep, result: PlayerStepCompleteResponse) => void;
  failed: (error: unknown) => void;
  changed: () => void;
}

const PREDICTION_LIMIT = 8;

function commandToken(): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), byte => byte.toString(16).padStart(2, "0")).join("");
}

/**
 * Presentation starts locally valid tiles immediately. This bounded ledger owns
 * one ordered command per started tile, independently of animation or replies.
 * A server-issued stream and strict sequence retain authority across turns,
 * rejection, effect boundaries, and scene retirement.
 */
export class OptimisticMovementPipeline {
  private queued: StartedStep[] = [];
  private submitted = new Map<number, StartedStep>();
  private sequence = 0;
  private confirmedSequence = 0;
  private streamId: string | null = null;
  private bootstrap: AbortController | null = null;
  private retired = false;

  constructor(private readonly callbacks: MovementPipelineCallbacks) {}

  canPredict(): boolean {
    return !this.retired && this.sequence - this.confirmedSequence < PREDICTION_LIMIT;
  }

  isConfirmed(sequence: number): boolean {
    return sequence <= this.confirmedSequence;
  }

  isBusy(): boolean {
    return !this.retired && (this.sequence > this.confirmedSequence || this.bootstrap !== null);
  }

  start(step: Omit<PredictedPlayerStep, "sequence">): PredictedPlayerStep | null {
    if (!this.canPredict()) return null;
    const started: StartedStep = {
      ...step, sequence: ++this.sequence, commandToken: commandToken(), abort: new AbortController(),
    };
    this.queued.push(started);
    this.pump();
    return started;
  }

  // There are no autonomous future commands to cancel: only already-started
  // visual tiles enter the ledger. A released key simply stops enqueueing.
  finishIntent(): void {}

  dispose(): void {
    if (this.retired) return;
    this.retired = true;
    this.bootstrap?.abort();
    this.bootstrap = null;
    for (const step of this.submitted.values()) step.abort.abort();
    for (const step of this.queued) step.abort.abort();
    this.queued = [];
    this.submitted.clear();
  }

  private pump(): void {
    if (this.retired || this.bootstrap) return;
    if (!this.streamId) {
      const first = this.queued[0];
      if (!first) return;
      const abort = new AbortController();
      this.bootstrap = abort;
      // This read is ordered after previous scene writes. Visual prediction does
      // not wait for it, but no command uses an old or client-invented stream.
      void readOwnedPlayerPosition(abort.signal, undefined, true).then(owned => {
        if (this.retired || abort.signal.aborted) return;
        this.bootstrap = null;
        if (owned.serverMovementPending || owned.mapId !== first.mapId || owned.x !== first.sourceX || owned.y !== first.sourceY) {
          this.dispose();
          this.callbacks.boundary(first, owned);
          return;
        }
        if (!owned.movementStreamId || (owned.movementSequence ?? 0) !== 0) {
          this.fail(new Error("Movement stream was not initialized"));
          return;
        }
        this.streamId = owned.movementStreamId;
        this.pump();
        this.callbacks.changed();
      }).catch(error => {
        this.bootstrap = null;
        if (!this.retired && !abort.signal.aborted) this.fail(error);
      });
      return;
    }
    while (this.queued.length > 0) {
      const step = this.queued.shift()!;
      this.submitted.set(step.sequence, step);
      // The reliable transport preserves these writes in sequence. The server
      // validates source/collision and paces each commit without another grant RTT.
      void submitPredictedPlayerStep({
        commandToken: step.commandToken, movementStreamId: this.streamId, sequence: step.sequence,
        mapId: step.mapId, fromX: step.sourceX, fromY: step.sourceY, direction: step.direction,
      }, step.abort.signal).then(result => this.receive(step, result)).catch(error => {
        if (!this.retired && !step.abort.signal.aborted && step.sequence > this.confirmedSequence) this.fail(error);
      });
    }
  }

  private receive(step: StartedStep, result: PlayerStepResponse): void {
    if (this.retired || step.abort.signal.aborted || step.sequence < this.confirmedSequence) return;
    if (!result.committed || result.replayed) {
      this.fail(new Error("Movement receipt requires current owned-position recovery"));
      return;
    }
    this.confirmedSequence = Math.max(this.confirmedSequence, step.sequence);
    // Strict server sequencing means a confirmed child proves its predecessors
    // committed. Retire older lost-reply listeners instead of retrying old moves.
    for (const [sequence, previous] of this.submitted) {
      if (sequence <= step.sequence) {
        this.submitted.delete(sequence);
        if (sequence < step.sequence) previous.abort.abort();
      }
    }
    if (result.stopPath || result.mapId !== step.mapId || result.x !== step.x || result.y !== step.y) {
      this.dispose();
      this.callbacks.boundary(step, result);
      return;
    }
    this.callbacks.confirmed(step, result);
    this.callbacks.changed();
  }

  private fail(error: unknown): void {
    if (this.retired) return;
    this.dispose();
    this.callbacks.failed(error);
  }
}
