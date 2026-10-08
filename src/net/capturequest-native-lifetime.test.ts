import {afterEach,expect,test,vi} from "vitest";
import {CaptureQuestSocket} from "./capturequest-socket";
import * as OpCodes from "./generated/opcodes";

function deferred<T>(){let resolve!:(value:T)=>void;const promise=new Promise<T>(r=>{resolve=r});return {promise,resolve};}
function nativeFixture(){
 vi.useFakeTimers();vi.spyOn(navigator,"userAgent","get").mockReturnValue("Chrome");
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue({text:async()=>btoa(String.fromCharCode(...new Uint8Array(32)))}));
 class Native {
  readyStage=deferred<void>();closedStage=deferred<{closeCode:number}>();
  ready=this.readyStage.promise;closed=this.closedStage.promise;
  datagramController!:ReadableStreamDefaultController<Uint8Array>;controlController!:ReadableStreamDefaultController<Uint8Array>;
  write=vi.fn();
  datagrams={readable:new ReadableStream<Uint8Array>({start:c=>{this.datagramController=c}}),writable:new WritableStream<Uint8Array>({write:bytes=>this.write(bytes)})};
  control={readable:new ReadableStream<Uint8Array>({start:c=>{this.controlController=c}}),writable:new WritableStream<Uint8Array>()};
  close=vi.fn(()=>this.closedStage.resolve({closeCode:0}));
  createBidirectionalStream=vi.fn(async()=>this.control);
  constructor(){created.push(this)}
 }
 const created:Native[]=[];vi.stubGlobal("WebTransport",Native);return created;
}
afterEach(()=>{vi.unstubAllGlobals();vi.restoreAllMocks();vi.useRealTimers();});

test("replacement cancels pending native handshake without waiting for closed",async()=>{
 const created=nativeFixture();const socket=new CaptureQuestSocket({allowReconnect:false});
 const old=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);expect(created).toHaveLength(1);
 const next=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);expect(await old).toBe(false);expect(created).toHaveLength(2);
 created[1].readyStage.resolve();await vi.advanceTimersByTimeAsync(0);expect(await next).toBe(true);
 created[0].readyStage.resolve();await vi.advanceTimersByTimeAsync(0);
 expect(socket.isConnected).toBe(true);expect(created[0].close).toHaveBeenCalledOnce();expect(created[1].close).not.toHaveBeenCalled();socket.close(false);
});

test("old native closure cannot retire its replacement",async()=>{
 const created=nativeFixture();const socket=new CaptureQuestSocket({allowReconnect:false});
 const old=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);created[0].readyStage.resolve();await vi.advanceTimersByTimeAsync(0);expect(await old).toBe(true);
 const next=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);created[1].readyStage.resolve();await vi.advanceTimersByTimeAsync(0);expect(await next).toBe(true);
 expect(socket.isConnected).toBe(true);expect(created[1].close).not.toHaveBeenCalled();socket.close(false);
});

test("queued native data cannot publish after retirement and readers release locks",async()=>{
 const created=nativeFixture();const socket=new CaptureQuestSocket({allowReconnect:false});
 const connect=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);created[0].readyStage.resolve();await vi.advanceTimersByTimeAsync(0);expect(await connect).toBe(true);
 const receive=vi.fn();socket.onJson=receive;
 const json=new TextEncoder().encode('{"timestamp":1}');const frame=new Uint8Array(6+json.length);const view=new DataView(frame.buffer);
 view.setUint32(0,2+json.length,true);view.setUint16(4,OpCodes.Heartbeat,true);frame.set(json,6);
 created[0].controlController.enqueue(frame);created[0].datagramController.enqueue(frame.slice(4));socket.close(false);await vi.advanceTimersByTimeAsync(0);
 expect(receive).not.toHaveBeenCalled();expect(created[0].control.readable.locked).toBe(false);expect(created[0].datagrams.readable.locked).toBe(false);
});

test("queued old datagram cannot use a replacement writer",async()=>{
 const created=nativeFixture();const socket=new CaptureQuestSocket({allowReconnect:false});
 const old=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);created[0].readyStage.resolve();await vi.advanceTimersByTimeAsync(0);await old;
 const blocked=deferred<void>();Object.assign(socket,{writeQueue:blocked.promise});
 const retired=socket.sendJsonMessage(OpCodes.Heartbeat,{timestamp:1}).then(()=>"sent",error=>(error as Error).name);
 const next=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);created[1].readyStage.resolve();await vi.advanceTimersByTimeAsync(0);await next;
 await socket.sendJsonMessage(OpCodes.Heartbeat,{timestamp:2});expect(created[1].write).toHaveBeenCalledOnce();
 blocked.resolve();await vi.advanceTimersByTimeAsync(0);expect(await retired).toBe("AbortError");expect(created[1].write).toHaveBeenCalledOnce();socket.close(false);
});

test("retired hash-fetch continuation cannot create a transport or fallback",async()=>{
 const created=nativeFixture();const hash=deferred<{text:()=>Promise<string>}>();
 vi.mocked(fetch).mockReturnValueOnce(hash.promise as Promise<Response>);
 const socket=new CaptureQuestSocket({allowReconnect:false});const old=socket.connect("localhost",4433,()=>{});socket.close(false);
 await vi.advanceTimersByTimeAsync(0);expect(await old).toBe(false);
 hash.resolve({text:async()=>"AA=="});await vi.advanceTimersByTimeAsync(0);
 expect(created).toHaveLength(0);expect(socket.isConnected).toBe(false);
});

test("a control stream arriving after cancellation is disposed instead of attached",async()=>{
 const created=nativeFixture();const socket=new CaptureQuestSocket({allowReconnect:false});
 const connect=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);
 const stream=deferred<typeof created[0]["control"]>();created[0].createBidirectionalStream.mockReturnValueOnce(stream.promise);
 created[0].readyStage.resolve();await vi.advanceTimersByTimeAsync(0);socket.close(false);await vi.advanceTimersByTimeAsync(0);expect(await connect).toBe(false);
 stream.resolve(created[0].control);await vi.advanceTimersByTimeAsync(0);
 const reader=created[0].control.readable.getReader();expect(await reader.read()).toEqual({value:undefined,done:true});reader.releaseLock();
 expect(socket.isConnected).toBe(false);
});

test("reconnect scheduling cannot compete with pending native setup",async()=>{
 const created=nativeFixture();const socket=new CaptureQuestSocket({allowReconnect:true});
 const connect=socket.connect("localhost",4433,()=>{});await vi.advanceTimersByTimeAsync(0);expect(created).toHaveLength(1);
 (socket as unknown as {scheduleReconnect:()=>void}).scheduleReconnect();
 expect(vi.getTimerCount()).toBe(1); // the owning handshake deadline only
 socket.close(false);await vi.advanceTimersByTimeAsync(0);expect(await connect).toBe(false);expect(vi.getTimerCount()).toBe(0);
});
