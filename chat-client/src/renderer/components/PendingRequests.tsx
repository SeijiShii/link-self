import type { FriendRequest } from '../types';

interface PendingRequestsProps {
  requests: FriendRequest[];
  onAccept: (fromDID: string) => void;
  onReject: (fromDID: string) => void;
}

export default function PendingRequests({ requests, onAccept, onReject }: PendingRequestsProps) {
  if (requests.length === 0) return null;

  return (
    <div className="pending-requests">
      <div className="pending-requests-header">
        <h3>友達申請</h3>
      </div>
      <ul className="pending-requests-list">
        {requests.map((req) => (
          <li key={req.fromDID} className="pending-request-item">
            <span className="pending-request-did" title={req.fromDID}>
              {req.fromDID.substring(0, 20)}...
            </span>
            <div className="pending-request-actions">
              <button type="button" className="btn-accept" onClick={() => onAccept(req.fromDID)}>
                承認
              </button>
              <button type="button" className="btn-reject" onClick={() => onReject(req.fromDID)}>
                拒否
              </button>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
