CREATE TABLE courses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by_user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    instructor_name TEXT NOT NULL CHECK (char_length(instructor_name) BETWEEN 1 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE course_videos (
    course_id UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    video_id UUID NOT NULL REFERENCES videos(id),
    sequence_number INTEGER NOT NULL CHECK (sequence_number > 0),
    chapter_title TEXT CHECK (chapter_title IS NULL OR char_length(chapter_title) BETWEEN 1 AND 200),
    PRIMARY KEY (course_id, video_id),
    UNIQUE (course_id, sequence_number)
);

CREATE INDEX course_videos_video_id_idx ON course_videos(video_id);
CREATE INDEX courses_created_by_user_id_idx ON courses(created_by_user_id);
