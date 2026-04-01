ALTER TABLE pages ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public'
    CHECK(visibility IN ('public', 'unlisted', 'private'));

ALTER TABLE collections ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public'
    CHECK(visibility IN ('public', 'unlisted', 'private'));
