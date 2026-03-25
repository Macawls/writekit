-- Linked accounts (many-to-one: providers → user)
CREATE TABLE linked_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    email TEXT NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_id)
);

CREATE INDEX idx_linked_accounts_user_id ON linked_accounts(user_id);
CREATE INDEX idx_linked_accounts_email ON linked_accounts(email);

-- Magic link tokens
CREATE TABLE magic_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_magic_links_token ON magic_links(token);

-- Migrate existing OAuth data into linked_accounts
INSERT INTO linked_accounts (user_id, provider, provider_id, email, email_verified)
SELECT id, oauth_provider, oauth_id, email,
    CASE WHEN oauth_provider IN ('google', 'github') THEN TRUE ELSE FALSE END
FROM users;

-- Remove OAuth columns from users
ALTER TABLE users DROP CONSTRAINT users_oauth_provider_oauth_id_key;
ALTER TABLE users DROP COLUMN oauth_provider;
ALTER TABLE users DROP COLUMN oauth_id;
