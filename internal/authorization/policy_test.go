package authorization

import "testing"

func TestLibraryVisibilityAndEnumeration(t *testing.T) {
	p := Policy{}
	admin := Actor{ID: "admin", Role: Admin}
	instructor := Actor{ID: "instructor", Role: Instructor}
	student := Actor{ID: "student", Role: Student}
	personal := Library{ID: "p", Type: Personal, OwnerID: "student"}
	shared := Library{ID: "s", Type: Shared}
	member := StudentMember
	tests := []struct {
		name       string
		actor      Actor
		library    Library
		membership *MembershipLevel
		want       bool
	}{
		{"personal owner", student, personal, nil, true}, {"admin cannot inspect personal", admin, personal, nil, false}, {"other cannot inspect personal", instructor, personal, nil, false},
		{"shared member", student, shared, &member, true}, {"admin needs shared membership", admin, shared, nil, false}, {"unassigned instructor", instructor, shared, nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := p.ViewLibrary(test.actor, test.library, test.membership); got != test.want {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
}

func TestUploadRequiresRoleAssignmentAndOwnership(t *testing.T) {
	p := Policy{}
	shared := Library{Type: Shared}
	personal := Library{Type: Personal, OwnerID: "i"}
	studentLevel := StudentMember
	instructorLevel := InstructorMember
	instructor := Actor{ID: "i", Role: Instructor}
	admin := Actor{ID: "a", Role: Admin}
	student := Actor{ID: "s", Role: Student}
	tests := []struct {
		name       string
		actor      Actor
		library    Library
		membership *MembershipLevel
		want       bool
	}{
		{"instructor assigned", instructor, shared, &instructorLevel, true}, {"instructor reader", instructor, shared, &studentLevel, false}, {"instructor unassigned", instructor, shared, nil, false},
		{"admin member", admin, shared, &studentLevel, true}, {"admin unassigned", admin, shared, nil, false}, {"student instructor membership", student, shared, &instructorLevel, false},
		{"instructor own personal", instructor, personal, nil, true}, {"student own personal", student, Library{Type: Personal, OwnerID: "s"}, nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := p.Upload(test.actor, test.library, test.membership); got != test.want {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
	if p.MutateContent(instructor, shared, &instructorLevel, Resource{OwnerID: "other"}) {
		t.Fatal("instructor edited another owner's content")
	}
	if !p.MutateContent(instructor, shared, &instructorLevel, Resource{OwnerID: "i"}) {
		t.Fatal("instructor could not edit owned content")
	}
}

func TestContentBasisPlacement(t *testing.T) {
	p := Policy{}
	actor := Actor{ID: "i", Role: Instructor}
	personal := Library{Type: Personal, OwnerID: "i"}
	shared := Library{Type: Shared}
	assigned := InstructorMember
	if !p.PlaceContent(actor, personal, nil, PersonalPurchase) {
		t.Fatal("personal purchase denied in own personal library")
	}
	if p.PlaceContent(actor, shared, &assigned, PersonalPurchase) {
		t.Fatal("personal purchase allowed in shared library")
	}
	if !p.PlaceContent(actor, shared, &assigned, LicensedForGroup) {
		t.Fatal("licensed content denied")
	}
	if p.PlaceContent(actor, personal, nil, ContentBasis("invalid")) {
		t.Fatal("invalid basis allowed")
	}
}

func TestManagementAndAssignment(t *testing.T) {
	p := Policy{}
	admin := Actor{ID: "a", Role: Admin}
	shared := Library{Type: Shared}
	personal := Library{Type: Personal, OwnerID: "a"}
	if !p.ManageUsers(admin) || !p.CreateSharedLibrary(admin) || !p.ManageMembership(admin, shared) {
		t.Fatal("admin management denied")
	}
	if p.ManageMembership(admin, personal) {
		t.Fatal("personal membership management allowed")
	}
	if p.AssignMembership(admin, shared, Actor{Role: Student}, InstructorMember) {
		t.Fatal("student assigned instructor membership")
	}
	if !p.AssignMembership(admin, shared, Actor{Role: Instructor}, InstructorMember) {
		t.Fatal("instructor assignment denied")
	}
	disabled := admin
	disabled.Disabled = true
	if p.ManageUsers(disabled) {
		t.Fatal("disabled admin allowed")
	}
}

func TestVideoVisibilityAndOwnership(t *testing.T) {
	p := Policy{}
	admin := Actor{ID: "admin", Role: Admin}
	instructor := Actor{ID: "instructor", Role: Instructor}
	other := Actor{ID: "other", Role: Instructor}
	student := Actor{ID: "student", Role: Student}
	shared := Video{UploaderID: instructor.ID, Visibility: SharedVideo, Ready: true}
	private := Video{UploaderID: instructor.ID, Visibility: PrivateVideo, Ready: true}
	if !p.CreateVideo(admin) || !p.CreateVideo(instructor) || p.CreateVideo(student) {
		t.Fatal("video creation role policy failed")
	}
	if !p.ViewVideo(student, shared) || p.ViewVideo(student, private) || !p.ViewVideo(admin, private) || !p.ViewVideo(instructor, private) {
		t.Fatal("video visibility policy failed")
	}
	if !p.ManageVideo(admin, private) || !p.ManageVideo(instructor, private) || p.ManageVideo(other, private) || p.ManageVideo(student, shared) {
		t.Fatal("video management policy failed")
	}
	shared.Ready = false
	if p.ViewVideo(student, shared) {
		t.Fatal("pending video was visible")
	}
}
