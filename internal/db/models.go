package db

import (
	"database/sql"
	"errors"
)

var (
	ErrNotFound = errors.New("not found")
)

type Models struct {
	Users       UserModel
	Permissions PermissionModel
	Tokens      TokenModel
	DB          *sql.DB
}

func NewModels(db *sql.DB) *Models {
	return &Models{
		Users:       UserModel{DB: db},
		Permissions: PermissionModel{DB: db},
		Tokens:      TokenModel{DB: db},
		DB:          db,
	}
}
