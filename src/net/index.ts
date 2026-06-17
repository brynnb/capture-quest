// Re-export the CaptureQuestSocket class and create singleton instances
import { CaptureQuestSocket } from "./capturequest-socket";

// Singleton socket instances for world and zone connections
export const WorldSocket = new CaptureQuestSocket({
  allowReconnect: true,
  maxRetries: 5,
});
export const ZoneSocket = new CaptureQuestSocket({ allowReconnect: true, maxRetries: 5 });

// Re-export types and utilities
export { CaptureQuestSocket } from "./capturequest-socket";
// Use tygo-generated opcodes for correct values matching the Go server
export * as OpCodes from "./generated/opcodes";
