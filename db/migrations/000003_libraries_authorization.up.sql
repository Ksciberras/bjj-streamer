CREATE TABLE libraries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type TEXT NOT NULL CHECK (type IN ('personal', 'shared')),
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    owner_user_id UUID REFERENCES users(id),
    created_by UUID NOT NULL REFERENCES users(id),
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((type = 'personal' AND owner_user_id IS NOT NULL) OR (type = 'shared' AND owner_user_id IS NULL)),
    UNIQUE (owner_user_id)
);

INSERT INTO libraries (type, name, owner_user_id, created_by, created_at, updated_at)
SELECT 'personal', 'Personal library', id, id, created_at, created_at FROM users;

CREATE FUNCTION create_personal_library_for_user() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO libraries (type, name, owner_user_id, created_by)
    VALUES ('personal', 'Personal library', NEW.id, NEW.id);
    RETURN NEW;
END;
$$;

CREATE TRIGGER users_create_personal_library
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION create_personal_library_for_user();

CREATE FUNCTION protect_library_identity() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.type <> OLD.type OR NEW.owner_user_id IS DISTINCT FROM OLD.owner_user_id THEN
        RAISE EXCEPTION 'library type and owner are immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER libraries_protect_identity
BEFORE UPDATE ON libraries
FOR EACH ROW EXECUTE FUNCTION protect_library_identity();

CREATE TABLE library_members (
    library_id UUID NOT NULL REFERENCES libraries(id),
    user_id UUID NOT NULL REFERENCES users(id),
    access_level TEXT NOT NULL CHECK (access_level IN ('student', 'instructor')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (library_id, user_id)
);

CREATE INDEX library_members_user_id_idx ON library_members (user_id);

CREATE FUNCTION validate_library_membership() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    library_type TEXT;
    target_role TEXT;
    target_disabled TIMESTAMPTZ;
BEGIN
    SELECT type INTO library_type FROM libraries WHERE id = NEW.library_id;
    IF library_type IS DISTINCT FROM 'shared' THEN
        RAISE EXCEPTION 'memberships require a shared library' USING ERRCODE = '23514';
    END IF;
    SELECT role, disabled_at INTO target_role, target_disabled FROM users WHERE id = NEW.user_id;
    IF target_disabled IS NOT NULL THEN
        RAISE EXCEPTION 'disabled users cannot receive memberships' USING ERRCODE = '23514';
    END IF;
    IF NEW.access_level = 'instructor' AND target_role NOT IN ('instructor', 'admin') THEN
        RAISE EXCEPTION 'instructor membership requires an eligible global role' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER library_members_validate
BEFORE INSERT OR UPDATE ON library_members
FOR EACH ROW EXECUTE FUNCTION validate_library_membership();

CREATE TABLE audit_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id UUID NOT NULL REFERENCES users(id),
    action TEXT NOT NULL CHECK (char_length(action) BETWEEN 1 AND 80),
    target_type TEXT NOT NULL CHECK (char_length(target_type) BETWEEN 1 AND 80),
    target_id UUID NOT NULL,
    request_id TEXT NOT NULL CHECK (char_length(request_id) BETWEEN 1 AND 128),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (jsonb_typeof(details) = 'object'),
    CHECK (octet_length(details::text) <= 8192)
);

CREATE INDEX audit_events_occurred_at_idx ON audit_events (occurred_at DESC, id DESC);
CREATE INDEX audit_events_target_idx ON audit_events (target_type, target_id);

CREATE FUNCTION reject_audit_event_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit events are append-only' USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER audit_events_append_only
BEFORE UPDATE OR DELETE ON audit_events
FOR EACH ROW EXECUTE FUNCTION reject_audit_event_mutation();

CREATE FUNCTION assert_content_basis_for_library(target_library_id UUID, content_basis TEXT) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    library_type TEXT;
BEGIN
    IF content_basis NOT IN ('self_created', 'licensed_for_group', 'personal_purchase') THEN
        RAISE EXCEPTION 'invalid content basis' USING ERRCODE = '23514';
    END IF;
    SELECT type INTO STRICT library_type FROM libraries WHERE id = target_library_id;
    IF content_basis = 'personal_purchase' AND library_type <> 'personal' THEN
        RAISE EXCEPTION 'personal purchases require a personal library' USING ERRCODE = '23514';
    END IF;
END;
$$;

