CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    password TEXT,
    oauth_provider TEXT,
    oauth_id TEXT,
    UNIQUE(oauth_provider, oauth_id)
);

