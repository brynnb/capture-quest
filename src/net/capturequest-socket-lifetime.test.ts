import {expect,test} from "vitest";
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
