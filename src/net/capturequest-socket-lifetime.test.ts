import {afterEach,expect,test,vi} from "vitest";
import {CaptureQuestSocket} from "./capturequest-socket";
import * as OpCodes from "./generated/opcodes";
function fakeWebSockets(){
 class FakeWebSocket {
  static OPEN=1;readyState=1;binaryType="arraybuffer";
  onopen:(()=>void)|null=null;onclose:(()=>void)|null=null;onerror:(()=>void)|null=null;onmessage:((event:MessageEvent)=>void)|null=null;
  close=vi.fn();send=vi.fn();constructor(){created.push(this)}
 }
 const created:FakeWebSocket[]=[];vi.stubGlobal("WebSocket",FakeWebSocket);return created;
}
afterEach(()=>{vi.unstubAllGlobals();vi.useRealTimers();});
test("close and authentication attempt retire the old session read generation",async()=>{
 const socket=new CaptureQuestSocket({allowReconnect:false});let retired=0;
 const stop=socket.subscribeSessionRetirement(()=>retired++);
 socket.close(false);expect(socket.sessionGeneration).toBe(1);expect(retired).toBe(1);
 await expect(socket.sendJsonRequest(OpCodes.JWTLogin,OpCodes.JWTResponse,{})).rejects.toThrow("Not connected");
 expect(socket.sessionGeneration).toBe(2);expect(retired).toBe(2);
 stop();socket.close(false);expect(retired).toBe(2);
});

test("retirement observers cannot read a closing connection or interrupt cleanup",()=>{
 const socket=new CaptureQuestSocket({allowReconnect:false});socket.isConnected=true;
 const log=vi.spyOn(console,"error").mockImplementation(()=>{});
 let available=true;let laterNotified=false;
 socket.subscribeSessionRetirement(()=>{available=socket.isConnected;throw new Error("broken observer");});
 socket.subscribeSessionRetirement(()=>{laterNotified=true;});
 socket.close(false);
 expect(available).toBe(false);expect(laterNotified).toBe(true);expect(socket.isConnected).toBe(false);
 expect(log).toHaveBeenCalledOnce();log.mockRestore();
});

test("manual connection retires a previously scheduled reconnect",async()=>{
 vi.useFakeTimers();
 const created=fakeWebSockets();
 const socket=new CaptureQuestSocket({allowReconnect:true});
 Object.assign(socket,{useWebSocket:true,onClose:()=>{}});
 (socket as unknown as {scheduleReconnect:()=>void}).scheduleReconnect();
 const connecting=socket.connect("localhost",4433,()=>{});
 created[0].onopen!();expect(await connecting).toBe(true);
 await vi.advanceTimersByTimeAsync(1000);
 expect(created).toHaveLength(1);
 socket.close(false);vi.unstubAllGlobals();vi.useRealTimers();
});

test("explicit close cancels a queued reconnect and duplicate scheduling is coalesced",async()=>{
 vi.useFakeTimers();
 const socket=new CaptureQuestSocket({allowReconnect:true});
 const control=socket as unknown as {scheduleReconnect:()=>void;connectWebSocket:(onClose:()=>void)=>Promise<boolean>};
 const connect=vi.spyOn(control,"connectWebSocket").mockResolvedValue(false);
 Object.assign(socket,{useWebSocket:true,onClose:()=>{}});
 control.scheduleReconnect();control.scheduleReconnect();expect(vi.getTimerCount()).toBe(1);
 socket.close(false);expect(vi.getTimerCount()).toBe(0);
 await vi.advanceTimersByTimeAsync(1000);expect(connect).not.toHaveBeenCalled();
});

test("pending replacement settles the old attempt and stale callbacks cannot take ownership",async()=>{
 vi.useFakeTimers();
 const created=fakeWebSockets();
 const socket=new CaptureQuestSocket({allowReconnect:false});
 const first=socket.connect("localhost",4433,()=>{});
 const stale={open:created[0].onopen!,close:created[0].onclose!,message:created[0].onmessage!};
 const second=socket.connect("localhost",4433,()=>{});created[1].onopen!();expect(await second).toBe(true);
 let firstSettled:boolean|undefined;void first.then(value=>{firstSettled=value});await Promise.resolve();
 expect(firstSettled).toBe(false);
 const receive=vi.fn();socket.onJson=receive;
 const json=new TextEncoder().encode('{"timestamp":1}');const bytes=new Uint8Array(6+json.length);
 const view=new DataView(bytes.buffer);view.setUint32(0,2+json.length,true);view.setUint16(4,OpCodes.Heartbeat,true);bytes.set(json,6);
 stale.open();stale.message(new MessageEvent("message",{data:bytes.buffer}));stale.close();
 expect(socket.isConnected).toBe(true);expect(receive).not.toHaveBeenCalled();expect(created[1].close).not.toHaveBeenCalled();
 socket.close(false);
});

test("close during setup settles the attempt and late open cannot resurrect it",async()=>{
 vi.useFakeTimers();const created=fakeWebSockets();const socket=new CaptureQuestSocket({allowReconnect:false});
 const connecting=socket.connect("localhost",4433,()=>{});const open=created[0].onopen!;
 socket.close(false);expect(await connecting).toBe(false);open();
 expect(socket.isConnected).toBe(false);expect(created[0].close).toHaveBeenCalledOnce();
});

test("retired automatic attempt cannot schedule retries over a pending manual attempt",async()=>{
 vi.useFakeTimers();const created=fakeWebSockets();const socket=new CaptureQuestSocket({allowReconnect:true});
 Object.assign(socket,{useWebSocket:true,onClose:()=>{}});
 (socket as unknown as {scheduleReconnect:()=>void}).scheduleReconnect();await vi.advanceTimersByTimeAsync(1000);
 expect(created).toHaveLength(1);
 const manual=socket.connect("localhost",4433,()=>{});await Promise.resolve();await Promise.resolve();
 expect(vi.getTimerCount()).toBe(0);created[1].onopen!();expect(await manual).toBe(true);socket.close(false);
});
