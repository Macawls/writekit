ALTER TABLE pages ADD COLUMN search_text TEXT NOT NULL DEFAULT '';

DROP TRIGGER IF EXISTS pages_ai;
DROP TRIGGER IF EXISTS pages_ad;
DROP TRIGGER IF EXISTS pages_au;
DROP TABLE IF EXISTS pages_fts;

CREATE VIRTUAL TABLE pages_fts USING fts5(
    title,
    search_text,
    tags,
    content=pages,
    content_rowid=rowid,
    tokenize = 'porter unicode61 remove_diacritics 2'
);

CREATE TRIGGER pages_ai AFTER INSERT ON pages BEGIN
    INSERT INTO pages_fts(rowid, title, search_text, tags) VALUES (new.rowid, new.title, new.search_text, new.tags);
END;

CREATE TRIGGER pages_ad AFTER DELETE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, search_text, tags) VALUES('delete', old.rowid, old.title, old.search_text, old.tags);
END;

CREATE TRIGGER pages_au AFTER UPDATE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, search_text, tags) VALUES('delete', old.rowid, old.title, old.search_text, old.tags);
    INSERT INTO pages_fts(rowid, title, search_text, tags) VALUES (new.rowid, new.title, new.search_text, new.tags);
END;

INSERT INTO pages_fts(pages_fts) VALUES('rebuild');
