CREATE TABLE IF NOT EXISTS profiles (
    user_id UUID PRIMARY KEY,
    full_name TEXT NOT NULL,
    phone TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
