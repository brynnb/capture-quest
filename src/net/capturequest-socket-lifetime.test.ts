import {afterEach,expect,test,vi} from "vitest";
import {CaptureQuestSocket} from "./capturequest-socket";
import * as OpCodes from "./generated/opcodes";
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
 const created:Array<{onopen:(()=>void)|null;close:ReturnType<typeof vi.fn>}>=[];
 class FakeWebSocket {
  static OPEN=1;readyState=1;binaryType="arraybuffer";
  onopen:(()=>void)|null=null;onclose:(()=>void)|null=null;onerror:(()=>void)|null=null;onmessage:unknown=null;
  close=vi.fn();send=vi.fn();constructor(){created.push(this)}
 }
 vi.stubGlobal("WebSocket",FakeWebSocket);
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
