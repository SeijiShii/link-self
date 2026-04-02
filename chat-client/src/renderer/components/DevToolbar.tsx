import { useState, useCallback } from "react";

interface DevToolbarProps {
  onTestContactAdded: (did: string) => void;
}

export default function DevToolbar({ onTestContactAdded }: DevToolbarProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [testDIDs, setTestDIDs] = useState<string[]>([]);
  const [status, setStatus] = useState<string | null>(null);

  const showStatus = useCallback((msg: string) => {
    setStatus(msg);
    setTimeout(() => setStatus(null), 2000);
  }, []);

  const handleGenerateTestDID = useCallback(async () => {
    try {
      const result = await window.dev.generateTestDID();
      setTestDIDs((prev) => [...prev, result.did]);
      onTestContactAdded(result.did);
      showStatus(`Test DID generated: ${result.did.substring(0, 24)}...`);
    } catch (e) {
      console.error("Failed to generate test DID:", e);
      showStatus("Failed to generate test DID");
    }
  }, [onTestContactAdded, showStatus]);

  const handleInjectMessage = useCallback(
    async (fromDID: string) => {
      const payload = `Test message at ${new Date().toLocaleTimeString()}`;
      try {
        await window.dev.injectTestMessage(fromDID, payload);
        showStatus("Test message injected");
      } catch (e) {
        console.error("Failed to inject test message:", e);
        showStatus("Failed to inject message");
      }
    },
    [showStatus],
  );

  const handleExport = useCallback(async () => {
    try {
      const result = await window.data.export();
      if (result.success) {
        showStatus(`Exported ${result.count} records`);
      } else if (result.error) {
        showStatus(`Export failed: ${result.error}`);
      }
    } catch (e) {
      console.error("Export failed:", e);
      showStatus("Export failed");
    }
  }, [showStatus]);

  const handleImport = useCallback(async () => {
    try {
      const result = await window.data.import();
      if (result.success) {
        showStatus(`Imported ${result.applied} records`);
      } else if (result.error) {
        showStatus(`Import failed: ${result.error}`);
      }
    } catch (e) {
      console.error("Import failed:", e);
      showStatus("Import failed");
    }
  }, [showStatus]);

  const handleInjectFriendRequest = useCallback(async () => {
    try {
      const result = await window.dev.generateTestDID();
      const payload = JSON.stringify({
        type: "friendRequest",
        from: result.did,
      });
      await window.dev.injectTestMessage(result.did, payload);
      showStatus("Test friend request injected");
    } catch (e) {
      console.error("Failed to inject friend request:", e);
      showStatus("Failed to inject friend request");
    }
  }, [showStatus]);

  if (!isOpen) {
    return (
      <button
        type="button"
        className="dev-toolbar-toggle"
        onClick={() => setIsOpen(true)}
        title="Open Dev Tools"
      >
        DEV
      </button>
    );
  }

  return (
    <div className="dev-toolbar">
      <div className="dev-toolbar-header">
        <span>Dev Tools</span>
        <button type="button" onClick={() => setIsOpen(false)} title="Close">
          ×
        </button>
      </div>
      <div className="dev-toolbar-actions">
        <button type="button" onClick={handleGenerateTestDID}>
          + Test Contact
        </button>
        <button type="button" onClick={handleInjectFriendRequest}>
          + Test Friend Request
        </button>
      </div>
      <div className="dev-toolbar-actions">
        <button type="button" onClick={handleExport}>
          Export Data
        </button>
        <button type="button" onClick={handleImport}>
          Import Data
        </button>
      </div>
      {testDIDs.length > 0 && (
        <div className="dev-toolbar-dids">
          {testDIDs.map((did) => (
            <div key={did} className="dev-toolbar-did-item">
              <code>{did.substring(0, 20)}...</code>
              <button
                type="button"
                onClick={() => handleInjectMessage(did)}
                title="Inject message from this DID"
              >
                Send
              </button>
            </div>
          ))}
        </div>
      )}
      {status && <div className="dev-toolbar-status">{status}</div>}
    </div>
  );
}
