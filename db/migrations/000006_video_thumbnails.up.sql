ALTER TABLE videos
    ADD COLUMN thumbnail_object_key TEXT
    CHECK (
        thumbnail_object_key IS NULL
        OR char_length(thumbnail_object_key) BETWEEN 1 AND 1024
    );

CREATE UNIQUE INDEX videos_thumbnail_object_key_idx
    ON videos (thumbnail_object_key)
    WHERE thumbnail_object_key IS NOT NULL;
