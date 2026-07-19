package identity

// T11.6 CSV billing exports (B2/B3): the same usage + invoice rows the JSON
// surfaces show, as text/csv. Registered pre-strict — the strict server buffers
// typed JSON; a CSV body streams raw (the SSE/testWebhook pre-strict pattern).
// PDF is a deferred follow-up (owner-ruled: CSV now).

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// MountBillingExports registers the CSV export routes (GET, `:export` literal
// segment — mux-routable, unlike testWebhook's `{wbh}:test`).
func (h *Handlers) MountBillingExports(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/orgs/{org}/billing/usage:export", func(w http.ResponseWriter, r *http.Request) {
		if err := h.exportUsage(r.Context(), w, r); err != nil {
			h.responseError(w, r, err)
		}
	})
	mux.HandleFunc("GET /v1/orgs/{org}/billing/invoices:export", func(w http.ResponseWriter, r *http.Request) {
		if err := h.exportInvoices(r.Context(), w, r); err != nil {
			h.responseError(w, r, err)
		}
	})
}

// authorizeBillingRead resolves the caller and gates on billing.view for the org.
func (h *Handlers) authorizeBillingRead(ctx context.Context, r *http.Request, orgID string) error {
	p, ok := h.svc.PrincipalFromRequest(ctx, r)
	if !ok {
		return ErrNoSession
	}
	return h.authz.Require(ctx, p, "billing.view", rbac.Scope{OrgID: orgID})
}

func (h *Handlers) exportUsage(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	org := r.PathValue("org")
	if err := h.authorizeBillingRead(ctx, r, org); err != nil {
		return err
	}
	period := r.URL.Query().Get("month")
	if period == "" {
		period = currentPeriod()
	}
	rows, err := h.svc.q.GetQuotaUsage(ctx, store.GetQuotaUsageParams{OrgID: org, Period: period})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="usage-`+period+`.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"period", "meter", "used", "rate_cents"})
	for _, u := range rows {
		_ = cw.Write([]string{u.Period, u.Meter, strconv.FormatInt(u.Used, 10), strconv.FormatInt(u.RateCents, 10)})
	}
	cw.Flush()
	return cw.Error()
}

func (h *Handlers) exportInvoices(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	org := r.PathValue("org")
	if err := h.authorizeBillingRead(ctx, r, org); err != nil {
		return err
	}
	rows, err := h.svc.ListInvoices(ctx, org)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="invoices.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "period", "status", "total_cents", "lines"})
	for _, inv := range rows {
		_ = cw.Write([]string{inv.ID, inv.Period, inv.Status, strconv.FormatInt(inv.TotalCents, 10), string(compactJSON(inv.Lines))})
	}
	cw.Flush()
	return cw.Error()
}

// compactJSON strips whitespace so a lines blob is one CSV cell.
func compactJSON(b []byte) []byte {
	if !json.Valid(b) {
		return b
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return b
	}
	return buf.Bytes()
}
