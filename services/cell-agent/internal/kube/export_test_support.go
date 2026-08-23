package kube

// SetSADirForTest points the in-cluster ServiceAccount path at a directory and
// returns a restore func. Exported (not an _test.go helper) because the arm it
// unlocks is selected in cmd/cell-agent, a different package — and that arm is
// the only one that runs on a real cell.
//
// It deliberately does NOT take *testing.T: that would put the testing package
// into the production binary's import graph to close a test gap. Callers use
// t.Cleanup(kube.SetSADirForTest(dir)).
func SetSADirForTest(dir string) (restore func()) {
	prev := saDir
	saDir = dir
	return func() { saDir = prev }
}
