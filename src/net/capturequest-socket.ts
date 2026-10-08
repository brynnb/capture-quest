import * as OpCodes from "./generated/opcodes";
import type { OpCode } from "./generated/opcodes";
import { FORCE_WEBSOCKET, getApiUrl, getWsUrl } from "@/config";

interface WebTransportOptions {
  serverCertificateHashes?: Array<{
    algorithm: "sha-256";
    value: ArrayBuffer;
  }>;
  allowPooling?: boolean;
  congestionControl?: "default" | "low-latency" | "throughput";
}

interface WebTransportBidirectionalStream {
  readable: ReadableStream<Uint8Array>;
  writable: WritableStream<Uint8Array>;
}

interface WebTransport {
  readonly datagrams: {
    readonly writable: WritableStream<Uint8Array>;
    readonly readable: ReadableStream<Uint8Array>;
  };
  readonly incomingBidirectionalStreams: ReadableStream<WebTransportBidirectionalStream>;
  readonly ready: Promise<void>;
  readonly closed: Promise<{ reason?: string; closeCode?: number }>;
  close(closeInfo?: { closeCode?: number; reason?: string }): void;
  createBidirectionalStream(): Promise<WebTransportBidirectionalStream>;
}

const TRANSPORT_CONNECT_TIMEOUT_MS = 8_000;

async function withTimeout<T>(
  promise: Promise<T>,
  timeoutMs: number,
  message: string,
  signal?:AbortSignal,
): Promise<T> {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  let abort:(()=>void)|undefined;
  const cancelled=new Promise<never>((_,reject)=>{
    abort=()=>reject(new DOMException("Connection attempt retired","AbortError"));
    signal?.addEventListener("abort",abort,{once:true});if(signal?.aborted)abort();
  });
  const timeoutPromise = new Promise<never>((_, reject) => {
    timeout = setTimeout(() => reject(new Error(message)), timeoutMs);
  });

  try {
    return await Promise.race([promise, timeoutPromise,cancelled]);
  } finally {
    if (timeout) clearTimeout(timeout);
    if(abort)signal?.removeEventListener("abort",abort);
  }
}

function isAppleWebKitBrowser(): boolean {
  const userAgent = navigator.userAgent;
  return (
    /AppleWebKit/i.test(userAgent) &&
    !/(Chrome|Chromium|Edg|OPR|SamsungBrowser)/i.test(userAgent)
  );
}

function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binaryString = atob(base64);
  const bytes = new Uint8Array(binaryString.length);
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }
  return bytes.buffer;
}

/*
function concatArrayBuffer(a: ArrayBuffer, b: ArrayBuffer): Uint8Array {
  const c = new Uint8Array(a.byteLength + b.byteLength);
  c.set(new Uint8Array(a), 0);
  c.set(new Uint8Array(b), a.byteLength);
  return c;
}
*/

function concatUint8(a: Uint8Array, b: Uint8Array): Uint8Array {
  const c = new Uint8Array(a.length + b.length);
  c.set(a, 0);
  c.set(b, a.length);
  return c;
}

// Pending request tracking for request/response pattern
interface PendingRequest<T> {
  resolve: (value: T) => void;
  reject: (error: Error) => void;
  timeout: ReturnType<typeof setTimeout>;
}

export class CaptureQuestSocket {
  private webtransport: WebTransport | null = null;
  private datagramWriter: WritableStreamDefaultWriter<Uint8Array> | null = null;
  private controlWriter: WritableStreamDefaultWriter<Uint8Array> | null = null;
  private writeQueue: Promise<void> = Promise.resolve();
  private opCodeHandlers: {
    [opcode: number]: (payload: Uint8Array) => void;
  } = {};

  // WebSocket fallback (Safari / iOS)
  private ws: WebSocket | null = null;
  private useWebSocket = false;
  private wsBuffer: Uint8Array = new Uint8Array(0);

  // Request/response tracking - queue per opcode for concurrent requests
  private pendingRequests: Map<OpCode, PendingRequest<Uint8Array>[]> = new Map();

  public isConnected = false;
  private onClose: (() => void) | null = null;
  public onDisconnect: (() => void) | null = null;
  private isClosing = false; // Track intentional close to suppress expected errors

  // Reconnect
  private url: string | null = null;
  private port: number | string | null = null;
  private allowReconnect: boolean;
  private maxRetries: number;
  private retryCount = 0;

