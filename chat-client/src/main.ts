import { app, BrowserWindow, ipcMain } from 'electron';
import { spawn, ChildProcess } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import type { JSONRPCRequest, JSONRPCResponse, JSONRPCNotification, StartParams, StartResult, SendMessageParams, ConnectParams } from './types/linkself';

// Disable hardware acceleration for Linux compatibility
app.disableHardwareAcceleration();

let mainWindow: BrowserWindow | null = null;
let daemonProcess: ChildProcess | null = null;
let requestIdCounter = 0;
const pendingRequests = new Map<number | string, { resolve: (value: unknown) => void; reject: (error: Error) => void }>();

function createWindow() {
  // Preload script is compiled to dist/src/preload.js
  // __dirname is dist/src when running from dist/src/main.js
  const preloadPath = path.join(__dirname, 'preload.js');

  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      preload: preloadPath,
      nodeIntegration: false,
      contextIsolation: true,
      webSecurity: true,
    },
    show: false, // Don't show until ready
  });

  // Show window when ready to prevent white screen
  mainWindow.once('ready-to-show', () => {
    mainWindow?.show();
  });

  // Load the React app
  const isDev = process.env.NODE_ENV === 'development' || !app.isPackaged;
  if (isDev) {
    const devUrl = 'http://localhost:5173';
    console.log('Loading dev URL:', devUrl);
    
    // Wait a bit for Vite dev server to be ready
    setTimeout(() => {
      mainWindow?.loadURL(devUrl).catch((err) => {
        console.error('Failed to load URL:', err);
        // Retry after a delay
        setTimeout(() => {
          console.log('Retrying to load URL...');
          mainWindow?.loadURL(devUrl).catch((retryErr) => {
            console.error('Retry failed:', retryErr);
          });
        }, 2000);
      });
      mainWindow?.webContents.openDevTools();
    }, 1000);
  } else {
    mainWindow.loadFile(path.join(__dirname, '../dist/renderer/index.html'));
  }

  // Debug: Log when page loads
  mainWindow.webContents.on('did-finish-load', () => {
    console.log('Page finished loading successfully');
  });

  mainWindow.webContents.on('did-fail-load', (event, errorCode, errorDescription, validatedURL) => {
    console.error('Failed to load page:', {
      errorCode,
      errorDescription,
      url: validatedURL,
    });
  });

  mainWindow.webContents.on('dom-ready', () => {
    console.log('DOM ready');
  });

  mainWindow.webContents.on('console-message', (event, level, message) => {
    console.log(`[Renderer ${level}]:`, message);
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function startDaemon(): Promise<void> {
  return new Promise((resolve, reject) => {
    // Try multiple possible paths for the daemon
    const possiblePaths = [
      path.join(__dirname, '../build/linkself-daemon.exe'),
      path.join(__dirname, '../../build/linkself-daemon.exe'),
      path.join(process.cwd(), 'build/linkself-daemon.exe'),
    ];

    let daemonPath: string | null = null;
    for (const possiblePath of possiblePaths) {
      if (fs.existsSync(possiblePath)) {
        daemonPath = possiblePath;
        break;
      }
    }

    if (!daemonPath) {
      reject(new Error(`Daemon not found. Please build it first with 'npm run build:daemon'. Tried: ${possiblePaths.join(', ')}`));
      return;
    }

    daemonProcess = spawn(daemonPath, [], {
      stdio: ['pipe', 'pipe', 'pipe'],
    });

    daemonProcess.stdout?.on('data', (data: Buffer) => {
      const lines = data.toString().split('\n').filter(line => line.trim());
      for (const line of lines) {
        try {
          const response = JSON.parse(line) as JSONRPCResponse | JSONRPCNotification;
          
          if ('id' in response && response.id !== undefined) {
            // It's a response
            const pending = pendingRequests.get(response.id);
            if (pending) {
              pendingRequests.delete(response.id);
              if (response.error) {
                pending.reject(new Error(response.error.message));
              } else {
                pending.resolve(response.result);
              }
            }
          } else if ('method' in response) {
            // It's a notification
            if (response.method === 'onMessage' && 'params' in response) {
              const params = response.params as { peerDID: string; payload: string };
              mainWindow?.webContents.send('linkself:message', params.peerDID, params.payload);
            }
          }
        } catch (error) {
          console.error('Failed to parse daemon output:', error, line);
        }
      }
    });

    daemonProcess.stderr?.on('data', (data: Buffer) => {
      console.error('Daemon stderr:', data.toString());
    });

    daemonProcess.on('error', (error) => {
      console.error('Daemon process error:', error);
      reject(error);
    });

    daemonProcess.on('exit', (code) => {
      console.log(`Daemon process exited with code ${code}`);
      daemonProcess = null;
    });

    // Wait a bit for daemon to start
    setTimeout(() => resolve(), 1000);
  });
}

function stopDaemon(): Promise<void> {
  return new Promise((resolve) => {
    if (daemonProcess) {
      daemonProcess.kill();
      daemonProcess = null;
    }
    resolve();
  });
}

function sendRequest(method: string, params?: unknown): Promise<unknown> {
  return new Promise((resolve, reject) => {
    if (!daemonProcess || !daemonProcess.stdin) {
      reject(new Error('Daemon process not running'));
      return;
    }

    const id = ++requestIdCounter;
    const request: JSONRPCRequest = {
      jsonrpc: '2.0',
      method,
      params,
      id,
    };

    pendingRequests.set(id, { resolve, reject });

    daemonProcess.stdin.write(JSON.stringify(request) + '\n');
  });
}

app.whenReady().then(() => {
  createWindow();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on('window-all-closed', async () => {
  await stopDaemon();
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('before-quit', async () => {
  await stopDaemon();
});

// IPC handlers
ipcMain.handle('linkself:start', async (_event, params: StartParams) => {
  try {
    await startDaemon();
    const result = await sendRequest('start', params) as StartResult;
    return result;
  } catch (error) {
    console.error('Failed to start daemon:', error);
    throw error;
  }
});

ipcMain.handle('linkself:stop', async () => {
  await stopDaemon();
});

ipcMain.handle('linkself:getMyDID', async () => {
  return await sendRequest('getMyDID');
});

ipcMain.handle('linkself:sendMessage', async (_event, params: SendMessageParams) => {
  return await sendRequest('sendMessage', params);
});

ipcMain.handle('linkself:connect', async (_event, params: ConnectParams) => {
  return await sendRequest('connect', params);
});
