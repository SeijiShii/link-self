import { useState, useCallback } from 'react';
import type { Contact, Group, Conversation } from '../types';

interface ContactListProps {
  contacts: Contact[];
  groups: Group[];
  currentConversation: Conversation | null;
  onSelect: (conv: Conversation) => void;
  myDID: string | null;
  connect: (peerDID: string) => Promise<void>;
  sendMessage: (peerDID: string, message: string) => Promise<void>;
  onCreateGroup: () => void;
}

/** 貼り付けテキストから DID のみ抽出（DID: 行や生 did:key:...） */
function parseDIDFromPaste(text: string): string | null {
  const trimmed = text.trim();
  if (trimmed.startsWith('DID:')) {
    const after = trimmed.slice(4).trim();
    const firstLine = after.split(/\r?\n/)[0].trim();
    return firstLine.startsWith('did:key:') ? firstLine : null;
  }
  const firstLine = trimmed.split(/\r?\n/)[0].trim();
  return firstLine.startsWith('did:key:') ? firstLine : null;
}

function isActiveConversation(current: Conversation | null, conv: Conversation): boolean {
  if (!current) return false;
  if (current.type === 'contact' && conv.type === 'contact') {
    return current.contact.did === conv.contact.did;
  }
  if (current.type === 'group' && conv.type === 'group') {
    return current.group.groupID === conv.group.groupID;
  }
  return false;
}

export default function ContactList({
  contacts,
  groups,
  currentConversation,
  onSelect,
  myDID,
  connect,
  sendMessage,
  onCreateGroup,
}: ContactListProps) {
  const [showAddModal, setShowAddModal] = useState(false);
  const [addDID, setAddDID] = useState('');
  const [addError, setAddError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);

  const handleSendFriendRequest = useCallback(async () => {
    const did = parseDIDFromPaste(addDID);
    setAddError(null);
    if (!addDID.trim()) {
      setAddError('DIDを入力してください');
      return;
    }
    if (!did) {
      setAddError('有効なDID（did:key:...）を入力してください');
      return;
    }
    if (myDID && did === myDID) {
      setAddError('自分のDIDは追加できません');
      return;
    }
    if (contacts.some((c) => c.did === did)) {
      setAddError('既に連絡先にいます');
      return;
    }
    setSending(true);
    try {
      await connect(did);
      await sendMessage(did, JSON.stringify({ type: 'friendRequest', from: myDID }));
      setShowAddModal(false);
      setAddDID('');
    } catch (e) {
      setAddError(e instanceof Error ? e.message : '送信に失敗しました');
    } finally {
      setSending(false);
    }
  }, [addDID, myDID, contacts, connect, sendMessage]);

  const openModal = () => {
    setAddError(null);
    setAddDID('');
    setShowAddModal(true);
  };

  return (
    <div className="contact-list">
      {/* Groups section */}
      {groups.length > 0 && (
        <>
          <div className="contact-list-header">
            <h2>Groups</h2>
            <button type="button" className="btn-add-contact" onClick={onCreateGroup} title="Create group">
              ＋
            </button>
          </div>
          <div className="contact-list-items">
            {groups.map((group) => {
              const conv: Conversation = { type: 'group', group };
              return (
                <div
                  key={group.groupID}
                  className={`contact-item ${isActiveConversation(currentConversation, conv) ? 'active' : ''}`}
                  onClick={() => onSelect(conv)}
                >
                  <div className="contact-avatar group-avatar">G</div>
                  <div className="contact-info">
                    <div className="contact-name">
                      {group.name || `Group (${group.memberDIDs.length})`}
                    </div>
                    {group.lastMessage && (
                      <div className="contact-last-message">{group.lastMessage}</div>
                    )}
                  </div>
                  {group.lastMessageTime && (
                    <div className="contact-time">
                      {group.lastMessageTime.toLocaleTimeString('ja-JP', {
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </>
      )}

      {/* Contacts section */}
      <div className="contact-list-header">
        <h2>Contacts</h2>
        <div className="contact-list-header-actions">
          {groups.length === 0 && (
            <button type="button" className="btn-add-contact" onClick={onCreateGroup} title="Create group">
              G＋
            </button>
          )}
          {myDID && (
            <button type="button" className="btn-add-contact" onClick={openModal} title="友達申請を送る">
              ＋
            </button>
          )}
        </div>
      </div>
      {showAddModal && (
        <div className="modal-overlay" onClick={() => !sending && setShowAddModal(false)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()}>
            <h3>友達申請を送る</h3>
            <p className="modal-hint">相手のDIDを入力（QRスキャンまたはコピペ）</p>
            <input
              type="text"
              className="modal-input"
              placeholder="did:key:... を貼り付け"
              value={addDID}
              onChange={(e) => setAddDID(e.target.value)}
              onPaste={(e) => {
                const pasted = e.clipboardData.getData('text');
                const parsed = parseDIDFromPaste(pasted);
                if (parsed) {
                  e.preventDefault();
                  setAddDID(parsed);
                  setAddError(null);
                }
              }}
              disabled={sending}
            />
            {addError && <p className="modal-error">{addError}</p>}
            <div className="modal-actions">
              <button type="button" className="btn-cancel" onClick={() => !sending && setShowAddModal(false)} disabled={sending}>
                キャンセル
              </button>
              <button type="button" className="btn-send-request" onClick={handleSendFriendRequest} disabled={sending}>
                {sending ? '送信中...' : '友達申請を送る'}
              </button>
            </div>
          </div>
        </div>
      )}
      {contacts.length === 0 && groups.length === 0 ? (
        <div className="contact-list-empty">
          <p>連絡先がありません</p>
          <p className="hint">友達申請を送るか、相手から申請が届くとここに表示されます</p>
        </div>
      ) : (
        <div className="contact-list-items">
          {contacts.map((contact) => {
            const conv: Conversation = { type: 'contact', contact };
            return (
              <div
                key={contact.did}
                className={`contact-item ${isActiveConversation(currentConversation, conv) ? 'active' : ''}`}
                onClick={() => onSelect(conv)}
              >
                <div className="contact-avatar">
                  {contact.name?.[0] || contact.did[10]}
                </div>
                <div className="contact-info">
                  <div className="contact-name">
                    {contact.name || contact.did.substring(0, 16) + '...'}
                  </div>
                  {contact.lastMessage && (
                    <div className="contact-last-message">{contact.lastMessage}</div>
                  )}
                </div>
                {contact.lastMessageTime && (
                  <div className="contact-time">
                    {contact.lastMessageTime.toLocaleTimeString('ja-JP', {
                      hour: '2-digit',
                      minute: '2-digit',
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
