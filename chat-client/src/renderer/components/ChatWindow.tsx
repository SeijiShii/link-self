import { useState } from 'react';
import MessageList from './MessageList';
import MessageInput from './MessageInput';
import type { Contact, Message } from '../types';

interface ChatWindowProps {
  contact: Contact | null;
  messages: Message[];
  onSendMessage: (text: string) => void;
}

export default function ChatWindow({ contact, messages, onSendMessage }: ChatWindowProps) {
  const [inputValue, setInputValue] = useState('');

  if (!contact) {
    return (
      <div className="chat-window empty">
        <div className="chat-window-placeholder">
          <p>連絡先を選択してチャットを開始</p>
        </div>
      </div>
    );
  }

  const handleSend = () => {
    if (inputValue.trim()) {
      onSendMessage(inputValue.trim());
      setInputValue('');
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="chat-window">
      <div className="chat-header">
        <div className="chat-contact-name">
          {contact.name || contact.did.substring(0, 16) + '...'}
        </div>
        <div className="chat-contact-did">
          {contact.did}
        </div>
      </div>
      <MessageList messages={messages} />
      <MessageInput
        value={inputValue}
        onChange={setInputValue}
        onSend={handleSend}
        onKeyPress={handleKeyPress}
      />
    </div>
  );
}
