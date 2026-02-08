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

contextBridge.exposeInMainWorld('linkself', linkselfAPI);