  // Heartbeat
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private cancelWebSocketConnect: (()=>void) | null = null;
  private webSocketAttemptGeneration = 0;
  private nativeAttempt:AbortController|null=null;
  private nativeReaders=new Set<{cancel:(reason?:unknown)=>Promise<void>}>();
  public latency = 0;
  public onPing: ((latency: number) => void) | null = null;
  private ownerGeneration = 0;
  private ownerRetireListeners = new Set<() => void>();
  public get sessionGeneration(): number { return this.ownerGeneration; }
  public subscribeSessionRetirement(receive: () => void): () => void {
    this.ownerRetireListeners.add(receive); return () => this.ownerRetireListeners.delete(receive);
  }
  private retireSessionReads(): void {
    this.ownerGeneration++;
    // A view subscriber must not prevent transport cleanup or another owner
    // from receiving retirement.
    for (const receive of [...this.ownerRetireListeners]) {
      try { receive(); } catch (error) { console.error("[Socket] Session read retirement failed:", error); }
    }
  }
  public onJson: ((opcode: OpCode, data: unknown) => void) | null = null;

  constructor(config: { maxRetries?: number; allowReconnect?: boolean } = {}) {
    this.allowReconnect = config.allowReconnect ?? true;
    this.maxRetries = config.maxRetries ?? 5;
    this.close = this.close.bind(this);
    window.addEventListener("beforeunload", () => this.close(false));
  }

  public setSessionId() {
    // Session ID no longer used for now
  }

  public async connect(
    url: string,
    port: number | string,
    onClose: () => void
  ): Promise<boolean> {
    this.clearReconnectTimer();
    this.isConnected = false;
    this.retireWebSocket();
    this.retireNativeTransport();
    this.retireSessionReads();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const WT = (window as any).WebTransport as {
      new(url: string, opts?: WebTransportOptions): WebTransport;
    };

    this.url = url;
    this.port = port;
    this.onClose = onClose;

    console.log(`[CaptureQuestSocket] Environment check: SecureContext=${window.isSecureContext}, WebTransportSupport=${!!WT}, Host=${window.location.hostname}`);

    if (FORCE_WEBSOCKET) {
      console.log("[CaptureQuestSocket] VITE_FORCE_WEBSOCKET=true, using WebSocket transport");
      return this.connectWebSocket(onClose);
    }

    // Safari 26.4 can expose WebTransport while hanging during setup with
    // non-Apple servers. All iOS browsers use WebKit, so prefer the reliable
    // WebSocket path there as well.
    if (isAppleWebKitBrowser()) {
      console.log("[CaptureQuestSocket] WebKit browser detected, using WebSocket transport");
      return this.connectWebSocket(onClose);
    }

    if (!WT) {
      console.warn("[CaptureQuestSocket] WebTransport not supported, falling back to WebSocket");
      return this.connectWebSocket(onClose);
    }

    this.useWebSocket = false;

    const attempt=new AbortController();this.nativeAttempt=attempt;
    const current=()=>this.nativeAttempt===attempt && !attempt.signal.aborted;
    try {
      const hash=await withTimeout(fetch(getApiUrl("/hash"),{signal:attempt.signal}).then(r=>r.text()),5000,"Certificate hash fetch timed out",attempt.signal);
      if(!current())return false;
      const transport=new WT(`https://${url}:${port}/cq`,{serverCertificateHashes:[{algorithm:"sha-256",value:base64ToArrayBuffer(hash)}]});
      this.webtransport=transport;
      const owns=()=>current() && this.webtransport===transport;
      // Handshake failure may reject closed before the established-close hook
      // is attached. The setup error owns reporting and fallback in that phase.
      void transport.closed.catch(()=>{});
      await withTimeout(transport.ready,TRANSPORT_CONNECT_TIMEOUT_MS,"WebTransport handshake timed out",attempt.signal);
      if(!owns())return false;
      this.datagramWriter=transport.datagrams.writable.getWriter();
      this.startDatagramLoop(transport);
      const streamPromise=transport.createBidirectionalStream();
      // A stream created after cancellation is still owned by the old attempt.
      void streamPromise.then(stream=>{
        if(!owns()){void stream.readable.cancel().catch(()=>{});void stream.writable.abort().catch(()=>{});}
      },()=>{});
      const stream=await withTimeout(streamPromise,TRANSPORT_CONNECT_TIMEOUT_MS,"WebTransport control stream timed out",attempt.signal);
      if(!owns())return false;
      this.attachControlStream(stream,transport);
      this.isConnected=true;this.isClosing=false;this.retryCount=0;this.startHeartbeat();
      void transport.closed.then(()=>{if(owns())this.close(false);},error=>{
        if(owns()){console.error("WebTransport closed with error:",error);this.close(false);}
      });
      return true;
    } catch(error) {
      if(!current())return false;
      console.warn("[CaptureQuestSocket] WebTransport failed; falling back to WebSocket:",error);
      this.retireNativeTransport();
      const connected=await this.connectWebSocket(onClose);
      if(!connected)this.scheduleReconnect();
      return connected;
    }
  }


