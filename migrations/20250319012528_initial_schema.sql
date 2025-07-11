-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    password VARCHAR(255),
    oauth_provider VARCHAR(255),
    oauth_id VARCHAR(255),
    UNIQUE(oauth_provider, oauth_id)
);
-- +goose StatementEnd
