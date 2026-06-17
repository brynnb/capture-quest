// Re-export Tygo-generated types for convenient importing
// Usage: import { CharacterData, CharacterWallet } from '@/types';

// World server types (hand-written Go structs with json tags)
export * from '../net/generated/world';

// Database model types from the owned server model package.
export * from '../net/generated/models';

// OpCodes for WebSocket communication
export * as OpCodes from '../net/generated/opcodes';
