package notify

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	ruled := map[string]bool{
		KindAlert: true, KindProposal: true, KindApproval: true, KindDeploy: true,
		KindLifecycle: true, KindBilling: true, KindSecurity: true,
	}

	cases := []struct {
		name   string
		action string
		wantOK bool
		want   string // expected kind when wantOK
	}{
		// Positive: the golden action → kind mapping. Exact equality, so a
		// title/kind attached to the wrong action fails here rather than
		// passing the gallery provenance check on someone else's copy.
		{"deploy created", "deploy.created", true, KindDeploy},

		// Negatives. A table of positives alone never exercises the branch that
		// keeps the badge honest, so these are load-bearing, not filler.
		{"unknown action", "service.exploded", false, ""},
		{"empty action", "", false, ""},
		{"near miss - singular", "deploy.create", false, ""},
		{"near miss - real action, no frame row", "deploy.rolled_back", false, ""},
		{"real action outside the bell", "org.updated", false, ""},
		{"prefix of a real action", "deploy.", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, title, ok := classifyAction(tc.action)
			if ok != tc.wantOK {
				t.Fatalf("Classify(%q) ok = %v, want %v", tc.action, ok, tc.wantOK)
			}
			if !ok {
				// Both returns must be empty — never half-populated.
				if kind != "" || title != "" {
					t.Fatalf("Classify(%q) not worthy but returned kind=%q title=%q; want both empty",
						tc.action, kind, title)
				}
				return
			}
			if kind != tc.want {
				t.Fatalf("Classify(%q) kind = %q, want %q", tc.action, kind, tc.want)
			}
			if title == "" {
				t.Fatalf("Classify(%q) is worthy but returned an empty title", tc.action)
			}
			if !ruled[kind] {
				t.Fatalf("Classify(%q) kind = %q, which is not one of the seven ruled kinds", tc.action, kind)
			}
		})
	}
}

// TestClassifyTitleIsGolden pins the exact title format per action.
//
// TestClassify asserts only the KIND, and the frame fragment is a prefix — so
// reordering deployCreatedTitle's verbs ("Deploy #%d by %s to %s") would pass
// both without this. The only other exact-title assertion lives in a
// container-gated integration test, so without this the golden pin disappears
// on any machine lacking Docker.
func TestClassifyTitleIsGolden(t *testing.T) {
	golden := map[string]struct{ kind, title string }{
		"deploy.created": {KindDeploy, "Deploy #%d to %s by %s"},
	}
	if len(golden) != len(classifications) {
		t.Fatalf("golden table has %d entries, classifications has %d — every classification "+
			"must be pinned", len(golden), len(classifications))
	}
	for action, want := range golden {
		kind, title, ok := classifyAction(action)
		if !ok {
			t.Errorf("%s: not classified", action)
			continue
		}
		if kind != want.kind {
			t.Errorf("%s: kind = %q, want %q", action, kind, want.kind)
		}
		if title != want.title {
			t.Errorf("%s: title = %q, want %q (verb ORDER is part of the frame)", action, title, want.title)
		}
	}
}

// galleryPath is the frame source — the only authority for notification copy.
const galleryPath = "../../../../docs/product/00-sources/Steloit-Console-Screens.html"

var tagRE = regexp.MustCompile(`<[^>]*>`)

// TestClassifyTitlesMatchFrameSource pins every title to the gallery so a
// plausible-sounding invented title cannot ship.
//
// The search is scoped to [N1, N3) because a bare substring search over the
// 1.4 MB gallery false-passes: " is ready" alone matches four places, two of
// them in unrelated frames.
func TestClassifyTitlesMatchFrameSource(t *testing.T) {
	raw, err := os.ReadFile(galleryPath)
	if err != nil {
		t.Fatalf("read gallery: %v", err)
	}
	gallery := string(raw)

	n1 := strings.Index(gallery, "<b>N1</b>")
	n3 := strings.Index(gallery, "<b>N3</b>")

	// Guard the guard. If a marker were renamed and the index clamped to -1,
	// the region would silently widen back to the whole file and this test
	// would degrade into exactly the false-pass it exists to prevent.
	if n1 < 0 || n3 < 0 {
		t.Fatalf("region markers not found (n1=%d n3=%d) — the gallery's frame labels changed; "+
			"fix the markers, do not widen the search", n1, n3)
	}
	if n1 >= n3 {
		t.Fatalf("region markers out of order: n1=%d must precede n3=%d", n1, n3)
	}

	// Strip markup: the gallery interleaves tags inside titles, so a fragment
	// is only contiguous after stripping.
	region := tagRE.ReplaceAllString(gallery[n1:n3], "")

	if len(classifications) == 0 {
		t.Fatal("no classifications to verify — the table cannot be empty")
	}
	for action, c := range classifications {
		if c.fragment == "" {
			t.Errorf("%s: no frame fragment declared; every title must be provable against the gallery", action)
			continue
		}
		if !strings.Contains(region, c.fragment) {
			t.Errorf("%s: fragment %q not found in the N1/N2 region — the title is not frame-verbatim",
				action, c.fragment)
		}
	}

	// Negative controls, one on EACH side. A single control only guards the
	// direction it sits in: widening n1 backwards is caught by the first,
	// widening n3 forwards (a frame inserted between N2 and N3) is caught only
	// by the second.
	for _, outside := range []string{
		"Your connect command is ready", // before N1
		"Nothing needs you.",            // inside N3, after the region ends
	} {
		if !strings.Contains(gallery, outside) {
			t.Fatalf("negative control %q no longer exists in the gallery; pick another", outside)
		}
		if strings.Contains(region, outside) {
			t.Fatalf("negative control %q found inside the N1/N2 region — the region is too wide, "+
				"so fragment matches prove nothing", outside)
		}
	}
}
