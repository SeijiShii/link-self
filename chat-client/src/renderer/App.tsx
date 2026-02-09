import { useState, useEffect, useCallback } from 'react';
import ChatWindow from './components/ChatWindow';
import ContactList from './components/ContactList';
import PendingRequests from './components/PendingRequests';
import { useLinkSelf } from './hooks/useLinkSelf';
import type { Message, Contact, FriendRequest } from './types';

function contactRecordToContact(r: { did: string; name?: string; lastMessage?: string; lastMessageTime?: string }): Contact {
  return {
    did: r.did,
    name: r.name,
    lastMessage: r.lastMessage,
    lastMessageTime: r.lastMessageTime ? new Date(r.lastMessageTime) : undefined,
  };
}

function App() {
  const { myDID, listenAddr, isConnected, start, sendMessage, connect, onMessage } = useLinkSelf();
  const [currentContact, setCurrentContact] = useState<Contact | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [friendRequests, setFriendRequests] = useState<FriendRequest[]>([]);
  const [appProfile, setAppProfile] = useState<string>('');

  // Load persisted contacts and friend requests on mount
  useEffect(() => {
    if (!window.contacts || !window.friendRequests) return;
    window.contacts.get().then((list) => {
      setContacts(list.map(contactRecordToContact));
    });
    window.friendRequests.get().then((list) => {
      setFriendRequests(list);
    });
  }, []);

  useEffect(() => {
    window.app?.getProfile().then((p) => setAppProfile(p)).catch(() => {});
  }, []);

  useEffect(() => {
    if (!isConnected && !myDID) {
      start({})
        .catch((error) => {
          console.error('Failed to initialize LinkSelf:', error);
        });
    }
  }, [isConnected, myDID, start]);

  useEffect(() => {
    onMessage((peerDID: string, payload: string) => {
      // Friend request: add to pending, do not add to messages
      try {
        const parsed = JSON.parse(payload) as { type?: string; from?: string };
        if (parsed.type === 'friendRequest' && parsed.from) {
          window.friendRequests?.add({ fromDID: peerDID, receivedAt: Date.now() }).then((list) => {
            setFriendRequests(list);
          });
          return;
        }
      } catch {
        // not JSON or not friendRequest, treat as normal message
      }

      const newMessage: Message = {
        id: `${Date.now()}-${Math.random()}`,
        peerDID,
        text: payload,
        timestamp: new Date(),
        isSent: false,
      };
      setMessages((prev) => [...prev, newMessage]);

      setContacts((prev) => {
        const existing = prev.find((c) => c.did === peerDID);
        if (existing) {
          return prev.map((contact) =>
            contact.did === peerDID
              ? { ...contact, lastMessage: payload, lastMessageTime: new Date() }
              : contact
          );
        } else {
          window.contacts?.add({ did: peerDID, lastMessage: payload, lastMessageTime: new Date().toISOString() }).catch(() => {});
          return [
            ...prev,
            { did: peerDID, lastMessage: payload, lastMessageTime: new Date() },
          ];
        }
      });
    });
  }, [onMessage]);

  const handleSendMessage = async (text: string) => {
    if (!currentContact) return;

    const newMessage: Message = {
      id: `${Date.now()}-${Math.random()}`,
      peerDID: currentContact.did,
      text,
      timestamp: new Date(),
      isSent: true,
    };
    setMessages((prev) => [...prev, newMessage]);

    // Send via LinkSelf API
    try {
      await sendMessage(currentContact.did, text);
    } catch (error) {
      console.error('Failed to send message:', error);
      // Optionally show error to user
    }
  };

  const handleContactSelect = (contact: Contact) => {
    setCurrentContact(contact);
  };

  const handleAcceptFriendRequest = useCallback(async (fromDID: string) => {
    if (!window.contacts || !window.friendRequests) return;
    try {
      await window.contacts.add({ did: fromDID });
      await window.friendRequests.remove(fromDID);
      setFriendRequests((prev) => prev.filter((r) => r.fromDID !== fromDID));
      setContacts((prev) => {
        if (prev.some((c) => c.did === fromDID)) return prev;
        return [...prev, { did: fromDID }];
      });
      // Optional: send "accepted" notification to sender
      try {
        await connect(fromDID);
        await sendMessage(fromDID, JSON.stringify({ type: 'friendRequestAccepted', from: myDID }));
      } catch {
        // ignore
      }
    } catch (e) {
      console.error('Failed to accept friend request:', e);
    }
  }, [connect, sendMessage, myDID]);

  const handleRejectFriendRequest = useCallback(async (fromDID: string) => {
    if (!window.friendRequests) return;
    try {
      await window.friendRequests.remove(fromDID);
      setFriendRequests((prev) => prev.filter((r) => r.fromDID !== fromDID));
    } catch (e) {
      console.error('Failed to reject friend request:', e);
    }
  }, []);

  const [copyFeedback, setCopyFeedback] = useState(false);
  const handleCopyCombined = useCallback(async () => {
    if (!myDID) return;
    try {
      const text = listenAddr
        ? `DID: ${myDID}\nListen: ${listenAddr}`
        : `DID: ${myDID}`;
      await navigator.clipboard.writeText(text);
      setCopyFeedback(true);
      setTimeout(() => setCopyFeedback(false), 1500);
    } catch (e) {
      console.error('Failed to copy DID+Listen:', e);
    }
  }, [myDID, listenAddr]);

  return (
    <div className="app">
      <div className="app-header">
        <h1>LinkSelf Chat</h1>
        {appProfile && <span className="app-profile" title="起動時のプロファイル（2インスタンス時は userA / userB で別DID）">{appProfile}</span>}
        {myDID && (
          <div className="my-did">
            <span>My DID: </span>
            <code>{myDID.substring(0, 20)}...</code>
            <button type="button" className="btn-copy-did" onClick={handleCopyCombined} title={copyFeedback ? 'コピーしました' : 'DIDとListen（結合形式）をコピー'}>
              {copyFeedback ? (
                <svg className="icon-copy" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              ) : (
                <svg className="icon-copy" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                </svg>
              )}
            </button>
            {listenAddr && (
              <div className="listen-addr" title="2つ目のインスタンス起動時に BOOTSTRAP_PEER に指定。QRコードではDIDと一緒に共有予定">
                <span className="listen-addr-label">Listen: </span>
                <code className="listen-addr-value">{listenAddr.length > 28 ? listenAddr.substring(0, 28) + '...' : listenAddr}</code>
              </div>
            )}
          </div>
        )}
      </div>
      <div className="app-body">
        <div className="app-sidebar">
          <PendingRequests
            requests={friendRequests}
            onAccept={handleAcceptFriendRequest}
            onReject={handleRejectFriendRequest}
          />
          <ContactList
            contacts={contacts}
            currentContact={currentContact}
            onSelect={handleContactSelect}
            myDID={myDID ?? null}
            connect={connect}
            sendMessage={sendMessage}
          />
        </div>
        <ChatWindow
          contact={currentContact}
          messages={messages}
          onSendMessage={handleSendMessage}
        />
      </div>
    </div>
  );
}

export default App;
