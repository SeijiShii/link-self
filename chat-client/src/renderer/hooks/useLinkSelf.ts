import { useState, useEffect, useCallback } from 'react';
import type { StartParams, StartResult, SendMessageParams, ConnectParams } from '../../types/linkself';

interface UseLinkSelfReturn {
  myDID: string | null;
  isConnected: boolean;
  start: (params: StartParams) => Promise<void>;
  stop: () => Promise<void>;
  sendMessage: (peerDID: string, message: string) => Promise<void>;
  connect: (peerDID: string) => Promise<void>;
  onMessage: (callback: (peerDID: string, payload: string) => void) => void;
}

export function useLinkSelf(): UseLinkSelfReturn {
  const [myDID, setMyDID] = useState<string | null>(null);
  const [isConnected, setIsConnected] = useState(false);

  const start = useCallback(async (params: StartParams) => {
    if (!window.linkself) {
      throw new Error('LinkSelf API not available');
    }
    try {
      const result = await window.linkself.start(params) as StartResult;
      setMyDID(result.did);
      setIsConnected(true);
    } catch (error) {
      console.error('Failed to start LinkSelf:', error);
      throw error;
    }
  }, []);

  const stop = useCallback(async () => {
    if (!window.linkself) {
      throw new Error('LinkSelf API not available');
    }
    try {
      await window.linkself.stop();
      setIsConnected(false);
    } catch (error) {
      console.error('Failed to stop LinkSelf:', error);
      throw error;
    }
  }, []);

  const sendMessage = useCallback(async (peerDID: string, message: string) => {
    if (!window.linkself) {
      throw new Error('LinkSelf API not available');
    }
    try {
      await window.linkself.sendMessage({ peerDID, message });
    } catch (error) {
      console.error('Failed to send message:', error);
      throw error;
    }
  }, []);

  const connect = useCallback(async (peerDID: string) => {
    if (!window.linkself) {
      throw new Error('LinkSelf API not available');
    }
    try {
      await window.linkself.connect({ peerDID });
    } catch (error) {
      console.error('Failed to connect:', error);
      throw error;
    }
  }, []);

  const onMessage = useCallback((callback: (peerDID: string, payload: string) => void) => {
    if (!window.linkself) {
      console.warn('LinkSelf API not available');
      return;
    }
    window.linkself.onMessage(callback);
  }, []);

  useEffect(() => {
    // Check if linkself API is available
    if (!window.linkself) {
      console.warn('LinkSelf API not available. Make sure preload script is loaded.');
      return;
    }

    // Try to get existing DID on mount
    window.linkself.getMyDID()
      .then((did: string) => {
        if (did) {
          setMyDID(did);
          setIsConnected(true);
        }
      })
      .catch(() => {
        // Not started yet, ignore
      });
  }, []);

  return {
    myDID,
    isConnected,
    start,
    stop,
    sendMessage,
    connect,
    onMessage,
  };
}
