package sqlite

import (
	"database/sql"
	"encoding/json"

	"github.com/SeijiShii/link-self/core/internal/network"
	"github.com/google/uuid"
)

type networkStore struct {
	db *sql.DB
}

func (s *networkStore) CreateNetwork(n *network.Network) (string, error) {
	id := uuid.New().String()
	members, _ := json.Marshal(n.Members)
	roles, _ := json.Marshal(n.MemberRoles)
	_, err := s.db.Exec(
		`INSERT INTO networks (id, suite_id, members, member_roles) VALUES (?, ?, ?, ?)`,
		id, n.SuiteID, members, roles)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *networkStore) GetNetwork(id string) (*network.Network, error) {
	var n network.Network
	var members, roles []byte
	err := s.db.QueryRow(
		`SELECT id, suite_id, members, member_roles FROM networks WHERE id = ?`, id,
	).Scan(&n.ID, &n.SuiteID, &members, &roles)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(members, &n.Members)
	json.Unmarshal(roles, &n.MemberRoles)
	return &n, nil
}

func (s *networkStore) UpdateNetwork(id string, n *network.Network) error {
	members, _ := json.Marshal(n.Members)
	roles, _ := json.Marshal(n.MemberRoles)
	res, err := s.db.Exec(
		`UPDATE networks SET suite_id = ?, members = ?, member_roles = ? WHERE id = ?`,
		n.SuiteID, members, roles, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return network.ErrNetworkNotFound
	}
	return nil
}

func (s *networkStore) DeleteNetwork(id string) error {
	_, err := s.db.Exec(`DELETE FROM networks WHERE id = ?`, id)
	return err
}

func (s *networkStore) ListForMember(memberDID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT id, members FROM networks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		var membersJSON []byte
		if err := rows.Scan(&id, &membersJSON); err != nil {
			return nil, err
		}
		var members []string
		json.Unmarshal(membersJSON, &members)
		for _, m := range members {
			if m == memberDID {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids, rows.Err()
}
