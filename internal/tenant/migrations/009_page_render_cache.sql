CREATE TABLE page_renders (
    page_id TEXT PRIMARY KEY REFERENCES pages(id) ON DELETE CASCADE,
    html BLOB NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