  public async sendJsonMessage(
    opCode: number,
    data: unknown
  ): Promise<void> {
    const json = JSON.stringify(data);
    const payload = new TextEncoder().encode(json);
    const op = new Uint16Array([opCode]).buffer;
    const packet = concatUint8(new Uint8Array(op), payload);
    await this.sendDatagram(packet);
  }

  public async sendStreamJsonMessage(
    opCode: number,
    data: unknown
  ): Promise<void> {
    const json = JSON.stringify(data);
    const payload = new TextEncoder().encode(json);

    // [length:uint32_LE][opcode:uint16_LE][payload]
    const header = new ArrayBuffer(4);
    new DataView(header).setUint32(0, 2 + payload.byteLength, true);
    const op = new Uint16Array([opCode]).buffer;

    const frame = concatUint8(
      new Uint8Array(header),
      concatUint8(new Uint8Array(op), payload)
    );

    // WebSocket fallback: send frame directly over WebSocket
    if (this.useWebSocket) {
      this.sendWsFrame(frame);
      return;
    }

    if (!this.controlWriter) {
      throw new Error("Control stream not open");
    }
    await this.controlWriter.write(frame);
  }



  public registerJsonHandler<T = unknown>(
    opCode: OpCode,
    handler: (msg: T) => void
  ) {
    this.opCodeHandlers[opCode] = (buf: Uint8Array) => {
      try {
        const text = new TextDecoder().decode(buf);
        const json = JSON.parse(text);

        // If no schema provided or using a simplified approach, pass raw JSON
        handler(json);
      } catch (e) {
        console.error(`JSON parse error for opcode ${opCode}:`, e);
      }
    };
  }

  public unregisterJsonHandler(opCode: OpCode) {
    if (this.opCodeHandlers[opCode]) {
      delete this.opCodeHandlers[opCode];
    }
  }


  /** Send a JSON request and wait for a response with the specified opcode */
  public async sendJsonRequest<TRes = unknown>(
    requestOpCode: OpCode,
    responseOpCode: OpCode,
    data: unknown,
    timeoutMs: number = 10000
  ): Promise<TRes> {
    if (requestOpCode === OpCodes.JWTLogin) this.retireSessionReads();
    if (!this.isConnected || (!this.controlWriter && !this.useWebSocket)) {
      throw new Error("Not connected");
    }

    const json = JSON.stringify(data);
    const payload = new TextEncoder().encode(json);

    const header = new ArrayBuffer(4);
    new DataView(header).setUint32(0, 2 + payload.byteLength, true);
    const op = new Uint16Array([requestOpCode]).buffer;

    const frame = concatUint8(
      new Uint8Array(header),
      concatUint8(new Uint8Array(op), payload)
    );

    // Create promise for response
    const responsePromise = new Promise<TRes>((resolve, reject) => {
      const pendingRequest: PendingRequest<Uint8Array> = {
        resolve: (buf: Uint8Array) => {
          try {
            const text = new TextDecoder().decode(buf);
            const json = JSON.parse(text);

            resolve(json as TRes);
          } catch (e) {
            reject(new Error(`JSON decode error for opcode ${responseOpCode}: ${e}`));
          }
        },
        reject,
        timeout: setTimeout(() => {
          // Remove this specific request from the queue on timeout
          const queue = this.pendingRequests.get(responseOpCode);
          if (queue) {
            const idx = queue.indexOf(pendingRequest);
            if (idx !== -1) queue.splice(idx, 1);
            if (queue.length === 0) this.pendingRequests.delete(responseOpCode);
          }
          reject(new Error(`Request timeout for opcode ${responseOpCode}`));
        }, timeoutMs),
      };

      // Add to queue for this opcode
      const queue = this.pendingRequests.get(responseOpCode) || [];
      queue.push(pendingRequest);
      this.pendingRequests.set(responseOpCode, queue);
    });

    // Send the request
    if (this.useWebSocket) {
      this.sendWsFrame(frame);
    } else {
      await this.controlWriter!.write(frame);
    }

    return responsePromise;
  }



