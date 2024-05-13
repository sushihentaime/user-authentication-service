package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
)

type Permission string
type Permissions []Permission

const (
	PermissionReadUser  Permission = "user:read"
	PermissionWriteUser Permission = "user:write"
)

type PermissionModel struct {
	DB *sql.DB
}

func (m *PermissionModel) Add(userID int, permissions ...Permission) error {
	query := `
		INSERT INTO user_permissions
		SELECT $1, permissions.id FROM permissions WHERE permissions.name = ANY($2)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID, pq.Array(permissions))
	if err != nil {
		return err
	}
	return nil
}

func (m *PermissionModel) Get(userID int) (*Permissions, error) {
	query := `
		SELECT permissions.name
		FROM permissions
		INNER JOIN user_permissions ON permissions.id = user_permissions.permission_id
		INNER JOIN users ON user_permissions.user_id = users.id
		WHERE users.id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions
	for rows.Next() {
		var permission Permission
		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &permissions, nil
}

func (p *Permissions) Include(permission Permission) bool {
	for _, p := range *p {
		if p == permission {
			return true
		}
	}
	return false
}
