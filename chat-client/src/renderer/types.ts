export interface Message {
  id: string;
  peerDID: string;
  text: string;
  timestamp: Date;
  isSent: boolean;
}

export interface Contact {
  did: string;
  name?: string;
  lastMessage?: string;
  lastMessageTime?: Date;
}

export interface FriendRequest {
  fromDID: string;
  receivedAt: number;
}

export interface ChatState {
  myDID: string | null;
  currentContact: Contact | null;
  messages: Message[];
  contacts: Contact[];
  friendRequests: FriendRequest[];
  isConnected: boolean;
}
