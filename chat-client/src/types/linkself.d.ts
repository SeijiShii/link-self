// LinkSelf daemon JSON-RPC API type definitions

export interface JSONRPCRequest {
  jsonrpc: "2.0";
  method: string;
  params?: unknown;
  id: number | string;
}

export interface JSONRPCResponse {
  jsonrpc: "2.0";
  result?: unknown;
  error?: {
    code: number;
    message: string;
    data?: unknown;
  };
  id: number | string;
}

export interface JSONRPCNotification {
  jsonrpc: "2.0";
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
  method: "onMessage";
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
  dangerouslyDeleteAllData(): Promise<void>;
}

export interface ContactRecord {
  did: string;
  name?: string;
  lastMessage?: string;
  lastMessageTime?: string;
}

export interface FriendRequestRecord {
  fromDID: string;
  receivedAt: number;
}

export interface ContactsAPI {
  get(): Promise<ContactRecord[]>;
  add(contact: ContactRecord): Promise<ContactRecord[]>;
}

export interface FriendRequestsAPI {
  get(): Promise<FriendRequestRecord[]>;
  add(req: FriendRequestRecord): Promise<FriendRequestRecord[]>;
  remove(fromDID: string): Promise<FriendRequestRecord[]>;
}

export interface AppAPI {
  getProfile(): Promise<string>;
}

export interface MessageRow {
  id: string;
  peer_did: string;
  text: string;
  timestamp: number;
  is_sent: number;
  group_id: string | null;
}

export interface MessagesAPI {
  migrate(): Promise<void>;
  insert(msg: {
    id: string;
    peerDID: string;
    text: string;
    timestamp: number;
    isSent: boolean;
    groupID?: string;
  }): Promise<void>;
  getByPeer(peerDID: string): Promise<MessageRow[]>;
  getByGroup(groupID: string): Promise<MessageRow[]>;
}

export interface GroupsAppAPI {
  save(group: {
    groupID: string;
    name?: string;
    memberDIDs: string[];
  }): Promise<void>;
  getAll(): Promise<
    Array<{ groupID: string; name?: string; memberDIDs: string[] }>
  >;
  delete(groupID: string): Promise<void>;
}

export interface NetworkClientAPI {
  create(memberDIDs: string[]): Promise<{ groupID: string }>;
  addMember(groupID: string, memberDID: string): Promise<void>;
  leave(groupID: string): Promise<void>;
  list(): Promise<string[] | null>;
  get(groupID: string): Promise<{ groupID: string; memberDIDs: string[] }>;
}

export interface PairingAPI {
  createToken(ttlSeconds: number): Promise<{
    secret: string;
    ttlSeconds: number;
  }>;
  complete(secret: string): Promise<{ userPrivKey: number[] }>;
}

export interface DataAPI {
  export(): Promise<{
    success: boolean;
    count?: number;
    path?: string;
    error?: string;
  }>;
  import(): Promise<{
    success: boolean;
    applied?: number;
    error?: string;
  }>;
}

export interface DevAPI {
  generateTestDID(): Promise<{ did: string }>;
  injectTestMessage(fromDID: string, payload: string): Promise<void>;
}

declare global {
  interface Window {
    linkself: LinkSelfAPI;
    contacts: ContactsAPI;
    friendRequests: FriendRequestsAPI;
    app: AppAPI;
    messages: MessagesAPI;
    groups: GroupsAppAPI;
    network: NetworkClientAPI;
    pairing: PairingAPI;
    data: DataAPI;
    dev: DevAPI;
  }
}
