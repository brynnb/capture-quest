import type { Page } from "@playwright/test";
import * as OpCodes from "../../../src/net/generated/opcodes";

// Exercise each adapter through the same real transport fault boundary. Suppress
// duplicate rejections so the first committed reply (or its timeout) settles it.
export async function inventoryCommandFaults<T>(page: Page, requestOpcode: number, responseOpcode: number, loseReply: boolean, matchesRequest: (request: Record<string, unknown>) => boolean = () => true) {
  const evidence = {requests: 0, successes: 0, rejections: 0, replies: [] as T[], deliverReply: (_index: number) => {}};
  const deliveries: Array<() => void> = [];
  const faultedRequests = new Set<string>();
  evidence.deliverReply = index => deliveries[index]();
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === requestOpcode
        && matchesRequest(JSON.parse(message.subarray(6).toString()))) {
        const request = JSON.parse(message.subarray(6).toString());
        if (typeof request.requestId === "string") faultedRequests.add(request.requestId);
        evidence.requests++;
        server.send(message); // Exact duplicate, including character/revision.
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && evidence.requests > 0) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.CQInventoryResponse) return;
        if (opcode === responseOpcode) {
          const reply = JSON.parse(message.subarray(6).toString());
          // A shared opcode may also carry recovery reads. Fault only replies
          // to the selected command IDs, leaving current reads observable.
          if (!faultedRequests.has(reply.requestId)) { socket.send(message); return; }
          if (!reply.success) { evidence.rejections++; return; }
          evidence.successes++; evidence.replies.push(reply); deliveries.push(() => socket.send(message));
          if (loseReply) return;
        }
      }
      socket.send(message);
    });
  });
  return evidence;
}
