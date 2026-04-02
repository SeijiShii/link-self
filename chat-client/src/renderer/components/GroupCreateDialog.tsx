import { useState, useCallback } from 'react';
import type { Contact } from '../types';

interface GroupCreateDialogProps {
  contacts: Contact[];
  onClose: () => void;
  onCreate: (name: string, memberDIDs: string[]) => void;
}

export default function GroupCreateDialog({ contacts, onClose, onCreate }: GroupCreateDialogProps) {
  const [name, setName] = useState('');
  const [selectedDIDs, setSelectedDIDs] = useState<Set<string>>(new Set());

  const toggleMember = useCallback((did: string) => {
    setSelectedDIDs((prev) => {
      const next = new Set(prev);
      if (next.has(did)) {
        next.delete(did);
      } else {
        next.add(did);
      }
      return next;
    });
  }, []);

  const handleCreate = () => {
    if (selectedDIDs.size === 0) return;
    onCreate(name.trim() || `Group (${selectedDIDs.size + 1})`, Array.from(selectedDIDs));
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-dialog group-create-dialog" onClick={(e) => e.stopPropagation()}>
        <h3>Create Group</h3>
        <input
          type="text"
          className="group-name-input"
          placeholder="Group name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          autoFocus
        />
        <div className="group-member-list">
          <p className="group-member-label">Select members:</p>
          {contacts.length === 0 && (
            <p className="group-member-empty">No contacts yet</p>
          )}
          {contacts.map((c) => (
            <label key={c.did} className="group-member-item">
              <input
                type="checkbox"
                checked={selectedDIDs.has(c.did)}
                onChange={() => toggleMember(c.did)}
              />
              <span>{c.name || c.did.substring(0, 20) + '...'}</span>
            </label>
          ))}
        </div>
        <div className="group-create-actions">
          <button type="button" onClick={onClose} className="btn-cancel">Cancel</button>
          <button
            type="button"
            onClick={handleCreate}
            className="btn-create"
            disabled={selectedDIDs.size === 0}
          >
            Create ({selectedDIDs.size} members)
          </button>
        </div>
      </div>
    </div>
  );
}
