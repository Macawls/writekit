CREATE TABLE team_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON UPDATE CASCADE ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('owner', 'editor', 'viewer')),
    token TEXT NOT NULL UNIQUE,
    invited_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_team_invitations_pending
    ON team_invitations(tenant_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

CREATE INDEX idx_team_invitations_email ON team_invitations(email);
