import {expect,test,vi} from "vitest";
import {CaptureQuestSocket} from "./capturequest-socket";
import * as OpCodes from "./generated/opcodes";
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
