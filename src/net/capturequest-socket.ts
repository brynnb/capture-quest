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
  public latency = 0;
  public onPing: ((latency: number) => void) | null = null;
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
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const WT = (window as any).WebTransport as {
      new(url: string, opts?: WebTransportOptions): WebTransport;
    };

    console.log(`[CaptureQuestSocket] Environment check: SecureContext=${window.isSecureContext}, WebTransportSupport=${!!WT}, Host=${window.location.hostname}`);

    if (FORCE_WEBSOCKET) {
      console.log("[CaptureQuestSocket] VITE_FORCE_WEBSOCKET=true, using WebSocket transport");
      return this.connectWebSocket(onClose);
    }

    if (!WT) {
      console.warn("[CaptureQuestSocket] WebTransport not supported, falling back to WebSocket");
      return this.connectWebSocket(onClose);
    }

    this.url = url;
    this.port = port;
    this.onClose = onClose;

    // if already open, shut it down first
    if (this.webtransport) {
      const closedInfo = await this.webtransport.closed.catch(() => null);
      if (!closedInfo) {
        this.close(false);
      }
    }

    try {
      // We always fetch the certificate hash from the server because our backend
      // generates a dynamic self-signed certificate for WebTransport (UDP).
      // The hash is securely fetched over HTTPS (Fly.io/Let's Encrypt).
      console.log(`[CaptureQuestSocket] Fetching certificate hash from ${getApiUrl("/hash")}...`);
      const controller = new AbortController();
      const fetchTimeout = setTimeout(() => controller.abort(), 5000);

      const hash = await fetch(getApiUrl("/hash"), { signal: controller.signal }).then(
        (r: Response) => r.text()
      );
      clearTimeout(fetchTimeout);
      console.log(`[CaptureQuestSocket] Certificate hash received: ${hash} `);

      const transportUrl = `https://${url}:${port}/cq`;
      console.log(`[CaptureQuestSocket] Initiating WebTransport connection to ${transportUrl}...`);
      this.webtransport = new WebTransport(transportUrl, {
        serverCertificateHashes: [
          { algorithm: "sha-256", value: base64ToArrayBuffer(hash) },
        ],
      });

      // wait for handshake
      console.log(`[CaptureQuestSocket] Waiting for WebTransport handshake...`);
      await this.webtransport.ready;
      console.log(`[CaptureQuestSocket] WebTransport connection established.`);

      // ——— datagram writer & loop ———
      this.datagramWriter = this.webtransport.datagrams.writable.getWriter();
      this.startDatagramLoop();

      const controlStream = await this.webtransport.createBidirectionalStream();
      this.attachControlStream(controlStream);
      console.log("[CaptureQuestSocket] Control stream ready");

      this.isConnected = true;
      this.isClosing = false;
      this.retryCount = 0;

      // Start heartbeat (registers handler + interval)
      this.startHeartbeat();

      // watch for close - don't auto-reconnect on normal close
      this.webtransport.closed
        .then((info) => {
          console.log("WebTransport closed:", info);
          this.close(false);
        })
        .catch((e) => {
          console.error("WebTransport closed with error:", e);
          this.close(false);
        });

      return true;
    } catch (e) {
      console.warn("Connect failed:", e);
      this.scheduleReconnect();
      return false;
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



  public registerJsonHandler(
    opCode: OpCode,
    handler: (msg: any) => void
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
  public async sendJsonRequest<TRes = any>(
    requestOpCode: OpCode,
    responseOpCode: OpCode,
    data: unknown,
    timeoutMs: number = 10000
  ): Promise<TRes> {
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
    this.isClosing = true;
    this.isConnected = false;

    // Clean up WebTransport
    this.datagramWriter?.releaseLock();
    this.controlWriter?.releaseLock();
    this.webtransport?.close();
    this.webtransport = null;
    this.datagramWriter = null;
    this.controlWriter = null;

    // Clean up WebSocket
    if (this.ws) {
      this.ws.onclose = null; // prevent re-entrant close handler
      this.ws.onerror = null;
      this.ws.onmessage = null;
      this.ws.close();
      this.ws = null;
    }
    this.wsBuffer = new Uint8Array(0);

    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }

    if (scheduleReconnect && this.allowReconnect) {
      this.scheduleReconnect();
    } else {
      this.onClose?.();
      this.onDisconnect?.();
    }
  }

  // ——— WebSocket fallback ———

  private async connectWebSocket(onClose: () => void): Promise<boolean> {
    this.onClose = onClose;
    this.useWebSocket = true;

    return new Promise<boolean>((resolve) => {
      const wsUrl = getWsUrl("/ws");
      console.log(`[CaptureQuestSocket] Connecting via WebSocket to ${wsUrl}...`);
      const ws = new WebSocket(wsUrl);
      ws.binaryType = "arraybuffer";

      ws.onopen = () => {
        console.log("[CaptureQuestSocket] WebSocket connection established");
        this.ws = ws;
        this.isConnected = true;
        this.isClosing = false;
        this.retryCount = 0;
        this.startHeartbeat();
        resolve(true);
      };

      ws.onerror = (e) => {
        console.error("[CaptureQuestSocket] WebSocket error:", e);
        if (!this.isConnected) {
          resolve(false);
        }
      };

      ws.onclose = () => {
        if (!this.isClosing) {
          console.log("[CaptureQuestSocket] WebSocket closed unexpectedly");
          this.close(true);
        }
      };

      ws.onmessage = (event: MessageEvent) => {
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
    this.writeQueue = this.writeQueue.then(() =>
      this.datagramWriter!.write(buf)
    );
    return this.writeQueue;
  }

  private startDatagramLoop() {
    if (!this.webtransport) {
      return;
    }
    const rdr = this.webtransport.datagrams.readable.getReader();
    (async () => {
      try {
        while (true) {
          const { value, done } = await rdr.read();
          if (done) {
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
            } catch (e) {
              // Not JSON or failed to parse
            }
          }

          this.opCodeHandlers[opcode]?.(payload);
        }
      } catch (e) {
        // Only log if this wasn't an intentional close
        if (!this.isClosing) {
          console.error("Datagram loop error:", e);
        }
      } finally {
        rdr.releaseLock();
      }
    })();
  }

  private attachControlStream(stream: WebTransportBidirectionalStream) {
    this.controlWriter?.releaseLock();
    this.controlWriter = stream.writable.getWriter();
    this.startControlReadLoop(stream.readable);
  }

  private startControlReadLoop(stream: ReadableStream<Uint8Array>) {
    const rdr = stream.getReader();
    let buffer: Uint8Array = new Uint8Array(0);
    (async () => {
      try {
        while (true) {
          const { value, done } = await rdr.read();
          if (done) {
            break;
          }
          buffer = concatUint8(buffer, value!);
          while (buffer.length >= 4) {
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
              } catch (e) {
                // Not JSON or failed to parse
              }
            }

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
        if (!this.isClosing) {
          console.error("Control stream loop error:", e);
        }
      } finally {
        rdr.releaseLock();
      }
    })();
  }

  private scheduleReconnect() {
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
    setTimeout(async () => {
      let ok: boolean;
      if (this.useWebSocket) {
        ok = await this.connectWebSocket(this.onClose!);
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
    this.registerJsonHandler(
      OpCodes.Heartbeat,
      (payload: any) => {
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
