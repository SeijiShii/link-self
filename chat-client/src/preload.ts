import { contextBridge, ipcRenderer } from 'electron';

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
  start: (params: StartParams) => ipcRenderer.invoke('linkself:start', params),
  stop: () => ipcRenderer.invoke('linkself:stop'),
  getMyDID: () => ipcRenderer.invoke('linkself:getMyDID'),
  sendMessage: (params: SendMessageParams) => ipcRenderer.invoke('linkself:sendMessage', params),
  connect: (params: ConnectParams) => ipcRenderer.invoke('linkself:connect', params),
  onMessage: (callback: (peerDID: string, payload: string) => void) => {
    ipcRenderer.on('linkself:message', (_event, peerDID: string, payload: string) => {
      callback(peerDID, payload);
    });
  },
};

const contactsAPI = {
  get: () => ipcRenderer.invoke('contacts:get'),
  add: (contact: { did: string; name?: string; lastMessage?: string; lastMessageTime?: string }) =>
    ipcRenderer.invoke('contacts:add', contact),
};

const friendRequestsAPI = {
  get: () => ipcRenderer.invoke('friendRequests:get'),
  add: (req: { fromDID: string; receivedAt: number }) => ipcRenderer.invoke('friendRequests:add', req),
  remove: (fromDID: string) => ipcRenderer.invoke('friendRequests:remove', fromDID),
};

const appAPI = {
  getProfile: () => ipcRenderer.invoke('app:getProfile'),
};

contextBridge.exposeInMainWorld('linkself', linkselfAPI);
contextBridge.exposeInMainWorld('contacts', contactsAPI);
contextBridge.exposeInMainWorld('friendRequests', friendRequestsAPI);
contextBridge.exposeInMainWorld('app', appAPI);
