package password

import (
	"strings"
	"testing"
)

func TestHashVerifyRoundtrip(t *testing.T) {
	h := NewHasher(DefaultParams())
	phc, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(phc, "$argon2id$v=19$") {
		t.Fatalf("not PHC argon2id: %s", phc)
	}
	ok, err := h.Verify("correct horse battery staple", phc)
	if err != nil || !ok {
		t.Fatalf("verify failed: ok=%v err=%v", ok, err)
	}
	ok, _ = h.Verify("wrong password", phc)
	if ok {
		t.Fatal("wrong password verified")
	}
}

func TestDistinctSalts(t *testing.T) {
	h := NewHasher(DefaultParams())
	a, _ := h.Hash("same input")
	b, _ := h.Hash("same input")
	if a == b {
		t.Fatal("two hashes of the same input are identical — salt broken")
	}
}

func TestMalformedHash(t *testing.T) {
	h := NewHasher(DefaultParams())
	for _, bad := range []string{"", "plaintext", "$argon2i$v=19$m=1,t=1,p=1$AA$AA", "$argon2id$v=18$m=1,t=1,p=1$AA$AA"} {
		if _, err := h.Verify("x", bad); err == nil {
			t.Fatalf("malformed hash accepted: %q", bad)
		}
	}
}

// Hashes verify with their STORED parameters even if the hasher's differ.
func TestParameterUpgradeCompatibility(t *testing.T) {
	old := NewHasher(Params{MemoryKiB: 8192, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32})
	phc, _ := old.Hash("legacy password!")
	current := NewHasher(DefaultParams())
	ok, err := current.Verify("legacy password!", phc)
	if err != nil || !ok {
		t.Fatalf("stored-parameter verify failed: ok=%v err=%v", ok, err)
	}
}
