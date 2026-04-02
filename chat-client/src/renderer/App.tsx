import { useState, useEffect, useCallback } from "react";
import ChatWindow from "./components/ChatWindow";
import ContactList from "./components/ContactList";
import PendingRequests from "./components/PendingRequests";
import GroupCreateDialog from "./components/GroupCreateDialog";
import PairingDialog from "./components/PairingDialog";
import DevToolbar from "./components/DevToolbar";
import { useLinkSelf } from "./hooks/useLinkSelf";
import type {
  Message,
  Contact,
  FriendRequest,
  Group,
  Conversation,
} from "./types";

function contactRecordToContact(r: {
  did: string;
  name?: string;
  lastMessage?: string;
  lastMessageTime?: string;
}): Contact {
  return {
    did: r.did,
    name: r.name,
    lastMessage: r.lastMessage,
    lastMessageTime: r.lastMessageTime
      ? new Date(r.lastMessageTime)
      : undefined,
  };
}

/** Extract groupID from a group message payload, or null for 1:1 */
function parseGroupMessage(
  payload: string,
): { groupID: string; text: string; senderDID: string } | null {
  try {
    const parsed = JSON.parse(payload) as {
      type?: string;
      groupID?: string;
      text?: string;
      senderDID?: string;
    };
    if (
      parsed.type === "groupMessage" &&
      parsed.groupID &&
      parsed.text != null
    ) {
      return {
        groupID: parsed.groupID,
        text: parsed.text,
        senderDID: parsed.senderDID ?? "",
      };
    }
  } catch {
    // not a group message
  }
  return null;
}

