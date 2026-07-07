/**
 * Message router: dispatches incoming messages by envelope type.
 * Port of core/internal/node/router.go (Go).
 */
import { TYPE_DEVICE_SYNC, TYPE_GROUP_SHARE, TYPE_SUB_ANNOUNCE, unwrap } from "./envelope.js";

export type MessageHandler = (peerDID: string, payload: Uint8Array) => void;

export class MessageRouter {
  onDeviceSync?: MessageHandler;
  onGroupShare?: MessageHandler;
  onSubAnnounce?: MessageHandler;
  /** Plain / legacy messages (also the fallback for unknown types). */
  onMessage?: MessageHandler;

  /** Route a raw incoming message to the appropriate handler. */
  dispatch(peerDID: string, data: Uint8Array): void {
    const { type, payload } = unwrap(data);
    switch (type) {
      case TYPE_DEVICE_SYNC:
        this.onDeviceSync?.(peerDID, payload);
        break;
      case TYPE_GROUP_SHARE:
        this.onGroupShare?.(peerDID, payload);
        break;
      case TYPE_SUB_ANNOUNCE:
        this.onSubAnnounce?.(peerDID, payload);
        break;
      default:
        this.onMessage?.(peerDID, payload);
    }
  }
}
