package auth

import "testing"

func TestPasswordHashAndVerification(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse battery staple" || !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("valid password did not verify securely")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("wrong password verified")
	}
}

func TestPasswordValidation(t *testing.T) {
	if ValidatePassword("short") == nil {
		t.Fatal("short password accepted")
	}
	if ValidatePassword("long-enough-password") != nil {
		t.Fatal("valid password rejected")
	}
}
