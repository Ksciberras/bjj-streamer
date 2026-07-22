CREATE TABLE videos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uploaded_by_user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    instructor_name TEXT NOT NULL CHECK (char_length(btrim(instructor_name)) BETWEEN 1 AND 200),
    instructional_name TEXT CHECK (instructional_name IS NULL OR char_length(btrim(instructional_name)) BETWEEN 1 AND 200),
    chapter_name TEXT CHECK (chapter_name IS NULL OR char_length(btrim(chapter_name)) BETWEEN 1 AND 200),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 10000),
    tags TEXT[] NOT NULL DEFAULT '{}',
    visibility TEXT NOT NULL CHECK (visibility IN ('shared', 'private')),
    content_basis TEXT NOT NULL CHECK (content_basis IN ('self_created', 'licensed_for_group', 'personal_purchase')),
    object_key TEXT NOT NULL UNIQUE CHECK (char_length(object_key) BETWEEN 1 AND 1024),
    original_filename TEXT NOT NULL CHECK (char_length(original_filename) BETWEEN 1 AND 255),
    mime_type TEXT NOT NULL CHECK (mime_type = 'video/mp4'),
    byte_size BIGINT NOT NULL CHECK (byte_size > 0 AND byte_size <= 5368709120),
    status TEXT NOT NULL DEFAULT 'pending_upload' CHECK (status IN ('pending_upload', 'ready', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (content_basis <> 'personal_purchase' OR visibility = 'private')
);

CREATE INDEX videos_uploaded_by_idx ON videos (uploaded_by_user_id);
CREATE INDEX videos_catalog_idx ON videos (status, visibility, created_at DESC);
