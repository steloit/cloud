package identity

// T11.3 invoice read surface (B3: an invoice is data — every line expands to the
// meter rows behind it). Generation lives in internal/invoice; this lists.

import (
	"context"
	"encoding/json"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

func (h *Handlers) ListInvoices(ctx context.Context, req gen.ListInvoicesRequestObject) (gen.ListInvoicesResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "billing.view", false); err != nil {
		return nil, err
	}
	rows, err := h.svc.ListInvoices(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	out := make([]gen.Invoice, 0, len(rows))
	for _, r := range rows {
		out = append(out, invoiceToAPI(r))
	}
	return gen.ListInvoices200JSONResponse{Data: &out}, nil
}

// invoiceLine mirrors the stored jsonb shape (internal/invoice.Line).
type invoiceLine struct {
	Description string  `json:"description"`
	ProjectID   *string `json:"project_id"`
	Cents       int64   `json:"cents"`
	UsageRef    string  `json:"usage_ref"`
}

func invoiceToAPI(r store.Invoice) gen.Invoice {
	inv := gen.Invoice{
		Id: r.ID, Period: r.Period, Status: gen.InvoiceStatus(r.Status),
		TotalCents: int(r.TotalCents),
	}
	var lines []invoiceLine
	if err := json.Unmarshal(r.Lines, &lines); err == nil && len(lines) > 0 {
		apiLines := make([]struct {
			Cents       *int    `json:"cents,omitempty"`
			Description *string `json:"description,omitempty"`
			ProjectId   *string `json:"project_id,omitempty"`
			UsageRef    *string `json:"usage_ref,omitempty"`
		}, 0, len(lines))
		for _, l := range lines {
			l := l
			cents := int(l.Cents)
			desc := l.Description
			ref := l.UsageRef
			apiLines = append(apiLines, struct {
				Cents       *int    `json:"cents,omitempty"`
				Description *string `json:"description,omitempty"`
				ProjectId   *string `json:"project_id,omitempty"`
				UsageRef    *string `json:"usage_ref,omitempty"`
			}{Cents: &cents, Description: &desc, ProjectId: l.ProjectID, UsageRef: &ref})
		}
		inv.Lines = &apiLines
	}
	return inv
}
