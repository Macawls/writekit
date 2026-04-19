DROP TRIGGER IF EXISTS pages_ai;
DROP TRIGGER IF EXISTS pages_ad;
DROP TRIGGER IF EXISTS pages_au;
DROP TABLE IF EXISTS pages_fts;

CREATE VIRTUAL TABLE pages_fts USING fts5(
    title,
    content,
    tags,
    content=pages,
    content_rowid=rowid,
    tokenize = 'porter unicode61 remove_diacritics 2'
);

CREATE TRIGGER pages_ai AFTER INSERT ON pages BEGIN
    INSERT INTO pages_fts(rowid, title, content, tags) VALUES (new.rowid, new.title, new.content, new.tags);
END;

CREATE TRIGGER pages_ad AFTER DELETE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, content, tags) VALUES('delete', old.rowid, old.title, old.content, old.tags);
END;

CREATE TRIGGER pages_au AFTER UPDATE ON pages BEGIN
    INSERT INTO pages_fts(pages_fts, rowid, title, content, tags) VALUES('delete', old.rowid, old.title, old.content, old.tags);
    INSERT INTO pages_fts(rowid, title, content, tags) VALUES (new.rowid, new.title, new.content, new.tags);
END;

INSERT INTO pages_fts(rowid, title, content, tags)
    SELECT rowid, title, content, tags FROM pages;
