DROP FUNCTION assert_content_basis_for_library(UUID, TEXT);
DROP TRIGGER audit_events_append_only ON audit_events;
DROP FUNCTION reject_audit_event_mutation();
DROP TABLE audit_events;
DROP TRIGGER library_members_validate ON library_members;
DROP FUNCTION validate_library_membership();
DROP TABLE library_members;
DROP TRIGGER libraries_protect_identity ON libraries;
DROP FUNCTION protect_library_identity();
DROP TRIGGER users_create_personal_library ON users;
DROP FUNCTION create_personal_library_for_user();
DROP TABLE libraries;

