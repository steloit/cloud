// Package ids mints the prefixed opaque identifiers of the API contract
// (x-conventions: usr_, ses_, …). Never expose raw UUIDs.
package ids

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns "<prefix>_<32 hex chars>" — 16 random bytes.
func New(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
