// Package estimates is the T3.1 pricing engine: shape → line items, pricing
// tables as DATA (pricing.json, embedded), canon numbers as regression
// fixtures. The estimate-before-provision law rides on this: an accepted
// estimate id is the only key that lets createService proceed (T3.3), and
// the line grammar here is the invoice line grammar, verbatim.
package estimates

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

//go:embed pricing.json
var pricingJSON []byte

type pgSize struct {
	BaseCents  int64 `json:"base_cents"`
	IncludedGB int   `json:"included_gb"`
}

type computeSize struct {
	ServiceBaseCents int64 `json:"service_base_cents"`
	InstanceCents    int64 `json:"instance_cents"`
}

type pricing struct {
	Postgres struct {
		Sizes             map[string]pgSize `json:"sizes"`
		StorageCentsPerGB int64             `json:"storage_cents_per_gb"`
		HACents           int64             `json:"ha_cents"`
	} `json:"postgres"`
	Valkey struct {
		MemoryCentsPerGB int64 `json:"memory_cents_per_gb"`
	} `json:"valkey"`
	Web struct {
		Sizes map[string]computeSize `json:"sizes"`
	} `json:"web"`
	Worker struct {
		Sizes map[string]computeSize `json:"sizes"`
	} `json:"worker"`
}

var table = func() pricing {
	var p pricing
	if err := json.Unmarshal(pricingJSON, &p); err != nil {
		panic(fmt.Sprintf("estimates: pricing.json invalid: %v", err))
	}
	return p
}()

// ShapeInput mirrors ServiceShapeInput: product + optional intent/name +
// product-specific shape block.
type ShapeInput struct {
	Product string
	Intent  string // optional; server-defaulted by product (S11)
	Name    string
	Shape   map[string]any
}

// Line mirrors the Estimate.lines item — the grammar shared with invoices.
type Line struct {
	Name         string  `json:"name"`
	Product      string  `json:"product"`
	Intent       string  `json:"intent"`
	MonthlyCents int64   `json:"monthly_cents"`
	Basis        string  `json:"basis"` // fixed | usage_projection
	EgressNote   *string `json:"egress_note"`
}

// ShapeError names the field a caller must fix (422 at the edge).
type ShapeError struct{ Field, Detail string }

func (e ShapeError) Error() string { return "estimates: " + e.Field + ": " + e.Detail }

// defaultIntent is the documented server default (Intent schema, S11):
// postgres→database, valkey→cache, web/worker→app.
func defaultIntent(product string) string {
	switch product {
	case "postgres":
		return "database"
	case "valkey":
		return "cache"
	default:
		return "app"
	}
}

// Price one shape. Deterministic, integer cents end-to-end (ADR-025).
// allowedShapeKeys is the CLOSED shape schema per product (T12.4 review
// blocker): a shape is not an open bag — an unknown key is rejected, never
// silently accepted, so nothing secret-shaped can ride a shape into storage,
// capture, or an org-shared template.
// The vocabulary is the CANON's shape vocabulary (19-canon fixtures) — priced
// keys plus the declared non-priced configuration keys. Growing it is a
// reviewed change citing the canon, never a convenience.
var allowedShapeKeys = map[string]map[string]bool{
	"postgres": {"size": true, "storage_gb": true, "ha": true, "connections": true, "pgmq": true, "version": true},
	"valkey":   {"memory_mb": true, "eviction": true, "mode": true},
	"web":      {"size": true, "instances": true, "health_check": true},
	"worker":   {"size": true, "instances": true, "health_check": true},
}

// ProjectShape filters a shape to the product's closed schema — the
// defense-in-depth projection template capture applies on top of Price's
// rejection (a pre-schema stored shape can never leak unknown keys onward).
func ProjectShape(product string, shape map[string]any) map[string]any {
	allowed := allowedShapeKeys[product]
	out := map[string]any{}
	for k, v := range shape {
		if allowed[k] {
			out[k] = v
		}
	}
	return out
}

func Price(in ShapeInput) (Line, error) {
	if allowed, ok := allowedShapeKeys[in.Product]; ok {
		for k := range in.Shape {
			if !allowed[k] {
				return Line{}, ShapeError{Field: "shape." + k, Detail: "unknown shape field — shapes are a closed schema, allowed: " + boolKeys(allowed)}
			}
		}
	}
	intent := in.Intent
	if intent == "" {
		intent = defaultIntent(in.Product)
	}
	name := in.Name
	if name == "" {
		name = in.Product
	}
	line := Line{Name: name, Product: in.Product, Intent: intent, Basis: "fixed"}

	switch in.Product {
	case "postgres":
		size := shapeString(in.Shape, "size", "dev")
		sz, ok := table.Postgres.Sizes[size]
		if !ok {
			return Line{}, ShapeError{Field: "shape.size", Detail: "unknown size " + size + " — one of " + keys(table.Postgres.Sizes)}
		}
		cents := sz.BaseCents
		storageGB := shapeInt(in.Shape, "storage_gb", 0)
		if storageGB < 0 {
			return Line{}, ShapeError{Field: "shape.storage_gb", Detail: "must be >= 0"}
		}
		if extra := storageGB - sz.IncludedGB; extra > 0 {
			cents += int64(extra) * table.Postgres.StorageCentsPerGB
		}
		if shapeBool(in.Shape, "ha") {
			cents += table.Postgres.HACents
		}
		line.MonthlyCents = cents
	case "valkey":
		memMB := shapeInt(in.Shape, "memory_mb", 1024)
		if memMB <= 0 {
			return Line{}, ShapeError{Field: "shape.memory_mb", Detail: "must be > 0"}
		}
		// price per GB, rounded up to whole GB (integer cents, never fractions)
		gb := int64(math.Ceil(float64(memMB) / 1024.0))
		line.MonthlyCents = gb * table.Valkey.MemoryCentsPerGB
	case "web", "worker":
		sizes := table.Web.Sizes
		if in.Product == "worker" {
			sizes = table.Worker.Sizes
		}
		size := shapeString(in.Shape, "size", "standard-1")
		sz, ok := sizes[size]
		if !ok {
			return Line{}, ShapeError{Field: "shape.size", Detail: "unknown size " + size + " — one of " + keys(sizes)}
		}
		instances := shapeInt(in.Shape, "instances", 1)
		if instances < 1 {
			return Line{}, ShapeError{Field: "shape.instances", Detail: "must be >= 1"}
		}
		line.MonthlyCents = sz.ServiceBaseCents + int64(instances)*sz.InstanceCents
	default:
		return Line{}, ShapeError{Field: "product", Detail: "unknown product " + in.Product + " — the surface is [postgres, valkey, web, worker] (ADR-0004)"}
	}
	return line, nil
}

func boolKeys(m map[string]bool) string {
	out := ""
	for k := range m {
		if out != "" {
			out += ", "
		}
		out += k
	}
	return out
}

// PriceAll prices a set of shapes; the total is the exact sum of lines —
// one arithmetic, everywhere.
func PriceAll(in []ShapeInput) ([]Line, int64, error) {
	lines := make([]Line, 0, len(in))
	var total int64
	for _, s := range in {
		l, err := Price(s)
		if err != nil {
			return nil, 0, err
		}
		lines = append(lines, l)
		total += l.MonthlyCents
	}
	return lines, total, nil
}

// ---- shape helpers (jsonb → typed, forgiving on JSON number forms) ---------

func shapeString(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func shapeInt(m map[string]any, key string, def int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return def
}

func shapeBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func keys[V any](m map[string]V) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// small maps; deterministic enough for an error message
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return strings.Join(out, "|")
}
