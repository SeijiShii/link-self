import type { Contact } from '../types';

interface ContactListProps {
  contacts: Contact[];
  currentContact: Contact | null;
  onSelect: (contact: Contact) => void;
}

export default function ContactList({ contacts, currentContact, onSelect }: ContactListProps) {
  if (contacts.length === 0) {
    return (
      <div className="contact-list">
        <div className="contact-list-header">
          <h2>連絡先</h2>
        </div>
        <div className="contact-list-empty">
          <p>連絡先がありません</p>
          <p className="hint">DIDを追加してチャットを開始してください</p>
        </div>
      </div>
    );
  }

  return (
    <div className="contact-list">
      <div className="contact-list-header">
        <h2>連絡先</h2>
      </div>
      <div className="contact-list-items">
        {contacts.map((contact) => (
          <div
            key={contact.did}
            className={`contact-item ${currentContact?.did === contact.did ? 'active' : ''}`}
            onClick={() => onSelect(contact)}
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
        ))}
      </div>
    </div>
  );
}
