CREATE TABLE images (
    id TEXT PRIMARY KEY,
    bytes BLOB NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    size_bytes INTEGER NOT NULL,
    frame_count INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX images_created_at ON images(created_at DESC);

CREATE TABLE image_refs (
    image_id TEXT NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    page_id  TEXT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    PRIMARY KEY (image_id, page_id)
);

CREATE INDEX image_refs_page ON image_refs(page_id);
