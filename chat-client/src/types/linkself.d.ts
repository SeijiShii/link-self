// LinkSelf daemon JSON-RPC API type definitions

export interface JSONRPCRequest {
  jsonrpc: '2.0';
  method: string;
  params?: unknown;
  id: number | string;
}

export interface JSONRPCResponse {
  jsonrpc: '2.0';
  result?: unknown;
  error?: {
    code: number;
    message: string;
    data?: unknown;
  };
  id: number | string;
}

export interface JSONRPCNotification {
  jsonrpc: '2.0';
  method: string;
  params?: unknown;
}

// LinkSelf daemon methods
export interface StartParams {
  listenAddrs?: string[];
  bootstrapPeers?: string[];
  identityPath?: string;
}

export interface StartResult {
  did: string;
  listenAddr: string;
}

export interface SendMessageParams {
  peerDID: string;
  message: string;
}

export interface ConnectParams {
  peerDID: string;
}

export interface MessageNotification {
  method: 'onMessage';
  params: {
    peerDID: string;
    payload: string;
  };
}

// IPC API exposed to renderer
export interface LinkSelfAPI {
  start(params: StartParams): Promise<StartResult>;
  stop(): Promise<void>;
  getMyDID(): Promise<string>;
  sendMessage(params: SendMessageParams): Promise<void>;
  connect(params: ConnectParams): Promise<void>;
  onMessage(callback: (peerDID: string, payload: string) => void): void;
}

declare global {
  interface Window {
    linkself: LinkSelfAPI;
  }
}
