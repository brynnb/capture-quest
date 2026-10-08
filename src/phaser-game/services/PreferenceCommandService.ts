import useGameStatusStore from "@/stores/GameStatusStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import useChatStore, { MessageType } from "@/stores/ChatStore";
import { OptionId } from "@/constants/optionId";
import type { PreferenceResponse } from "@/net/generated/world_api";
import { correlatedRequest } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";

// Preferences use the character/session lifetime and their own revision policy;
// correlation/timeout/listener cleanup come from the shared request primitive.
let active: AbortController | null = null;
export async function setTrainerPreference(enabled: boolean): Promise<void> {
  if (active) return;
  const characterId = usePlayerCharacterStore.getState().characterProfile.id;
  if (!characterId || useGameScreenStore.getState().currentScreen !== "game") return;
  const controller = new AbortController(); active = controller;
  const current = () => !controller.signal.aborted && usePlayerCharacterStore.getState().characterProfile.id === characterId;
  const stopCharacter = usePlayerCharacterStore.subscribe(state => { if (state.characterProfile.id !== characterId) controller.abort(); });
  const stopScreen = useGameScreenStore.subscribe(state => { if (state.currentScreen !== "game") controller.abort(); });
  const initial = useGameStatusStore.getState();
  const read = (currentView: boolean) => correlatedRequest<PreferenceResponse>(PhaserNet.onPreferences, requestId => PhaserNet.requestPreferences({ requestId, characterId, current: currentView, revision: initial.preferenceRevision, optionId: OptionId.AllowTrainerRebattles, value: enabled ? 1 : 0 }), controller.signal);
  const apply = (reply: PreferenceResponse) => {
    if (!current()) return;
    const state = useGameStatusStore.getState();
    if (reply.characterId !== characterId || !Number.isSafeInteger(reply.revision) || reply.revision < state.preferenceRevision || typeof reply.allowTrainerRebattles !== "boolean" || typeof reply.showNetworkStats !== "boolean") throw new Error("Invalid or stale preference snapshot");
    useGameStatusStore.setState({ preferenceRevision: reply.revision, allowTrainerRebattles: reply.allowTrainerRebattles, preferenceRecoveryRequired: false });
  };
  useGameStatusStore.setState({ preferenceCommandPending: true });
  try {
    try {
      const reply = await read(initial.preferenceRecoveryRequired);
      if (!initial.preferenceRecoveryRequired && reply.revision < initial.preferenceRevision + 1) throw new Error("Preference revision did not advance");
      apply(reply);
    }
    catch (error) {
      if (!current()) return;
      // Never resend a mutation. Read the committed current preference after
      // rejection, lost reply, or uncertain transport outcome.
      useGameStatusStore.setState({ preferenceRecoveryRequired: true });
      apply(await read(true));
    }
  } catch {
    if (current()) useChatStore.getState().addMessage("Preferences could not be confirmed. Try again to recover their current state.", MessageType.SYSTEM_ERROR);
  } finally {
    stopCharacter(); stopScreen();
    if (active === controller) { active = null; useGameStatusStore.setState({ preferenceCommandPending: false }); }
  }
}