  public close(scheduleReconnect: boolean = true) {
    this.clearReconnectTimer();
    this.retirePendingRequests();
    // Observers must see an unavailable connection before retirement can
    // synchronously trigger another read.
    this.isClosing = true;
    this.isConnected = false;
    this.retireSessionReads();

    this.retireNativeTransport();

    this.retireWebSocket();

    if (scheduleReconnect && this.allowReconnect) {
      this.scheduleReconnect();
    } else {
      this.onClose?.();
      this.onDisconnect?.();
    }
  }

  // ——— WebSocket fallback ———

  private retireNativeTransport(){
    if(this.webtransport || this.nativeAttempt)this.retirePendingRequests();
    const attempt=this.nativeAttempt;this.nativeAttempt=null;attempt?.abort();
    const transport=this.webtransport;this.webtransport=null;
    for(const reader of this.nativeReaders)void reader.cancel().catch(()=>{});
    this.nativeReaders.clear();
    this.datagramWriter?.releaseLock();this.datagramWriter=null;
    this.controlWriter?.releaseLock();this.controlWriter=null;
    this.writeQueue=Promise.resolve();
    transport?.close();
  }

  private retireWebSocket() {
    if(this.ws || this.cancelWebSocketConnect)this.retirePendingRequests();
    this.webSocketAttemptGeneration++;
    const cancel=this.cancelWebSocketConnect;this.cancelWebSocketConnect=null;cancel?.();
    if(this.ws){
      const ws=this.ws;this.ws=null;
      ws.onopen=null;ws.onclose=null;ws.onerror=null;ws.onmessage=null;ws.close();
    }
    this.wsBuffer=new Uint8Array(0);
    if(this.heartbeatInterval){clearInterval(this.heartbeatInterval);this.heartbeatInterval=null;}
  }

  private retirePendingRequests() {
    // Untagged replies cannot cross a physical transport boundary. Authentication
    // read-generation changes on the same socket do not reorder its FIFO queue.
    const queues=[...this.pendingRequests.values()];this.pendingRequests.clear();
    for(const queue of queues)for(const pending of queue){
      clearTimeout(pending.timeout);pending.reject(new Error("Connection session retired"));
    }
  }

  private async connectWebSocket(onClose: () => void): Promise<boolean> {
    this.clearReconnectTimer();
    this.isConnected=false;
    this.retireWebSocket();
    this.onClose = onClose;
    this.useWebSocket = true;

    return new Promise<boolean>((resolve) => {
      const wsUrl = getWsUrl("/ws");
      console.log(`[CaptureQuestSocket] Connecting via WebSocket to ${wsUrl}...`);
      const ws = new WebSocket(wsUrl);
      this.ws=ws;
      ws.binaryType = "arraybuffer";
      const owns=()=>this.ws===ws;
      let settled=false;
      let setupTimeout:ReturnType<typeof setTimeout>|undefined;
      const finish=(connected:boolean)=>{
        if(settled)return;settled=true;
        if(setupTimeout!==undefined)clearTimeout(setupTimeout);
        if(this.cancelWebSocketConnect===cancel)this.cancelWebSocketConnect=null;
        resolve(connected);
      };
      const cancel=()=>finish(false);
      this.cancelWebSocketConnect=cancel;
      setupTimeout=setTimeout(()=>{
        finish(false);
        if(owns())this.close(true);
      },TRANSPORT_CONNECT_TIMEOUT_MS);

      ws.onopen = () => {
        if(!owns()){finish(false);return;}
        console.log("[CaptureQuestSocket] WebSocket connection established");
        this.isConnected = true;
        this.isClosing = false;
        this.retryCount = 0;
        this.startHeartbeat();
        finish(true);
      };

      ws.onerror = (e) => {
        if(!owns()){finish(false);return;}
        console.error("[CaptureQuestSocket] WebSocket error:", e);
        if (!this.isConnected) {
          finish(false);
        }
      };

      ws.onclose = () => {
        finish(false);
        if(!owns())return;
        if (!this.isClosing) {
          console.log("[CaptureQuestSocket] WebSocket closed unexpectedly");
          this.close(true);
        }
      };

      ws.onmessage = (event: MessageEvent) => {
        if(!owns())return;
        const data = new Uint8Array(event.data as ArrayBuffer);
        // Append to buffer for length-prefixed frame parsing
        this.wsBuffer = concatUint8(this.wsBuffer, data);
        this.processWsBuffer();
      };
    });
  }

