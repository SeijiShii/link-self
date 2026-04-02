import { useState, useCallback, useEffect, useRef } from "react";

interface PairingDialogProps {
  onClose: () => void;
}

type PairingMode = "menu" | "generate" | "enter";

export default function PairingDialog({ onClose }: PairingDialogProps) {
  const [mode, setMode] = useState<PairingMode>("menu");
  const [secret, setSecret] = useState<string | null>(null);
  const [ttl] = useState(300);
  const [remaining, setRemaining] = useState(0);
  const [inputSecret, setInputSecret] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Cleanup timer
  useEffect(() => {
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, []);

  const handleGenerateToken = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await window.pairing.createToken(ttl);
      setSecret(result.secret);
      setRemaining(ttl);
      setMode("generate");
      // Start countdown
      timerRef.current = setInterval(() => {
        setRemaining((prev) => {
          if (prev <= 1) {
            if (timerRef.current) clearInterval(timerRef.current);
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to generate token");
    } finally {
      setLoading(false);
    }
  }, [ttl]);

  const handleCompletePairing = useCallback(async () => {
    if (!inputSecret.trim()) {
      setError("Enter the pairing secret");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await window.pairing.complete(inputSecret.trim());
      setStatus(
        "Pairing successful! Restart the app to use the shared identity.",
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "Pairing failed");
    } finally {
      setLoading(false);
    }
  }, [inputSecret]);

  const handleCopySecret = useCallback(async () => {
    if (!secret) return;
    try {
      await navigator.clipboard.writeText(secret);
      setStatus("Copied!");
      setTimeout(() => setStatus(null), 1500);
    } catch {
      // fallback
    }
  }, [secret]);

  const formatTime = (seconds: number) => {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, "0")}`;
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal-dialog pairing-dialog"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="pairing-header">
          <h3>Device Pairing</h3>
          <button type="button" className="btn-close" onClick={onClose}>
            x
          </button>
        </div>

        {mode === "menu" && (
          <div className="pairing-menu">
            <p className="pairing-desc">
              Link another device to share your identity (DID).
            </p>
            <button
              type="button"
              className="pairing-option"
              onClick={handleGenerateToken}
              disabled={loading}
            >
              <strong>Generate Pairing Code</strong>
              <span>
                Create a code on this device to share with a new device
              </span>
            </button>
            <button
              type="button"
              className="pairing-option"
              onClick={() => setMode("enter")}
            >
              <strong>Enter Pairing Code</strong>
              <span>Enter a code received from another device</span>
            </button>
          </div>
        )}

        {mode === "generate" && secret && (
          <div className="pairing-generate">
            <p className="pairing-desc">
              Share this code with your new device:
            </p>
            <div className="pairing-secret-display">
              <code className="pairing-secret">{secret}</code>
              <button
                type="button"
                onClick={handleCopySecret}
                className="btn-copy-secret"
              >
                Copy
              </button>
            </div>
            <div
              className={`pairing-timer ${remaining <= 30 ? "expiring" : ""}`}
            >
              {remaining > 0
                ? `Expires in ${formatTime(remaining)}`
                : "Expired - generate a new code"}
            </div>
            <button
              type="button"
              className="btn-back"
              onClick={() => {
                if (timerRef.current) clearInterval(timerRef.current);
                setSecret(null);
                setMode("menu");
              }}
            >
              Back
            </button>
          </div>
        )}

        {mode === "enter" && (
          <div className="pairing-enter">
            <p className="pairing-desc">
              Enter the pairing code from your existing device:
            </p>
            <input
              type="text"
              className="pairing-input"
              placeholder="Paste pairing code..."
              value={inputSecret}
              onChange={(e) => setInputSecret(e.target.value)}
              disabled={loading || !!status}
              autoFocus
            />
            {!status && (
              <button
                type="button"
                className="btn-pair"
                onClick={handleCompletePairing}
                disabled={loading || !inputSecret.trim()}
              >
                {loading ? "Pairing..." : "Complete Pairing"}
              </button>
            )}
            {!status && (
              <button
                type="button"
                className="btn-back"
                onClick={() => {
                  setMode("menu");
                  setInputSecret("");
                  setError(null);
                }}
              >
                Back
              </button>
            )}
          </div>
        )}

        {error && <p className="pairing-error">{error}</p>}
        {status && <p className="pairing-success">{status}</p>}
      </div>
    </div>
  );
}
