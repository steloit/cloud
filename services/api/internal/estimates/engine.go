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
	"sort"
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
// fieldSpec declares one shape field: its type and the value that an omitted
// key means. This is the SINGLE source of shape truth — Price computes from it
// and Canonical renders from it, so the pricing path and the contract identity
// cannot drift apart. Retyping defaults in two places was a live hazard: a
// default that changed in one and not the other either false-refuses a
// legitimate create or, if the two values happen to price equally, silently
// reopens the substitution hole this schema exists to close (US-3.7).
type fieldSpec struct {
	kind string // "string" | "int" | "bool" | "opaque"
	def  any
}

// shapeSchema is the closed schema per product. It includes fields the price
// does NOT depend on (postgres connections/pgmq/version, valkey eviction/mode,
// web/worker health_check) because they are DECLARED CONFIGURATION: a customer
// who priced `pgmq: false` did not contract for `pgmq: true`, however equal the
// bill. "Any shape substitution impossible regardless of price equality" means
// every declared field, not merely every billed one.
//
// A default of "" or 0 for an unpriced key means UNSET: pricing `{}` and
// creating `{version: "17"}` are different contracts and must not match.
var shapeSchema = map[string]map[string]fieldSpec{
	"postgres": {
		"size":       {"string", "dev"},
		"storage_gb": {"int", 0},
		"ha":         {"bool", false},
		// Unpriced but contracted. OPAQUE because these carry structure, not
		// scalars — canon's `connections` is `{used, max}` and `pgmq` is
		// `{delivery, dlq, dlq_depth}`. The engine has no opinion on their
		// contents; it only requires that what was priced is what is built.
		"connections": {"opaque", nil},
		"pgmq":        {"opaque", nil},
		"version":     {"string", ""},
	},
	"valkey": {
		"memory_mb": {"int", 1024},
		"eviction":  {"string", ""},
		"mode":      {"string", ""},
	},
	"web": {
		"size":         {"string", "standard-1"},
		"instances":    {"int", 1},
		"health_check": {"string", ""},
	},
	"worker": {
		"size":         {"string", "standard-1"},
		"instances":    {"int", 1},
		"health_check": {"string", ""},
	},
}

// allowedShapeKeys is DERIVED, so the allow-list and the schema cannot disagree.
var allowedShapeKeys = func() map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for product, fields := range shapeSchema {
		keys := map[string]bool{}
		for k := range fields {
			keys[k] = true
		}
		out[product] = keys
	}
	return out
}()

// resolve applies the closed schema to a shape: it rejects unknown keys, rejects
// a known key carrying the wrong TYPE, and fills every declared field with its
// default. The result is what both the price and the contract identity are
// computed from.
//
// Rejecting a wrong type rather than falling back to the default is
// load-bearing. The old helpers defaulted silently, so `{storage_gb: "78"}`
// priced as 0 GB while the raw shape was persisted verbatim — an under-billed
// provision the moment any driver reads the field.
func resolve(in ShapeInput) (map[string]any, string, error) {
	fields, ok := shapeSchema[in.Product]
	if !ok {
		return nil, "", ShapeError{Field: "product", Detail: "unknown product " + in.Product + " — the surface is [postgres, valkey, web, worker] (ADR-0004)"}
	}
	for k := range in.Shape {
		if _, known := fields[k]; !known {
			return nil, "", ShapeError{Field: "shape." + k, Detail: "unknown shape field — shapes are a closed schema, allowed: " + boolKeys(allowedShapeKeys[in.Product])}
		}
	}
	out := make(map[string]any, len(fields))
	for k, spec := range fields {
		raw, present := in.Shape[k]
		if !present || raw == nil {
			out[k] = spec.def
			continue
		}
		switch spec.kind {
		case "string":
			v, ok := raw.(string)
			if !ok {
				return nil, "", ShapeError{Field: "shape." + k, Detail: "must be a string"}
			}
			out[k] = v
		case "bool":
			v, ok := raw.(bool)
			if !ok {
				return nil, "", ShapeError{Field: "shape." + k, Detail: "must be true or false"}
			}
			out[k] = v
		case "int":
			n, ok := asInt(raw)
			if !ok {
				return nil, "", ShapeError{Field: "shape." + k, Detail: "must be a whole number"}
			}
			out[k] = n
		case "opaque":
			// Carried through unexamined; the identity compares it structurally.
			out[k] = raw
		}
	}
	intent := in.Intent
	if intent == "" {
		intent = defaultIntent(in.Product)
	}
	return out, intent, nil
}

