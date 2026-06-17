package world

import "testing"

func TestHashAccountPasswordUsesBcrypt(t *testing.T) {
	password := "correct horse battery staple"

	hash, err := hashAccountPassword(password)
	if err != nil {
		t.Fatalf("hashAccountPassword failed: %v", err)
	}
	if hash == password {
		t.Fatal("hashAccountPassword returned plaintext")
	}
	if !isBcryptHash(hash) {
		t.Fatalf("expected bcrypt hash, got %q", hash)
	}
	if err := verifyAccountPassword(hash, password); err != nil {
		t.Fatalf("verifyAccountPassword rejected correct password: %v", err)
	}
	if err := verifyAccountPassword(hash, "wrong password"); err == nil {
		t.Fatal("verifyAccountPassword accepted wrong password")
	}
}

func TestIsBcryptHashRejectsPlaintext(t *testing.T) {
	if isBcryptHash("plaintext-password") {
		t.Fatal("isBcryptHash accepted plaintext")
	}
}
