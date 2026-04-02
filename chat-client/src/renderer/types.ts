export interface Message {
  id: string;
  peerDID: string;
  text: string;
  timestamp: Date;
  isSent: boolean;
  groupID?: string;
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

export interface Group {
  groupID: string;
  name?: string;
  memberDIDs: string[];
  lastMessage?: string;
  lastMessageTime?: Date;
}

/** A conversation can be either a 1:1 contact or a group */
export type Conversation =
  | { type: "contact"; contact: Contact }
  | { type: "group"; group: Group };

export interface ChatState {
  myDID: string | null;
  currentContact: Contact | null;
  messages: Message[];
  contacts: Contact[];
  friendRequests: FriendRequest[];
  isConnected: boolean;
}
