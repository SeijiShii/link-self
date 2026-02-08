import { contextBridge, ipcRenderer } from 'electron';
const linkselfAPI = {
    start: (params) => ipcRenderer.invoke('linkself:start', params),
    stop: () => ipcRenderer.invoke('linkself:stop'),
    getMyDID: () => ipcRenderer.invoke('linkself:getMyDID'),
    sendMessage: (params) => ipcRenderer.invoke('linkself:sendMessage', params),
    connect: (params) => ipcRenderer.invoke('linkself:connect', params),
    onMessage: (callback) => {
        ipcRenderer.on('linkself:message', (_event, peerDID, payload) => {
            callback(peerDID, payload);
        });
    },
};
contextBridge.exposeInMainWorld('linkself', linkselfAPI);
