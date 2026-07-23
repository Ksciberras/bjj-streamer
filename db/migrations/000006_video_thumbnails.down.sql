DROP INDEX IF EXISTS videos_thumbnail_object_key_idx;

ALTER TABLE videos DROP COLUMN IF EXISTS thumbnail_object_key;