function App() {
  const { myDID, isConnected, start, sendMessage, connect, onMessage } =
    useLinkSelf();
  const [currentConversation, setCurrentConversation] =
    useState<Conversation | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [friendRequests, setFriendRequests] = useState<FriendRequest[]>([]);
  const [appProfile, setAppProfile] = useState<string>("");
  const [showGroupCreate, setShowGroupCreate] = useState(false);
  const [showPairing, setShowPairing] = useState(false);

  useEffect(() => {
    window.app
      ?.getProfile()
      .then((p) => setAppProfile(p))
      .catch(() => {});
  }, []);

  // Start daemon, run migrations, then load persisted data
  useEffect(() => {
    if (!isConnected && !myDID) {
      start({})
        .then(() => window.messages?.migrate())
        .then(async () => {
          const [contactList, frList, groupList] = await Promise.all([
            window.contacts?.get() ?? Promise.resolve([]),
            window.friendRequests?.get() ?? Promise.resolve([]),
            window.groups?.getAll() ?? Promise.resolve([]),
          ]);
          setContacts(contactList.map(contactRecordToContact));
          setFriendRequests(frList);
          setGroups(
            groupList.map((g) => ({
              groupID: g.groupID,
              name: g.name,
              memberDIDs: g.memberDIDs,
            })),
          );
        })
        .catch((error) => {
          console.error("Failed to initialize LinkSelf:", error);
        });
    }
  }, [isConnected, myDID, start]);

  useEffect(() => {
    onMessage((peerDID: string, payload: string) => {
      // Friend request
      try {
        const parsed = JSON.parse(payload) as { type?: string; from?: string };
        if (parsed.type === "friendRequest" && parsed.from) {
          window.friendRequests
            ?.add({ fromDID: peerDID, receivedAt: Date.now() })
            .then((list) => {
              setFriendRequests(list);
            });
          return;
        }
        if (parsed.type === "friendRequestAccepted") {
          return;
        }
      } catch {
        // not JSON, treat as normal message
      }

      // Group message
      const groupMsg = parseGroupMessage(payload);
      if (groupMsg) {
        const newMessage: Message = {
          id: `${Date.now()}-${Math.random()}`,
          peerDID,
          text: groupMsg.text,
          timestamp: new Date(),
          isSent: false,
          groupID: groupMsg.groupID,
        };
        setMessages((prev) => [...prev, newMessage]);
        window.messages
          ?.insert({
            id: newMessage.id,
            peerDID,
            text: groupMsg.text,
            timestamp: newMessage.timestamp.getTime(),
            isSent: false,
            groupID: groupMsg.groupID,
          })
          .catch(() => {});

        // Update group lastMessage
        setGroups((prev) =>
          prev.map((g) =>
            g.groupID === groupMsg.groupID
              ? {
                  ...g,
                  lastMessage: groupMsg.text,
                  lastMessageTime: new Date(),
                }
              : g,
          ),
        );
        return;
      }

      // 1:1 message
      const newMessage: Message = {
        id: `${Date.now()}-${Math.random()}`,
        peerDID,
        text: payload,
        timestamp: new Date(),
        isSent: false,
      };
      setMessages((prev) => [...prev, newMessage]);
      window.messages
        ?.insert({
          id: newMessage.id,
          peerDID,
          text: payload,
          timestamp: newMessage.timestamp.getTime(),
          isSent: false,
        })
        .catch(() => {});

      setContacts((prev) => {
        const existing = prev.find((c) => c.did === peerDID);
        if (existing) {
          return prev.map((contact) =>
            contact.did === peerDID
              ? {
                  ...contact,
                  lastMessage: payload,
                  lastMessageTime: new Date(),
                }
              : contact,
          );
        } else {
          window.contacts
            ?.add({
              did: peerDID,
              lastMessage: payload,
              lastMessageTime: new Date().toISOString(),
            })
            .catch(() => {});
          return [
            ...prev,
            { did: peerDID, lastMessage: payload, lastMessageTime: new Date() },
          ];
        }
      });
    });
  }, [onMessage]);

  const handleSendMessage = async (text: string) => {
    if (!currentConversation) return;

    if (currentConversation.type === "contact") {
      const contact = currentConversation.contact;
      const newMessage: Message = {
        id: `${Date.now()}-${Math.random()}`,
        peerDID: contact.did,
        text,
        timestamp: new Date(),
        isSent: true,
      };
      setMessages((prev) => [...prev, newMessage]);
      window.messages
        ?.insert({
          id: newMessage.id,
          peerDID: contact.did,
          text,
          timestamp: newMessage.timestamp.getTime(),
          isSent: true,
        })
        .catch(() => {});

      try {
        await sendMessage(contact.did, text);
      } catch (error) {
        console.error("Failed to send message:", error);
      }
    } else {
      // Group message: send to each member individually
      const group = currentConversation.group;
      const groupPayload = JSON.stringify({
        type: "groupMessage",
        groupID: group.groupID,
        text,
        senderDID: myDID,
      });
      const newMessage: Message = {
        id: `${Date.now()}-${Math.random()}`,
        peerDID: myDID ?? "",
        text,
        timestamp: new Date(),
        isSent: true,
        groupID: group.groupID,
      };
      setMessages((prev) => [...prev, newMessage]);
      window.messages
        ?.insert({
          id: newMessage.id,
          peerDID: myDID ?? "",
          text,
          timestamp: newMessage.timestamp.getTime(),
          isSent: true,
          groupID: group.groupID,
        })
        .catch(() => {});

      // Send to each member except self
      for (const memberDID of group.memberDIDs) {
        if (memberDID === myDID) continue;
        try {
          await sendMessage(memberDID, groupPayload);
        } catch (error) {
          console.error(`Failed to send group message to ${memberDID}:`, error);
        }
      }
    }
  };

  const handleConversationSelect = useCallback(async (conv: Conversation) => {
    setCurrentConversation(conv);
    try {
      if (conv.type === "contact") {
        const rows = await window.messages?.getByPeer(conv.contact.did);
        if (rows && rows.length > 0) {
          const loaded: Message[] = rows.map((r) => ({
            id: r.id,
            peerDID: r.peer_did,
            text: r.text,
            timestamp: new Date(r.timestamp),
            isSent: r.is_sent === 1,
          }));
          setMessages((prev) => {
            const existingIds = new Set(loaded.map((m) => m.id));
            const inMemoryOnly = prev.filter(
              (m) =>
                m.peerDID === conv.contact.did &&
                !m.groupID &&
                !existingIds.has(m.id),
            );
            return [...loaded, ...inMemoryOnly];
          });
        }
      } else {
        const rows = await window.messages?.getByGroup(conv.group.groupID);
        if (rows && rows.length > 0) {
          const loaded: Message[] = rows.map((r) => ({
            id: r.id,
            peerDID: r.peer_did,
            text: r.text,
            timestamp: new Date(r.timestamp),
            isSent: r.is_sent === 1,
            groupID: r.group_id ?? undefined,
          }));
          setMessages((prev) => {
            const existingIds = new Set(loaded.map((m) => m.id));
            const inMemoryOnly = prev.filter(
              (m) => m.groupID === conv.group.groupID && !existingIds.has(m.id),
            );
            return [...loaded, ...inMemoryOnly];
          });
        }
      }
    } catch {
      // DB not ready yet
    }
  }, []);

  const handleCreateGroup = useCallback(
    async (name: string, memberDIDs: string[]) => {
      try {
        const result = await window.network.create(memberDIDs);
        const groupID = result.groupID;
        // Include self in member list for display
        const allMembers = myDID
          ? [myDID, ...memberDIDs.filter((d) => d !== myDID)]
          : memberDIDs;
        const newGroup: Group = { groupID, name, memberDIDs: allMembers };
        await window.groups?.save(newGroup);
        setGroups((prev) => [...prev, newGroup]);
        setShowGroupCreate(false);
        // Auto-select the new group
        setCurrentConversation({ type: "group", group: newGroup });
      } catch (e) {
        console.error("Failed to create group:", e);
      }
    },
    [myDID],
  );

  const handleAcceptFriendRequest = useCallback(
    async (fromDID: string) => {
      if (!window.contacts || !window.friendRequests) return;
      try {
        await window.contacts.add({ did: fromDID });
        await window.friendRequests.remove(fromDID);
        setFriendRequests((prev) => prev.filter((r) => r.fromDID !== fromDID));
        setContacts((prev) => {
          if (prev.some((c) => c.did === fromDID)) return prev;
          return [...prev, { did: fromDID }];
        });
        try {
          await connect(fromDID);
          await sendMessage(
            fromDID,
            JSON.stringify({ type: "friendRequestAccepted", from: myDID }),
          );
        } catch {
          // ignore
        }
      } catch (e) {
        console.error("Failed to accept friend request:", e);
      }
    },
    [connect, sendMessage, myDID],
  );

  const handleRejectFriendRequest = useCallback(async (fromDID: string) => {
    if (!window.friendRequests) return;
    try {
      await window.friendRequests.remove(fromDID);
      setFriendRequests((prev) => prev.filter((r) => r.fromDID !== fromDID));
    } catch (e) {
      console.error("Failed to reject friend request:", e);
    }
  }, []);

  const handleTestContactAdded = useCallback(async (did: string) => {
    try {
      await window.contacts?.add({ did });
      setContacts((prev) => {
        if (prev.some((c) => c.did === did)) return prev;
        return [...prev, { did }];
      });
    } catch (e) {
      console.error("Failed to add test contact:", e);
    }
  }, []);

  const [copyFeedback, setCopyFeedback] = useState(false);
  const handleCopyDID = useCallback(async () => {
    if (!myDID) return;
    try {
      await navigator.clipboard.writeText(myDID);
      setCopyFeedback(true);
      setTimeout(() => setCopyFeedback(false), 1500);
    } catch (e) {
      console.error("Failed to copy DID:", e);
    }
  }, [myDID]);

  // Filter messages for current conversation
  const filteredMessages = (() => {
    if (!currentConversation) return [];
    if (currentConversation.type === "contact") {
      return messages.filter(
        (m) => m.peerDID === currentConversation.contact.did && !m.groupID,
      );
    }
    return messages.filter(
      (m) => m.groupID === currentConversation.group.groupID,
    );
  })();

  return (
    <div className="app">
      <div className="app-header">
        <h1>LinkSelf Chat</h1>
        {appProfile && (
          <span
            className="app-profile"
            title="起動時のプロファイル（2インスタンス時は userA / userB で別DID）"
          >
            {appProfile}
          </span>
        )}
        {myDID && (
          <div className="my-did">
            <span>My DID: </span>
            <code>{myDID.substring(0, 20)}...</code>
            <button
              type="button"
              className="btn-copy-did"
              onClick={handleCopyDID}
              title={copyFeedback ? "コピーしました" : "DIDをコピー"}
            >
              {copyFeedback ? (
                <svg
                  className="icon-copy"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  aria-hidden
                >
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              ) : (
                <svg
                  className="icon-copy"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  aria-hidden
                >
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                </svg>
              )}
            </button>
            <button
              type="button"
              className="btn-pairing"
              onClick={() => setShowPairing(true)}
              title="Device Pairing"
            >
              Link
            </button>
          </div>
        )}
        <DevToolbar onTestContactAdded={handleTestContactAdded} />
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
            groups={groups}
            currentConversation={currentConversation}
            onSelect={handleConversationSelect}
            myDID={myDID ?? null}
            connect={connect}
            sendMessage={sendMessage}
            onCreateGroup={() => setShowGroupCreate(true)}
          />
        </div>
        <ChatWindow
          conversation={currentConversation}
          messages={filteredMessages}
          onSendMessage={handleSendMessage}
        />
      </div>
      {showGroupCreate && (
        <GroupCreateDialog
          contacts={contacts}
          onClose={() => setShowGroupCreate(false)}
          onCreate={handleCreateGroup}
        />
      )}
      {showPairing && <PairingDialog onClose={() => setShowPairing(false)} />}
    </div>
  );
}

export default App;
