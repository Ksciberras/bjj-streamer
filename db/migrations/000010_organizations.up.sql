DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM users) AND NOT EXISTS (SELECT 1 FROM users WHERE email='kyranu2@gmail.com') THEN
    RAISE EXCEPTION 'missing platform owner account';
  END IF;
  IF EXISTS (SELECT 1 FROM users) AND NOT EXISTS (SELECT 1 FROM users WHERE email='info@bjjcork.com') THEN
    RAISE EXCEPTION 'missing BJJ Cork admin account';
  END IF;
END $$;

CREATE TABLE organizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  slug TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO organizations(name,slug) VALUES('BJJ Cork','bjj-cork');

ALTER TABLE users ADD COLUMN organization_id UUID REFERENCES organizations(id), ADD COLUMN is_platform_owner BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE users SET organization_id=(SELECT id FROM organizations WHERE slug='bjj-cork');
UPDATE users SET organization_id=NULL,is_platform_owner=TRUE,role='admin' WHERE email='kyranu2@gmail.com';
UPDATE users SET role='admin' WHERE email='info@bjjcork.com';
ALTER TABLE users ADD CONSTRAINT users_organization_required CHECK ((is_platform_owner AND organization_id IS NULL AND role='admin') OR (NOT is_platform_owner AND organization_id IS NOT NULL));
CREATE FUNCTION assign_default_organization() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NOT NEW.is_platform_owner AND NEW.organization_id IS NULL THEN
    SELECT id INTO NEW.organization_id FROM organizations ORDER BY created_at,id LIMIT 1;
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER users_assign_default_organization BEFORE INSERT ON users FOR EACH ROW EXECUTE FUNCTION assign_default_organization();
CREATE INDEX users_organization_idx ON users(organization_id,created_at);

ALTER TABLE videos ADD COLUMN organization_id UUID REFERENCES organizations(id);
UPDATE videos v SET organization_id=COALESCE(u.organization_id,(SELECT id FROM organizations WHERE slug='bjj-cork')) FROM users u WHERE u.id=v.uploaded_by_user_id;
ALTER TABLE videos ALTER COLUMN organization_id SET NOT NULL;
CREATE FUNCTION assign_video_organization() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.organization_id IS NULL THEN SELECT organization_id INTO NEW.organization_id FROM users WHERE id=NEW.uploaded_by_user_id; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER videos_assign_organization BEFORE INSERT ON videos FOR EACH ROW EXECUTE FUNCTION assign_video_organization();
CREATE TABLE video_organizations(video_id UUID REFERENCES videos(id) ON DELETE CASCADE,organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(video_id,organization_id));
INSERT INTO video_organizations SELECT id,organization_id,CURRENT_TIMESTAMP FROM videos;
CREATE FUNCTION share_video_with_owner_organization() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO video_organizations(video_id,organization_id) VALUES(NEW.id,NEW.organization_id) ON CONFLICT DO NOTHING;
  RETURN NEW;
END $$;
CREATE TRIGGER videos_share_with_owner AFTER INSERT ON videos FOR EACH ROW EXECUTE FUNCTION share_video_with_owner_organization();

ALTER TABLE courses ADD COLUMN organization_id UUID REFERENCES organizations(id);
UPDATE courses c SET organization_id=COALESCE(u.organization_id,(SELECT id FROM organizations WHERE slug='bjj-cork')) FROM users u WHERE u.id=c.created_by_user_id;
ALTER TABLE courses ALTER COLUMN organization_id SET NOT NULL;
CREATE TABLE course_organizations(course_id UUID REFERENCES courses(id) ON DELETE CASCADE,organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(course_id,organization_id));
INSERT INTO course_organizations SELECT id,organization_id,CURRENT_TIMESTAMP FROM courses;
