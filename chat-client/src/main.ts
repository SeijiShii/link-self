import { app, BrowserWindow, ipcMain, dialog } from "electron";
import { spawn, ChildProcess } from "child_process";
import * as path from "path";
import * as fs from "fs";
import type {
  JSONRPCRequest,
  JSONRPCResponse,
  JSONRPCNotification,
  StartParams,
  StartResult,
  SendMessageParams,
  ConnectParams,
} from "./types/linkself";

// Disable hardware acceleration for Linux compatibility
app.disableHardwareAcceleration();

// User data paths (per-instance when using --user-data-dir)
function getUserDataPaths() {
  const userData = app.getPath("userData");
  const identityPath = path.resolve(userData, "identity.json");
  return { userData, identityPath };
}

// どのプロファイル（userA / userB / default）で動いているか。2インスタンス時に別DIDか確認する目安。
function getAppProfile(): string {
  const userData = app.getPath("userData");
  const normalized = path.normalize(userData);
  if (normalized.includes("userA")) return "userA";
  if (normalized.includes("userB")) return "userB";
  return "default";
}

// Contact shape for persistence (lastMessageTime as ISO string)
interface ContactRecord {
  did: string;
  name?: string;
  lastMessage?: string;
  lastMessageTime?: string;
}

interface FriendRequestRecord {
  fromDID: string;
  receivedAt: number;
}

let mainWindow: BrowserWindow | null = null;
let daemonProcess: ChildProcess | null = null;
let requestIdCounter = 0;
const pendingRequests = new Map<
  number | string,
  { resolve: (value: unknown) => void; reject: (error: Error) => void }
>();

