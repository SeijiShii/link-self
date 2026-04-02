import { useState } from "react";
import MessageList from "./MessageList";
import MessageInput from "./MessageInput";
import type { Conversation, Message } from "../types";

interface ChatWindowProps {
  conversation: Conversation | null;
  messages: Message[];
  onSendMessage: (text: string) => void;
}

export default function ChatWindow({
  conversation,
  messages,
  onSendMessage,
}: ChatWindowProps) {
  const [inputValue, setInputValue] = useState("");

  if (!conversation) {
    return (
      <div className="chat-window empty">
        <div className="chat-window-placeholder">
          <p>連絡先またはグループを選択してチャットを開始</p>
        </div>
      </div>
    );
  }

  const handleSend = () => {
    if (inputValue.trim()) {
      onSendMessage(inputValue.trim());
      setInputValue("");
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const isGroup = conversation.type === "group";
  const title = isGroup
    ? conversation.group.name || `Group (${conversation.group.memberDIDs.length})`
    : conversation.contact.name ||
      conversation.contact.did.substring(0, 16) + "...";
  const subtitle = isGroup
    ? `${conversation.group.memberDIDs.length} members`
    : conversation.contact.did;

  return (
    <div className="chat-window">
      <div className="chat-header">
        <div className="chat-contact-name">
          {isGroup && <span className="group-badge">G</span>}
          {title}
        </div>
        <div className="chat-contact-did">{subtitle}</div>
      </div>
      <MessageList messages={messages} />
      <MessageInput
        value={inputValue}
        onChange={setInputValue}
        onSend={handleSend}
        onKeyDown={handleKeyDown}
      />
    </div>
  );
}
