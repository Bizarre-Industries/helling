package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPassword_Roundtrip(t *testing.T) {
	phc, err := HashPassword("correct-horse-battery-staple", DefaultArgon2idParams)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(phc, "$argon2id$v=19$") {
		t.Fatalf("expected PHC prefix, got %q", phc)
	}
	if err := VerifyPassword("correct-horse-battery-staple", phc); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyPassword_WrongPasswordRejected(t *testing.T) {
	phc, err := HashPassword("p4ss", DefaultArgon2idParams)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := VerifyPassword("nope", phc); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("want ErrPasswordMismatch, got %v", err)
	}
}

func TestVerifyPassword_MalformedHashRejected(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"wrong prefix":     "$argon2i$v=19$m=65536,t=3,p=4$YWJjZA$ZXhh",
		"bad version":      "$argon2id$v=42$m=65536,t=3,p=4$YWJjZA$ZXhh",
		"bad params":       "$argon2id$v=19$m=x,t=3,p=4$YWJjZA$ZXhh",
		"bad salt base64":  "$argon2id$v=19$m=65536,t=3,p=4$!!!!$ZXhh",
		"bad digest b64":   "$argon2id$v=19$m=65536,t=3,p=4$YWJjZA$!!!",
		"too few sections": "$argon2id$v=19",
	}
	for name, phc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := VerifyPassword("x", phc); !errors.Is(err, ErrMalformedHash) {
				t.Fatalf("want ErrMalformedHash, got %v", err)
			}
		})
	}
}

func TestHashPassword_RejectsEmpty(t *testing.T) {
	if _, err := HashPassword("", DefaultArgon2idParams); err == nil {
		t.Fatal("expected error on empty password")
	}
}

func TestHashPassword_Unique(t *testing.T) {
	a, err := HashPassword("same", DefaultArgon2idParams)
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("same", DefaultArgon2idParams)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("expected distinct hashes due to random salt")
	}
}
