CREATE TABLE tag_renders (
    slug TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    html BLOB NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tag_index_render (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    html BLOB NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
