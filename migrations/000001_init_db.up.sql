CREATE EXTENSION IF NOT EXISTS CITEXT;

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email CITEXT NOT NULL UNIQUE,
    password_hash BYTEA NOT NULL,
    activated BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP(0) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO permissions (name) 
VALUES
    ('user:read'),
    ('user:write');

CREATE TABLE IF NOT EXISTS user_permissions (
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_id INT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, permission_id)
);

CREATE TABLE IF NOT EXISTS scopes (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO scopes (name)
VALUES
    ('token:access'),
    ('token:refresh'),
    ('token:activate'),
    ('token:resetpwd');

CREATE TABLE IF NOT EXISTS tokens (
    hash BYTEA,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expiry TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    scope_id INT NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    PRIMARY KEY(user_id, scope_id)
);
