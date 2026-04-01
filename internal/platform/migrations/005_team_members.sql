CREATE TABLE team_members (
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON UPDATE CASCADE ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'viewer' CHECK(role IN ('owner', 'editor', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, user_id)
);

CREATE INDEX idx_team_members_user_id ON team_members(user_id);

-- Seed existing tenant owners as team members
INSERT INTO team_members (tenant_id, user_id, role)
SELECT id, user_id, 'owner' FROM tenants
ON CONFLICT DO NOTHING;
