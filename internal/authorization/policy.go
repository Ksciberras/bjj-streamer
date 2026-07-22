package authorization

type Role string
type LibraryType string
type MembershipLevel string
type ContentBasis string
type Visibility string

const (
	Admin            Role            = "admin"
	Instructor       Role            = "instructor"
	Student          Role            = "student"
	Personal         LibraryType     = "personal"
	Shared           LibraryType     = "shared"
	StudentMember    MembershipLevel = "student"
	InstructorMember MembershipLevel = "instructor"
	SelfCreated      ContentBasis    = "self_created"
	LicensedForGroup ContentBasis    = "licensed_for_group"
	PersonalPurchase ContentBasis    = "personal_purchase"
	SharedVideo      Visibility      = "shared"
	PrivateVideo     Visibility      = "private"
)

type Actor struct {
	ID       string
	Role     Role
	Disabled bool
}
type Library struct {
	ID       string
	Type     LibraryType
	OwnerID  string
	Archived bool
}
type Resource struct {
	OwnerID   string
	Published bool
}
type Video struct {
	UploaderID string
	Visibility Visibility
	Ready      bool
}

type Policy struct{}

func (Policy) ManageUsers(actor Actor) bool         { return activeAdmin(actor) }
func (Policy) ViewAudit(actor Actor) bool           { return activeAdmin(actor) }
func (Policy) CreateSharedLibrary(actor Actor) bool { return activeAdmin(actor) }
func (Policy) ManageSharedLibrary(actor Actor, library Library) bool {
	return activeAdmin(actor) && library.Type == Shared
}
func (Policy) ManageMembership(actor Actor, library Library) bool {
	return activeAdmin(actor) && library.Type == Shared && !library.Archived
}

func (Policy) ViewLibrary(actor Actor, library Library, membership *MembershipLevel) bool {
	if actor.Disabled {
		return false
	}
	if library.Type == Personal {
		return library.OwnerID == actor.ID
	}
	return library.Type == Shared && membership != nil
}

func (Policy) EditPersonalLibrary(actor Actor, library Library) bool {
	return !actor.Disabled && library.Type == Personal && library.OwnerID == actor.ID
}

func (Policy) AssignMembership(actor Actor, library Library, target Actor, level MembershipLevel) bool {
	if !(Policy{}).ManageMembership(actor, library) || target.Disabled {
		return false
	}
	if level == StudentMember {
		return true
	}
	return level == InstructorMember && (target.Role == Instructor || target.Role == Admin)
}

func (Policy) ViewMembers(actor Actor, library Library, membership *MembershipLevel) bool {
	if actor.Disabled || library.Type != Shared {
		return false
	}
	return actor.Role == Admin || membership != nil
}

func (Policy) Upload(actor Actor, library Library, membership *MembershipLevel) bool {
	if actor.Disabled || library.Archived || actor.Role == Student {
		return false
	}
	if library.Type == Personal {
		return library.OwnerID == actor.ID
	}
	if membership == nil {
		return false
	}
	return actor.Role == Admin || (actor.Role == Instructor && *membership == InstructorMember)
}

func (Policy) MutateContent(actor Actor, library Library, membership *MembershipLevel, resource Resource) bool {
	if !(Policy{}).Upload(actor, library, membership) {
		return false
	}
	if actor.Role == Admin {
		return true
	}
	return resource.OwnerID == actor.ID
}

func (Policy) ViewContent(actor Actor, library Library, membership *MembershipLevel, resource Resource) bool {
	if !(Policy{}).ViewLibrary(actor, library, membership) {
		return false
	}
	if resource.Published {
		return true
	}
	if actor.Role == Student {
		return false
	}
	return actor.Role == Admin || resource.OwnerID == actor.ID
}

func (Policy) PlaceContent(actor Actor, library Library, membership *MembershipLevel, basis ContentBasis) bool {
	if basis != SelfCreated && basis != LicensedForGroup && basis != PersonalPurchase {
		return false
	}
	if basis == PersonalPurchase && library.Type != Personal {
		return false
	}
	return (Policy{}).Upload(actor, library, membership)
}

func (Policy) CreateVideo(actor Actor) bool {
	return !actor.Disabled && (actor.Role == Admin || actor.Role == Instructor)
}

func (Policy) ManageVideo(actor Actor, video Video) bool {
	return !actor.Disabled && (actor.Role == Admin || (actor.Role == Instructor && video.UploaderID == actor.ID))
}

func (Policy) ViewVideo(actor Actor, video Video) bool {
	if actor.Disabled || !video.Ready {
		return false
	}
	return video.Visibility == SharedVideo || actor.Role == Admin || video.UploaderID == actor.ID
}

func activeAdmin(actor Actor) bool { return !actor.Disabled && actor.Role == Admin }
