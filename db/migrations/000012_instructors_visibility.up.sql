ALTER TABLE videos DROP CONSTRAINT videos_visibility_check;
ALTER TABLE videos ADD CONSTRAINT videos_visibility_check CHECK (visibility IN ('shared', 'private', 'instructors'));
