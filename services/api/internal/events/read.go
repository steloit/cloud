package events

import (
	"context"
	"fmt"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// Reader serves the JSON views over the spine (newest first, keyset cursor).
type Reader struct{ q *store.Queries }

func NewReader(q *store.Queries) *Reader { return &Reader{q: q} }

const pageSize = 50

// Filters narrows a page; nil fields don't filter.
type Filters struct {
	Kind, Actor, Action *string
}

// Page returns one org-fenced page, newest first. An empty next_cursor means
// the ledger is exhausted; a cursor from a foreign org matches nothing.
func (r *Reader) Page(ctx context.Context, orgID string, f Filters, cursor string) (gen.EventList, error) {
	params := store.ListOrgEventsDescParams{OrgID: orgID, Limit: pageSize + 1}
	if f.Kind != nil {
		params.Kind = textOf(*f.Kind)
	}
	if f.Actor != nil {
		params.Actor = textOf(*f.Actor)
	}
	if f.Action != nil {
		params.Action = textOf(*f.Action)
	}
	if cursor != "" {
		cur, err := DecodeCursor(cursor)
		if err != nil {
			return gen.EventList{}, ErrBadCursor
		}
		params.BeforeAt = tstzOf(cur.At)
		params.BeforeID = textOf(cur.ID)
	}
	rows, err := r.q.ListOrgEventsDesc(ctx, params)
	if err != nil {
		return gen.EventList{}, fmt.Errorf("events: page: %w", err)
	}
	var next *string
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		lastRow := rows[len(rows)-1]
		c := EncodeCursor(lastRow.At.Time, lastRow.ID)
		next = &c
	}
	data := make([]gen.Event, 0, len(rows))
	for _, evt := range rows {
		data = append(data, ToAPI(evt))
	}
	return gen.EventList{Data: &data, NextCursor: next}, nil
}

// ErrBadCursor marks a malformed cursor (422 at the edge).
var ErrBadCursor = fmt.Errorf("events: bad cursor")
