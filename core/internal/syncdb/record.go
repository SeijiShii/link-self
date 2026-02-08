// Package syncdb implements the sync layer: meta attachment, immediate group delivery, last-write-wins apply.
// Built on top of group concept. See docs/sync-db-plan.md.
package syncdb

// SyncRecord is the unit of sync: meta plus app payload. Used for conflict resolution and delivery.
type SyncRecord struct {
	ID        string `json:"id"`        // unique; used for conflict resolution
	GroupID   string `json:"groupId"`   // target group for delivery
	DID       string `json:"did"`       // writer's DID
	Timestamp int64  `json:"timestamp"` // milliseconds; for last-write-wins
	Body      []byte `json:"body"`     // app payload (e.g. JSON)
	Deleted   bool   `json:"deleted"`   // if true, treat as delete (apply as Delete on remote)
}
