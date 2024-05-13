package db

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

func TestPermissionModel_Add(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := PermissionModel{DB: db}

	query := regexp.QuoteMeta(`
		INSERT INTO user_permissions
		SELECT $1, permissions.id FROM permissions WHERE permissions.name = ANY($2)`)

	mock.ExpectExec(query).WithArgs(1, pq.Array([]string{"user:read", "user:write"})).WillReturnResult(sqlmock.NewResult(1, 0))

	err := m.Add(1, PermissionReadUser, PermissionWriteUser)
	if err != nil {
		t.Errorf("failed to add permissions: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPermissionModel_Get(t *testing.T) {
	db, mock := MockDB()
	defer db.Close()

	m := PermissionModel{DB: db}

	query := regexp.QuoteMeta(`
		SELECT permissions.name
		FROM permissions
		INNER JOIN user_permissions ON permissions.id = user_permissions.permission_id
		INNER JOIN users ON user_permissions.user_id = users.id
		WHERE users.id = $1`)

	rows := sqlmock.NewRows([]string{"name"}).
		AddRow("user:read").
		AddRow("user:write")

	mock.ExpectQuery(query).WithArgs(1).WillReturnRows(rows)

	permissions, err := m.Get(1)
	if err != nil {
		t.Errorf("failed to get permissions: %v", err)
	}

	if len(*permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(*permissions))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