  private processWsBuffer() {
    // Parse length-prefixed frames: [length:uint32_LE][opcode:uint16_LE][payload]
    while (this.wsBuffer.length >= 4) {
      const len = new DataView(this.wsBuffer.buffer, this.wsBuffer.byteOffset, this.wsBuffer.byteLength).getUint32(0, true);
      if (this.wsBuffer.length < 4 + len) {
        break; // incomplete frame
      }
      const msg = this.wsBuffer.slice(4, 4 + len);
      const opcode = new DataView(msg.buffer, msg.byteOffset, msg.byteLength).getUint16(0, true) as OpCode;
      const payload = msg.slice(2);

      if (this.onJson) {
        try {
          const text = new TextDecoder().decode(payload);
          const json = JSON.parse(text);
          this.onJson(opcode, json);
        } catch {
          // Not JSON or failed to parse
        }
      }

      // Check pending requests (FIFO queue)
      const queue = this.pendingRequests.get(opcode);
      if (queue && queue.length > 0) {
        const pendingRequest = queue.shift()!;
        clearTimeout(pendingRequest.timeout);
        if (queue.length === 0) this.pendingRequests.delete(opcode);
        pendingRequest.resolve(payload);
      } else {
        try {
          this.opCodeHandlers[opcode]?.(payload);
        } catch (e) {
          console.error(`opCodeHandler[${opcode}] threw:`, e);
        }
      }

      this.wsBuffer = this.wsBuffer.slice(4 + len);
    }
  }

