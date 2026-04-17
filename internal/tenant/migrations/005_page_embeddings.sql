CREATE TABLE IF NOT EXISTS page_embeddings (
    page_id TEXT PRIMARY KEY REFERENCES pages(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    dims INTEGER NOT NULL,
    vec BLOB NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_page_embeddings_model ON page_embeddings(model, dims);
