package identity_test

// US-11.2 — the 80% warning routes end-to-end: a member gets a bell notification
// (and email if on) carrying the meter, the percent, and the overage price.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/notify"
	"github.com/steloit/cloud/services/api/internal/quota"
)

func TestQuotaWarningRoutesToBell(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "qw-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"qwco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// QA scenario 2 (canon-pinned Business egress): 87/100 GB, 9¢/GB.
	msg, warn := quota.WarnMessage("egress", 100, 87, 9)
	if !warn {
		t.Fatal("87/100 must warn")
	}
	router := notify.NewRouter(store.New(w.pool), w.kek)
	if err := identity.EmitQuotaWarning(ctx, router, org.Id, msg); err != nil {
		t.Fatalf("emit: %v", err)
	}

	// the owner's bell carries the warning with the math (percent + $0.09).
	_, lb := w.get(t, "/v1/me/notifications", ownerCk)
	if !strings.Contains(lb, "egress at 87%") {
		t.Fatalf("bell missing the 80%% warning: %s", lb)
	}
	if !strings.Contains(lb, "$0.09") {
		t.Fatalf("warning did not show the overage math ($0.09): %s", lb)
	}
}