  private sendWsFrame(buf: Uint8Array) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return;
    }
    this.ws.send(buf);
  }

  // ——— private helpers ———

  private async sendDatagram(buf: Uint8Array) {
    // WebSocket fallback: wrap datagram as a length-prefixed frame
    if (this.useWebSocket) {
      const header = new ArrayBuffer(4);
      new DataView(header).setUint32(0, buf.byteLength, true);
      const frame = concatUint8(new Uint8Array(header), buf);
      this.sendWsFrame(frame);
      return;
    }

    if (!this.datagramWriter) {
      return;
    }
    const writer=this.datagramWriter,transport=this.webtransport;
    const write=this.writeQueue.then(()=>{
      if(this.datagramWriter!==writer || this.webtransport!==transport)throw new DOMException("Datagram owner retired","AbortError");
      return writer.write(buf);
    });
    // A failed/retired write rejects its caller, without poisoning later demand.
    this.writeQueue=write.catch(()=>{});
    return write;
  }

  private startDatagramLoop(transport:WebTransport) {
    const rdr = transport.datagrams.readable.getReader();
    this.nativeReaders.add(rdr);
    (async () => {
      try {
        while (true) {
          const { value, done } = await rdr.read();
          if (done || this.webtransport!==transport) {
            break;
          }
          if (!value) {
            continue;
          }
          const opcode = new Uint16Array(value.buffer.slice(0, 2))[0] as OpCode;
          const payload = value.slice(2);

          if (this.onJson) {
            try {
              const text = new TextDecoder().decode(payload);
              const json = JSON.parse(text);
              this.onJson(opcode, json);
            } catch {
              // Not JSON or failed to parse
            }
          }

          if(this.webtransport!==transport)break;
          this.opCodeHandlers[opcode]?.(payload);
        }
      } catch (e) {
        // Only log if this wasn't an intentional close
        if (this.webtransport===transport && !this.isClosing) {
          console.error("Datagram loop error:", e);
        }
      } finally {
        this.nativeReaders.delete(rdr);
        rdr.releaseLock();
      }
    })();
  }

  private attachControlStream(stream: WebTransportBidirectionalStream,transport:WebTransport) {
    this.controlWriter?.releaseLock();
    this.controlWriter = stream.writable.getWriter();
    this.startControlReadLoop(stream.readable,transport);
  }

  private startControlReadLoop(stream: ReadableStream<Uint8Array>,transport:WebTransport) {
    const rdr = stream.getReader();
    this.nativeReaders.add(rdr);
    let buffer: Uint8Array = new Uint8Array(0);
    (async () => {
      try {
        while (true) {
          const { value, done } = await rdr.read();
          if (done || this.webtransport!==transport) {
            break;
          }
          buffer = concatUint8(buffer, value!);
          while (buffer.length >= 4 && this.webtransport===transport) {
            const len = new DataView(buffer.buffer).getUint32(0, true);
            if (buffer.length < 4 + len) {
              break;
            }
            const msg = buffer.slice(4, 4 + len);
            const opcode = new Uint16Array(
              msg.buffer.slice(0, 2)
            )[0] as OpCode;
            const payload = msg.slice(2);

            if (this.onJson) {
              try {
                const text = new TextDecoder().decode(payload);
                const json = JSON.parse(text);
                this.onJson(opcode, json);
              } catch {
                // Not JSON or failed to parse
              }
            }

            if(this.webtransport!==transport)break;
            // Check if this is a response to a pending request (FIFO queue)
            const queue = this.pendingRequests.get(opcode);
            if (queue && queue.length > 0) {
              const pendingRequest = queue.shift()!; // Get first (oldest) request
              clearTimeout(pendingRequest.timeout);
              if (queue.length === 0) this.pendingRequests.delete(opcode);
              pendingRequest.resolve(payload);
            } else {
              // Otherwise, use the registered handler
              this.opCodeHandlers[opcode]?.(payload);
            }
            buffer = buffer.slice(4 + len);
          }
        }
      } catch (e) {
        // Only log if this wasn't an intentional close
        if (this.webtransport===transport && !this.isClosing) {
          console.error("Control stream loop error:", e);
        }
      } finally {
        this.nativeReaders.delete(rdr);
        rdr.releaseLock();
      }
    })();
  }

  private clearReconnectTimer() {
    if(this.reconnectTimer!==null){clearTimeout(this.reconnectTimer);this.reconnectTimer=null;}
  }

  private scheduleReconnect() {
    if(this.reconnectTimer!==null || this.isConnected || this.cancelWebSocketConnect!==null || this.nativeAttempt!==null)return;
    if (
      this.retryCount >= this.maxRetries ||
      !this.onClose
    ) {
      this.onClose?.();
      this.onDisconnect?.();
      this.retryCount = 0;
      return;
    }
    // For WebTransport mode, also require url/port
    if (!this.useWebSocket && (!this.url || !this.port)) {
      this.onClose?.();
      this.onDisconnect?.();
      this.retryCount = 0;
      return;
    }
    const delay = Math.min(2 ** this.retryCount * 1000, 30_000);
    this.retryCount++;
    this.reconnectTimer=setTimeout(async () => {
      this.reconnectTimer=null;
      // A user connection owns authentication. A queued retry must never
      // replace it with a second, unauthenticated transport.
      if(this.isConnected)return;
      let ok: boolean;
      if (this.useWebSocket) {
        const connecting=this.connectWebSocket(this.onClose!);
        const generation=this.webSocketAttemptGeneration;
        ok = await connecting;
        if(generation!==this.webSocketAttemptGeneration)return;
      } else {
        ok = await this.connect(this.url!, this.port!, this.onClose!);
      }
      if (!ok) {
        this.scheduleReconnect();
      }
    }, delay);
  }

  private startHeartbeat() {
    // Register a JSON handler for Heartbeat response to update latency
    this.registerJsonHandler<{ timestamp: number }>(
      OpCodes.Heartbeat,
      (payload) => {
        const now = performance.now();
        this.latency = Math.round(now - payload.timestamp);
        this.onPing?.(this.latency);
      }
    );

    // Heartbeat every 5 seconds is plenty for "keep-alive" while keeping UI responsive
    this.heartbeatInterval = setInterval(() => {
      this.ping();
    }, 5000);
  }

  private async ping() {
    if (!this.isConnected) return;
    const now = performance.now();
    await this.sendJsonMessage(OpCodes.Heartbeat, { timestamp: now });
  }
}