// asInt accepts every JSON number form a whole number can arrive as — but NOT a
// fractional one, which would silently truncate a shape the customer did not ask
// for.
func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	}
	return 0, false
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
	shape, intent, err := resolve(in)
	if err != nil {
		return Line{}, err
	}
	name := in.Name
	if name == "" {
		name = in.Product
	}
	line := Line{Name: name, Product: in.Product, Intent: intent, Basis: "fixed"}

	switch in.Product {
	case "postgres":
		size, ok := shape["size"].(string)
		if !ok {
			return Line{}, fmt.Errorf("estimates: resolved size is %T, not string — shapeSchema and Price disagree", shape["size"])
		}
		sz, ok := table.Postgres.Sizes[size]
		if !ok {
			return Line{}, ShapeError{Field: "shape.size", Detail: "unknown size " + size + " — one of " + keys(table.Postgres.Sizes)}
		}
		cents := sz.BaseCents
		storageGB, ok := shape["storage_gb"].(int)
		if !ok {
			return Line{}, fmt.Errorf("estimates: resolved storage_gb is %T, not int — shapeSchema and Price disagree", shape["storage_gb"])
		}
		if storageGB < 0 {
			return Line{}, ShapeError{Field: "shape.storage_gb", Detail: "must be >= 0"}
		}
		if extra := storageGB - sz.IncludedGB; extra > 0 {
			cents += int64(extra) * table.Postgres.StorageCentsPerGB
		}
		ha, ok := shape["ha"].(bool)
		if !ok {
			return Line{}, fmt.Errorf("estimates: resolved ha is %T, not bool — shapeSchema and Price disagree", shape["ha"])
		}
		if ha {
			cents += table.Postgres.HACents
		}
		line.MonthlyCents = cents
	case "valkey":
		memMB, ok := shape["memory_mb"].(int)
		if !ok {
			return Line{}, fmt.Errorf("estimates: resolved memory_mb is %T, not int — shapeSchema and Price disagree", shape["memory_mb"])
		}
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
		size, sok := shape["size"].(string)
		if !sok {
			return Line{}, fmt.Errorf("estimates: resolved size is %T, not string — shapeSchema and Price disagree", shape["size"])
		}
		sz, ok := sizes[size]
		if !ok {
			return Line{}, ShapeError{Field: "shape.size", Detail: "unknown size " + size + " — one of " + keys(sizes)}
		}
		instances, iok := shape["instances"].(int)
		if !iok {
			return Line{}, fmt.Errorf("estimates: resolved instances is %T, not int — shapeSchema and Price disagree", shape["instances"])
		}
		if instances < 1 {
			return Line{}, ShapeError{Field: "shape.instances", Detail: "must be >= 1"}
		}
		line.MonthlyCents = sz.ServiceBaseCents + int64(instances)*sz.InstanceCents
	default:
		// resolve() already rejected products absent from shapeSchema, so
		// reaching here means a product was DECLARED but never given a pricing
		// arm. Without this it would price at zero — the mis-billing direction.
		// Derived by construction: no second table to keep in sync.
		return Line{}, ShapeError{Field: "product", Detail: "product " + in.Product + " is declared but not priced"}
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

// Canonical returns the CONFIGURATION IDENTITY of a shape: the product, the
// resolved intent, and every shape field the engine reads, with defaults
// applied — rendered as a deterministic string.
//
// This exists because the estimate gate used to match on (product, price), and
// prices collide: a postgres `dev` with 78 GB and a `standard` both come to
// 5800¢, so a caller could price one configuration and provision the other.
// "The estimate IS the contract" has to mean the whole contracted
// configuration, not a number that happens to agree (US-3.7).
//
// Defaults are RESOLVED rather than compared literally, so the same
// configuration spelled differently still matches: `{size: dev}` and
// `{size: dev, ha: false}` are the same contract and must not be refused.
// Fields outside the closed schema are rejected by the same rule Price uses,
// so an unknown key can never widen the identity.
//
// Name is deliberately excluded — a customer may call the service anything;
// what is contracted is the configuration, not the label.
func Canonical(in ShapeInput) (string, error) {
	shape, intent, err := resolve(in)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(shape))
	for k := range shape {
		names = append(names, k)
	}
	sort.Strings(names) // map order must not change the identity
	var b strings.Builder
	b.WriteString(in.Product)
	b.WriteString("|")
	b.WriteString(intent)
	for _, k := range names {
		// json.Marshal sorts map keys, so a structured value renders the same
		// however it was spelled — %v over a map would not.
		enc, err := json.Marshal(shape[k])
		if err != nil {
			return "", fmt.Errorf("estimates: canonicalize %s: %w", k, err)
		}
		fmt.Fprintf(&b, "|%s=%s", k, enc)
	}
	return b.String(), nil
}

// Resolve is the exported form: the shape as the engine understands it, with
// every declared field present and defaulted. CreateService persists THIS
// rather than the raw request map, so what is stored — and what the cell is
// handed — is the configuration that was priced and contracted.
func Resolve(in ShapeInput) (map[string]any, error) {
	shape, _, err := resolve(in)
	return shape, err
}
