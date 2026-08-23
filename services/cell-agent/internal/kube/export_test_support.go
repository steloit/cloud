package kube

import "testing"

// SetSADirForTest points the in-cluster ServiceAccount path at a temp dir and
// restores it afterwards. Exported (not an _test.go helper) because the arm it
// unlocks is selected in cmd/cell-agent, a different package — and that arm is
// the only one that runs on a real cell.
func SetSADirForTest(t *testing.T, dir string) {
	t.Helper()
	prev := saDir
	saDir = dir
	t.Cleanup(func() { saDir = prev })
}