function createWindow() {
  // Preload script is compiled to dist/src/preload.js
  // __dirname is dist/src when running from dist/src/main.js
  const preloadPath = path.join(__dirname, "preload.js");

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
  mainWindow.once("ready-to-show", () => {
    mainWindow?.show();
  });

  // Load the React app
  const isDev = process.env.NODE_ENV === "development" || !app.isPackaged;
  if (isDev) {
    const devUrl = "http://localhost:5173";
    console.log("Loading dev URL:", devUrl);

    // Wait a bit for Vite dev server to be ready
    setTimeout(() => {
      mainWindow?.loadURL(devUrl).catch((err) => {
        console.error("Failed to load URL:", err);
        // Retry after a delay
        setTimeout(() => {
          console.log("Retrying to load URL...");
          mainWindow?.loadURL(devUrl).catch((retryErr) => {
            console.error("Retry failed:", retryErr);
          });
        }, 2000);
      });
      mainWindow?.webContents.openDevTools();
    }, 1000);
  } else {
    mainWindow.loadFile(path.join(__dirname, "../dist/renderer/index.html"));
  }

  // Debug: Log when page loads
  mainWindow.webContents.on("did-finish-load", () => {
    console.log("Page finished loading successfully");
  });

  mainWindow.webContents.on(
    "did-fail-load",
    (event, errorCode, errorDescription, validatedURL) => {
      console.error("Failed to load page:", {
        errorCode,
        errorDescription,
        url: validatedURL,
      });
      // electron:userB 単体起動時など、Vite が動いていないと白画面になる。案内を表示する。
      const isDev = process.env.NODE_ENV === "development" || !app.isPackaged;
      if (
        isDev &&
        validatedURL &&
        validatedURL.startsWith("http://localhost:5173")
      ) {
        const msg = [
          "<h2>Vite が起動していません</h2>",
          "<p>electron:userB は、Vite が <code>http://localhost:5173</code> で動いている必要があります。</p>",
          "<p><strong>手順:</strong></p>",
          "<ol><li>ターミナル 1 で <code>npm run dev:userA</code> を先に実行する</li>",
          "<li>Vite が起動したら、ターミナル 2 で <code>npm run electron:userB</code> を実行する</li></ol>",
        ].join("");
        mainWindow?.loadURL(
          "data:text/html;charset=utf-8," +
            encodeURIComponent(
              '<!DOCTYPE html><html><head><meta charset="utf-8"></head><body style="font-family:sans-serif;padding:2rem;max-width:480px;">' +
                msg +
                "</body></html>",
            ),
        );
      }
    },
  );

  mainWindow.webContents.on("dom-ready", () => {
    console.log("DOM ready");
  });

  mainWindow.webContents.on("console-message", (event, level, message) => {
    console.log(`[Renderer ${level}]:`, message);
  });

  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

function startDaemon(): Promise<void> {
  return new Promise((resolve, reject) => {
    // Try multiple possible paths for the daemon
    const possiblePaths = [
      path.join(__dirname, "../build/linkself-daemon.exe"),
      path.join(__dirname, "../../build/linkself-daemon.exe"),
      path.join(process.cwd(), "build/linkself-daemon.exe"),
    ];

    let daemonPath: string | null = null;
    for (const possiblePath of possiblePaths) {
      if (fs.existsSync(possiblePath)) {
        daemonPath = possiblePath;
        break;
      }
    }

    if (!daemonPath) {
      reject(
        new Error(
          `Daemon not found. Please build it first with 'npm run build:daemon'. Tried: ${possiblePaths.join(", ")}`,
        ),
      );
      return;
    }

    daemonProcess = spawn(daemonPath, [], {
      stdio: ["pipe", "pipe", "pipe"],
    });

    daemonProcess.stdout?.on("data", (data: Buffer) => {
      const lines = data
        .toString()
        .split("\n")
        .filter((line) => line.trim());
      for (const line of lines) {
        try {
          const response = JSON.parse(line) as
            | JSONRPCResponse
            | JSONRPCNotification;

          if ("id" in response && response.id !== undefined) {
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
          } else if ("method" in response) {
            // It's a notification
            if (response.method === "onMessage" && "params" in response) {
              const params = response.params as {
                peerDID: string;
                payload: string;
              };
              mainWindow?.webContents.send(
                "linkself:message",
                params.peerDID,
                params.payload,
              );
            }
          }
        } catch (error) {
          console.error("Failed to parse daemon output:", error, line);
        }
      }
    });

    daemonProcess.stderr?.on("data", (data: Buffer) => {
      console.error("Daemon stderr:", data.toString());
    });

    daemonProcess.on("error", (error) => {
      console.error("Daemon process error:", error);
      reject(error);
    });

    daemonProcess.on("exit", (code) => {
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
      reject(new Error("Daemon process not running"));
      return;
    }

    const id = ++requestIdCounter;
    const request: JSONRPCRequest = {
      jsonrpc: "2.0",
      method,
      params,
      id,
    };

    pendingRequests.set(id, { resolve, reject });

    daemonProcess.stdin.write(JSON.stringify(request) + "\n");
  });
}

app.whenReady().then(() => {
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", async () => {
  await stopDaemon();
  if (process.platform !== "darwin") {
    app.quit();
  }
});

app.on("before-quit", async () => {
  await stopDaemon();
});

// IPC handlers: contacts (via MyDB SQL)
ipcMain.handle("contacts:get", async () => {
  try {
    const rows = (await sendRequest("mydb.query", {
      sql: `SELECT did, name, last_message, last_message_time FROM contacts ORDER BY last_message_time DESC`,
    })) as Array<{
      did: string;
      name: string | null;
      last_message: string | null;
      last_message_time: string | null;
    }> | null;
    if (!rows) return [];
    return rows.map((r) => ({
      did: r.did,
      name: r.name ?? undefined,
      lastMessage: r.last_message ?? undefined,
      lastMessageTime: r.last_message_time ?? undefined,
    }));
  } catch {
    return [];
  }
});

ipcMain.handle("contacts:add", async (_event, contact: ContactRecord) => {
  await sendRequest("mydb.exec", {
    sql: `INSERT INTO contacts (did, name, last_message, last_message_time) VALUES (?, ?, ?, ?)
          ON CONFLICT(did) DO UPDATE SET
            name = COALESCE(excluded.name, contacts.name),
            last_message = COALESCE(excluded.last_message, contacts.last_message),
            last_message_time = COALESCE(excluded.last_message_time, contacts.last_message_time)`,
    args: [
      contact.did,
      contact.name ?? null,
      contact.lastMessage ?? null,
      contact.lastMessageTime ?? null,
    ],
  });
  const rows = (await sendRequest("mydb.query", {
    sql: `SELECT did, name, last_message, last_message_time FROM contacts ORDER BY last_message_time DESC`,
  })) as Array<{
    did: string;
    name: string | null;
    last_message: string | null;
    last_message_time: string | null;
  }> | null;
  if (!rows) return [contact];
  return rows.map((r) => ({
    did: r.did,
    name: r.name ?? undefined,
    lastMessage: r.last_message ?? undefined,
    lastMessageTime: r.last_message_time ?? undefined,
  }));
});

// IPC handlers: friend requests (via MyDB SQL)
ipcMain.handle("friendRequests:get", async () => {
  try {
    const rows = (await sendRequest("mydb.query", {
      sql: `SELECT from_did, received_at FROM friend_requests ORDER BY received_at DESC`,
    })) as Array<{ from_did: string; received_at: number }> | null;
    if (!rows) return [];
    return rows.map((r) => ({
      fromDID: r.from_did,
      receivedAt: r.received_at,
    }));
  } catch {
    return [];
  }
});

ipcMain.handle(
  "friendRequests:add",
  async (_event, req: FriendRequestRecord) => {
    await sendRequest("mydb.exec", {
      sql: `INSERT OR IGNORE INTO friend_requests (from_did, received_at) VALUES (?, ?)`,
      args: [req.fromDID, req.receivedAt],
    });
    const rows = (await sendRequest("mydb.query", {
      sql: `SELECT from_did, received_at FROM friend_requests ORDER BY received_at DESC`,
    })) as Array<{ from_did: string; received_at: number }> | null;
    return rows
      ? rows.map((r) => ({ fromDID: r.from_did, receivedAt: r.received_at }))
      : [req];
  },
);

ipcMain.handle("friendRequests:remove", async (_event, fromDID: string) => {
  await sendRequest("mydb.exec", {
    sql: `DELETE FROM friend_requests WHERE from_did = ?`,
    args: [fromDID],
  });
  const rows = (await sendRequest("mydb.query", {
    sql: `SELECT from_did, received_at FROM friend_requests ORDER BY received_at DESC`,
  })) as Array<{ from_did: string; received_at: number }> | null;
  return rows
    ? rows.map((r) => ({ fromDID: r.from_did, receivedAt: r.received_at }))
    : [];
});

// IPC handlers: app (profile for multi-instance)
ipcMain.handle("app:getProfile", () => getAppProfile());

// 公開DHT用のデフォルト bootstrap（libp2p の bootstrap.libp2p.io）。検証環境・実運用とも同じDHTに参加する。
const DEFAULT_PUBLIC_DHT_BOOTSTRAP_PEERS = [
  "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
  "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
  "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
  "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
];

// IPC handlers: linkself (daemon)
ipcMain.handle("linkself:start", async (_event, params: StartParams) => {
  try {
    await startDaemon();
    const { identityPath } = getUserDataPaths();
    const mergedParams: StartParams = { ...params, identityPath };
    // bootstrap: 環境変数で上書きしなければ公開DHTのデフォルトを使用（検証・実運用とも同じDHT）
    const bootstrapPeer = process.env.BOOTSTRAP_PEER;
    const bootstrapPeersEnv = process.env.BOOTSTRAP_PEERS;
    const disablePublicDHT =
      process.env.DISABLE_PUBLIC_DHT === "1" ||
      process.env.DISABLE_PUBLIC_DHT === "true";
    if (bootstrapPeer) {
      mergedParams.bootstrapPeers = [bootstrapPeer.trim()];
    } else if (bootstrapPeersEnv) {
      mergedParams.bootstrapPeers = bootstrapPeersEnv
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
    } else if (!disablePublicDHT) {
      mergedParams.bootstrapPeers = [...DEFAULT_PUBLIC_DHT_BOOTSTRAP_PEERS];
    }
    console.log(
      "[LinkSelf] profile:",
      getAppProfile(),
      "identityPath:",
      identityPath,
      "bootstrapPeers:",
      mergedParams.bootstrapPeers?.length ?? 0,
    );
    const result = (await sendRequest("start", mergedParams)) as StartResult;
    return result;
  } catch (error) {
    console.error("Failed to start daemon:", error);
    throw error;
  }
});

ipcMain.handle("linkself:stop", async () => {
  await stopDaemon();
});

ipcMain.handle("linkself:getMyDID", async () => {
  return await sendRequest("getMyDID");
});

ipcMain.handle(
  "linkself:sendMessage",
  async (_event, params: SendMessageParams) => {
    return await sendRequest("sendMessage", params);
  },
);

ipcMain.handle("linkself:connect", async (_event, params: ConnectParams) => {
  return await sendRequest("connect", params);
});

ipcMain.handle("linkself:dangerouslyDeleteAllData", async () => {
  await sendRequest("dangerouslyDeleteAllData");
});

// IPC handlers: pairing
ipcMain.handle("pairing:createToken", async (_event, ttlSeconds: number) => {
  return await sendRequest("createPairingToken", {
    ttlSeconds: ttlSeconds || 300,
  });
});

ipcMain.handle("pairing:complete", async (_event, secret: string) => {
  return await sendRequest("completePairing", { secret });
});

// IPC handlers: network (group management)
ipcMain.handle("network:create", async (_event, memberDIDs: string[]) => {
  return await sendRequest("network.create", { memberDIDs });
});

ipcMain.handle(
  "network:addMember",
  async (_event, params: { groupID: string; memberDID: string }) => {
    return await sendRequest("network.addMember", params);
  },
);

ipcMain.handle("network:leave", async (_event, groupID: string) => {
  return await sendRequest("network.leave", { groupID });
});

ipcMain.handle("network:list", async () => {
  return await sendRequest("network.list", {});
});

ipcMain.handle("network:get", async (_event, groupID: string) => {
  return await sendRequest("network.get", { groupID });
});

// IPC handlers: dev (test data injection)
ipcMain.handle("dev:generateTestDID", async () => {
  return await sendRequest("generateTestDID");
});

ipcMain.handle(
  "dev:injectTestMessage",
  async (_event, params: { fromDID: string; payload: string }) => {
    return await sendRequest("injectTestMessage", params);
  },
);

// IPC handlers: messages (SQL-based persistence)
ipcMain.handle("messages:migrate", async () => {
  await sendRequest("mydb.migrate", {
    migrations: [
      {
        version: 1,
        sql: `CREATE TABLE IF NOT EXISTS messages (
          id TEXT PRIMARY KEY,
          peer_did TEXT NOT NULL,
          text TEXT NOT NULL,
          timestamp INTEGER NOT NULL,
          is_sent INTEGER NOT NULL
        )`,
      },
      {
        version: 2,
        sql: `CREATE TABLE IF NOT EXISTS contacts (
          did TEXT PRIMARY KEY,
          name TEXT,
          last_message TEXT,
          last_message_time TEXT
        );
        CREATE TABLE IF NOT EXISTS friend_requests (
          from_did TEXT PRIMARY KEY,
          received_at INTEGER NOT NULL
        )`,
      },
      {
        version: 3,
        sql: `CREATE TABLE IF NOT EXISTS groups (
          group_id TEXT PRIMARY KEY,
          name TEXT
        );
        CREATE TABLE IF NOT EXISTS group_members (
          group_id TEXT NOT NULL,
          member_did TEXT NOT NULL,
          PRIMARY KEY (group_id, member_did)
        );
        ALTER TABLE messages ADD COLUMN group_id TEXT`,
      },
    ],
  });
});

ipcMain.handle(
  "messages:insert",
  async (
    _event,
    msg: {
      id: string;
      peerDID: string;
      text: string;
      timestamp: number;
      isSent: boolean;
      groupID?: string;
    },
  ) => {
    await sendRequest("mydb.exec", {
      sql: `INSERT OR IGNORE INTO messages (id, peer_did, text, timestamp, is_sent, group_id) VALUES (?, ?, ?, ?, ?, ?)`,
      args: [
        msg.id,
        msg.peerDID,
        msg.text,
        msg.timestamp,
        msg.isSent ? 1 : 0,
        msg.groupID ?? null,
      ],
    });
  },
);

ipcMain.handle("messages:getByPeer", async (_event, peerDID: string) => {
  return await sendRequest("mydb.query", {
    sql: `SELECT id, peer_did, text, timestamp, is_sent, group_id FROM messages WHERE peer_did = ? AND group_id IS NULL ORDER BY timestamp ASC`,
    args: [peerDID],
  });
});

ipcMain.handle("messages:getByGroup", async (_event, groupID: string) => {
  return await sendRequest("mydb.query", {
    sql: `SELECT id, peer_did, text, timestamp, is_sent, group_id FROM messages WHERE group_id = ? ORDER BY timestamp ASC`,
    args: [groupID],
  });
});

// IPC handlers: groups (app-level metadata persistence)
ipcMain.handle(
  "groups:save",
  async (
    _event,
    group: { groupID: string; name?: string; memberDIDs: string[] },
  ) => {
    await sendRequest("mydb.exec", {
      sql: `INSERT INTO groups (group_id, name) VALUES (?, ?) ON CONFLICT(group_id) DO UPDATE SET name = excluded.name`,
      args: [group.groupID, group.name ?? null],
    });
    // Sync members: delete old, insert current
    await sendRequest("mydb.exec", {
      sql: `DELETE FROM group_members WHERE group_id = ?`,
      args: [group.groupID],
    });
    for (const did of group.memberDIDs) {
      await sendRequest("mydb.exec", {
        sql: `INSERT OR IGNORE INTO group_members (group_id, member_did) VALUES (?, ?)`,
        args: [group.groupID, did],
      });
    }
  },
);

ipcMain.handle("groups:getAll", async () => {
  const groups = (await sendRequest("mydb.query", {
    sql: `SELECT g.group_id, g.name FROM groups g ORDER BY g.name`,
  })) as Array<{ group_id: string; name: string | null }> | null;
  if (!groups || groups.length === 0) return [];
  const result = [];
  for (const g of groups) {
    const members = (await sendRequest("mydb.query", {
      sql: `SELECT member_did FROM group_members WHERE group_id = ?`,
      args: [g.group_id],
    })) as Array<{ member_did: string }> | null;
    result.push({
      groupID: g.group_id,
      name: g.name ?? undefined,
      memberDIDs: members ? members.map((m) => m.member_did) : [],
    });
  }
  return result;
});

ipcMain.handle("groups:delete", async (_event, groupID: string) => {
  await sendRequest("mydb.exec", {
    sql: `DELETE FROM group_members WHERE group_id = ?`,
    args: [groupID],
  });
  await sendRequest("mydb.exec", {
    sql: `DELETE FROM groups WHERE group_id = ?`,
    args: [groupID],
  });
});

// IPC handlers: data export/import (dump/restore)
ipcMain.handle("data:export", async () => {
  if (!mainWindow) return { success: false, error: "No window" };
  const result = await dialog.showSaveDialog(mainWindow, {
    title: "Export LinkSelf Data",
    defaultPath: `linkself-backup-${new Date().toISOString().slice(0, 10)}.json`,
    filters: [{ name: "JSON", extensions: ["json"] }],
  });
  if (result.canceled || !result.filePath) return { success: false };
  try {
    const records = await sendRequest("mydb.dump");
    fs.writeFileSync(
      result.filePath,
      JSON.stringify(records, null, 2),
      "utf-8",
    );
    const count = Array.isArray(records) ? records.length : 0;
    return { success: true, count, path: result.filePath };
  } catch (e) {
    return {
      success: false,
      error: e instanceof Error ? e.message : String(e),
    };
  }
});

ipcMain.handle("data:import", async () => {
  if (!mainWindow) return { success: false, error: "No window" };
  const result = await dialog.showOpenDialog(mainWindow, {
    title: "Import LinkSelf Data",
    filters: [{ name: "JSON", extensions: ["json"] }],
    properties: ["openFile"],
  });
  if (result.canceled || result.filePaths.length === 0)
    return { success: false };
  try {
    const data = fs.readFileSync(result.filePaths[0], "utf-8");
    const records = JSON.parse(data);
    const res = (await sendRequest("mydb.restore", { records })) as {
      applied: number;
    };
    return { success: true, applied: res.applied };
  } catch (e) {
    return {
      success: false,
      error: e instanceof Error ? e.message : String(e),
    };
  }
});
