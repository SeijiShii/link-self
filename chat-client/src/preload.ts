import { contextBridge, ipcRenderer } from "electron";

// Type definitions (inline to avoid import issues)
interface StartParams {
  listenAddrs?: string[];
  bootstrapPeers?: string[];
  identityPath?: string;
}

interface SendMessageParams {
  peerDID: string;
  message: string;
}

interface ConnectParams {
  peerDID: string;
}

const linkselfAPI = {
  start: (params: StartParams) => ipcRenderer.invoke("linkself:start", params),
  stop: () => ipcRenderer.invoke("linkself:stop"),
  getMyDID: () => ipcRenderer.invoke("linkself:getMyDID"),
  sendMessage: (params: SendMessageParams) =>
    ipcRenderer.invoke("linkself:sendMessage", params),
  connect: (params: ConnectParams) =>
    ipcRenderer.invoke("linkself:connect", params),
  onMessage: (callback: (peerDID: string, payload: string) => void) => {
    ipcRenderer.on(
      "linkself:message",
      (_event, peerDID: string, payload: string) => {
        callback(peerDID, payload);
      },
    );
  },
  dangerouslyDeleteAllData: () =>
    ipcRenderer.invoke("linkself:dangerouslyDeleteAllData"),
};

const contactsAPI = {
  get: () => ipcRenderer.invoke("contacts:get"),
  add: (contact: {
    did: string;
    name?: string;
    lastMessage?: string;
    lastMessageTime?: string;
  }) => ipcRenderer.invoke("contacts:add", contact),
};

const friendRequestsAPI = {
  get: () => ipcRenderer.invoke("friendRequests:get"),
  add: (req: { fromDID: string; receivedAt: number }) =>
    ipcRenderer.invoke("friendRequests:add", req),
  remove: (fromDID: string) =>
    ipcRenderer.invoke("friendRequests:remove", fromDID),
};

const appAPI = {
  getProfile: () => ipcRenderer.invoke("app:getProfile"),
};

interface MessageRow {
  id: string;
  peer_did: string;
  text: string;
  timestamp: number;
  is_sent: number;
  group_id: string | null;
}

const messagesAPI = {
  migrate: () => ipcRenderer.invoke("messages:migrate"),
  insert: (msg: {
    id: string;
    peerDID: string;
    text: string;
    timestamp: number;
    isSent: boolean;
    groupID?: string;
  }) => ipcRenderer.invoke("messages:insert", msg),
  getByPeer: (peerDID: string) =>
    ipcRenderer.invoke("messages:getByPeer", peerDID) as Promise<MessageRow[]>,
  getByGroup: (groupID: string) =>
    ipcRenderer.invoke("messages:getByGroup", groupID) as Promise<MessageRow[]>,
};

const groupsAPI = {
  save: (group: { groupID: string; name?: string; memberDIDs: string[] }) =>
    ipcRenderer.invoke("groups:save", group),
  getAll: () =>
    ipcRenderer.invoke("groups:getAll") as Promise<
      Array<{ groupID: string; name?: string; memberDIDs: string[] }>
    >,
  delete: (groupID: string) => ipcRenderer.invoke("groups:delete", groupID),
};

const networkAPI = {
  create: (memberDIDs: string[]) =>
    ipcRenderer.invoke("network:create", memberDIDs) as Promise<{
      groupID: string;
    }>,
  addMember: (groupID: string, memberDID: string) =>
    ipcRenderer.invoke("network:addMember", { groupID, memberDID }),
  leave: (groupID: string) => ipcRenderer.invoke("network:leave", groupID),
  list: () => ipcRenderer.invoke("network:list") as Promise<string[] | null>,
  get: (groupID: string) =>
    ipcRenderer.invoke("network:get", groupID) as Promise<{
      groupID: string;
      memberDIDs: string[];
    }>,
};

const pairingAPI = {
  createToken: (ttlSeconds: number) =>
    ipcRenderer.invoke("pairing:createToken", ttlSeconds) as Promise<{
      secret: string;
      ttlSeconds: number;
    }>,
  complete: (secret: string) =>
    ipcRenderer.invoke("pairing:complete", secret) as Promise<{
      userPrivKey: number[];
    }>,
};

const dataAPI = {
  export: () =>
    ipcRenderer.invoke("data:export") as Promise<{
      success: boolean;
      count?: number;
      path?: string;
      error?: string;
    }>,
  import: () =>
    ipcRenderer.invoke("data:import") as Promise<{
      success: boolean;
      applied?: number;
      error?: string;
    }>,
};

const devAPI = {
  generateTestDID: () =>
    ipcRenderer.invoke("dev:generateTestDID") as Promise<{ did: string }>,
  injectTestMessage: (fromDID: string, payload: string) =>
    ipcRenderer.invoke("dev:injectTestMessage", { fromDID, payload }),
};

contextBridge.exposeInMainWorld("linkself", linkselfAPI);
contextBridge.exposeInMainWorld("contacts", contactsAPI);
contextBridge.exposeInMainWorld("friendRequests", friendRequestsAPI);
contextBridge.exposeInMainWorld("app", appAPI);
contextBridge.exposeInMainWorld("messages", messagesAPI);
contextBridge.exposeInMainWorld("groups", groupsAPI);
contextBridge.exposeInMainWorld("network", networkAPI);
contextBridge.exposeInMainWorld("pairing", pairingAPI);
contextBridge.exposeInMainWorld("data", dataAPI);
contextBridge.exposeInMainWorld("dev", devAPI);
