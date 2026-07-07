/**
 * LinkSelf wire framing: uint32 big-endian length prefix + payload.
 * Matches the Go side (encoding/binary BigEndian + raw bytes).
 */

/** Maximum accepted payload size (matches the Go node's 1 MiB guard). */
export const MAX_FRAME_SIZE = 1 << 20;

/** Minimal duplex stream: what we need from a @libp2p/interface Stream. */
export interface DuplexStream extends AsyncIterable<Uint8Array | { subarray(): Uint8Array }> {
  send(data: Uint8Array): boolean;
  close(): Promise<void>;
}

/** Encode a payload as a length-prefixed frame. */
export function encodeFrame(payload: Uint8Array): Uint8Array {
  const frame = new Uint8Array(4 + payload.length);
  new DataView(frame.buffer).setUint32(0, payload.length, false);
  frame.set(payload, 4);
  return frame;
}

/**
 * Buffered reader over a stream's async iterator, for reading exact byte
 * counts regardless of how the transport chunks the data.
 */
export class StreamReader {
  private readonly iterator: AsyncIterator<Uint8Array | { subarray(): Uint8Array }>;
  private buffer: Uint8Array = new Uint8Array(0);

  constructor(stream: AsyncIterable<Uint8Array | { subarray(): Uint8Array }>) {
    this.iterator = stream[Symbol.asyncIterator]();
  }

  /** Read exactly n bytes; throws if the stream ends first. */
  async read(n: number): Promise<Uint8Array> {
    while (this.buffer.length < n) {
      const { done, value } = await this.iterator.next();
      if (done === true) {
        throw new Error(`stream ended: wanted ${n} bytes, got ${this.buffer.length}`);
      }
      const chunk = value instanceof Uint8Array ? value : value.subarray();
      const merged = new Uint8Array(this.buffer.length + chunk.length);
      merged.set(this.buffer, 0);
      merged.set(chunk, this.buffer.length);
      this.buffer = merged;
    }
    const out = this.buffer.subarray(0, n);
    this.buffer = this.buffer.subarray(n);
    return out;
  }

  /** Read a big-endian uint32. */
  async readUint32(): Promise<number> {
    const b = await this.read(4);
    return new DataView(b.buffer, b.byteOffset, 4).getUint32(0, false);
  }

  /** Read one length-prefixed frame. */
  async readFrame(): Promise<Uint8Array> {
    const len = await this.readUint32();
    if (len > MAX_FRAME_SIZE) {
      throw new Error(`frame too large: ${len} bytes`);
    }
    return await this.read(len);
  }
}
