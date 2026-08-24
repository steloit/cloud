// Internal test: the scope classifier is unexported, and it is the one place
// that decides what a teardown deletes by name versus what dies with the
// namespace. Testing it through TeardownObjects cannot reach the inputs that
// matter, because Render only emits objects this classifier already agrees on.
package tenancy

import "testing"

// SCOPE IS READ FROM THE BYTES, AND THE PARSER MUST NOT BE FOOLED.
//
// TeardownObjects decides what to delete by asking whether an object declares
// `metadata.namespace`. A hardcoded `Kind == "Namespace"` list passes every
// test today, because Render emits exactly one cluster-scoped object — so the
// classifier is exercised directly here, on inputs Render does not yet produce
// but US-3.3c will.
func TestScopeIsReadFromTheBytesNotFromAKindList(t *testing.T) {
	for _, tc := range []struct {
		name       string
		yaml       string
		namespaced bool
	}{
		{"plain", "kind: X\nmetadata:\n  name: a\n  namespace: env-a\n", true},
		{"quoted", "kind: X\nmetadata:\n  name: a\n  namespace: \"env-a\"\n", true},
		{"single quoted", "kind: X\nmetadata:\n  name: a\n  namespace: 'env-a'\n", true},
		{"flow", "kind: X\nmetadata: {name: a, namespace: env-a}\n", true},
		{"absent", "kind: X\nmetadata:\n  name: a\n", false},
		{"empty string", "kind: X\nmetadata:\n  name: a\n  namespace: \"\"\n", false},
		{"null", "kind: X\nmetadata:\n  name: a\n  namespace: null\n", false},
		// A `namespace:` key somewhere OTHER than metadata must not be mistaken
		// for the object's own scope — a ClusterRoleBinding's subjects carry one.
		{"namespace outside metadata", "kind: X\nmetadata:\n  name: a\nsubjects:\n  - namespace: env-a\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := declaresNamespace([]byte(tc.yaml))
			if err != nil {
				t.Fatalf("classify: %v", err)
			}
			if got != tc.namespaced {
				t.Errorf("declaresNamespace = %v, want %v — a misread here either deletes a "+
					"namespaced object by name after its namespace is already Terminating, or "+
					"leaves a cluster-scoped object behind forever", got, tc.namespaced)
			}
		})
	}
}

// A MULTI-DOCUMENT MANIFEST IS REFUSED, NOT READ AS ITS FIRST DOCUMENT.
//
// yaml.Unmarshal decodes only the first document and returns nil error, so a
// stream whose SECOND object is cluster-scoped would be classified from the
// first and silently never torn down. kube.applyOne refuses multi-doc explicitly
// (exactlyOneDocument) for the same reason; the classifier now agrees.
func TestAMultiDocumentManifestIsRefused(t *testing.T) {
	multi := "kind: A\nmetadata:\n  name: a\n  namespace: env-a\n---\nkind: B\nmetadata:\n  name: b\n"
	if _, err := declaresNamespace([]byte(multi)); err == nil {
		t.Fatal("a multi-document manifest was classified from its FIRST document — the " +
			"second object's scope is never even looked at, so a cluster-scoped one leaks")
	}
}
