import { useState, useEffect } from 'react';
import ChatWindow from './components/ChatWindow';
import ContactList from './components/ContactList';
import { useLinkSelf } from './hooks/useLinkSelf';
import type { Message, Contact } from './types';

function App() {
  const { myDID, isConnected, start, sendMessage, onMessage } = useLinkSelf();
  const [currentContact, setCurrentContact] = useState<Contact | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [contacts, setContacts] = useState<Contact[]>([]);

  useEffect(() => {
    // Initialize LinkSelf on mount
    if (!isConnected && !myDID) {
      start({})
        .catch((error) => {
          console.error('Failed to initialize LinkSelf:', error);
        });
    }
  }, [isConnected, myDID, start]);

  useEffect(() => {
    // Set up message listener
    onMessage((peerDID: string, payload: string) => {
      const newMessage: Message = {
        id: `${Date.now()}-${Math.random()}`,
        peerDID,
        text: payload,
        timestamp: new Date(),
        isSent: false,
      };
      setMessages((prev) => [...prev, newMessage]);

      // Update contact's last message
      setContacts((prev) => {
        const existing = prev.find((c) => c.did === peerDID);
        if (existing) {
          return prev.map((contact) =>
            contact.did === peerDID
              ? { ...contact, lastMessage: payload, lastMessageTime: new Date() }
              : contact
          );
        } else {
          // Add new contact
          return [
            ...prev,
            {
              did: peerDID,
              lastMessage: payload,
              lastMessageTime: new Date(),
            },
          ];
        }
      });

      // If this is the current contact, add to messages
      if (currentContact?.did === peerDID) {
        setMessages((prev) => [...prev, newMessage]);
      }
    });
  }, [onMessage, currentContact]);

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
    // Filter messages for the selected contact
    const contactMessages = messages.filter((msg) => msg.peerDID === contact.did);
    // If no messages exist, start with empty array
    // In a real app, load messages from storage/API
  };

  return (
    <div className="app">
      <div className="app-header">
        <h1>LinkSelf Chat</h1>
        {myDID && (
          <div className="my-did">
            <span>My DID: </span>
            <code>{myDID.substring(0, 20)}...</code>
          </div>
        )}
      </div>
      <div className="app-body">
        <ContactList
          contacts={contacts}
          currentContact={currentContact}
          onSelect={handleContactSelect}
        />
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
