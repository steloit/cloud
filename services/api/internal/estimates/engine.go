// Package estimates is the T3.1 pricing engine: shape → line items, pricing
// tables as DATA (pricing.json, embedded), canon numbers as regression
// fixtures. The estimate-before-provision law rides on this: an accepted
// estimate id is the only key that lets createService proceed (T3.3), and
// the line grammar here is the invoice line grammar, verbatim.
package estimates

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/steloit/cloud/services/api/internal/platform/money"
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
	Name         string      `json:"name"`
	Product      string      `json:"product"`
	Intent       string      `json:"intent"`
	MonthlyCents money.Cents `json:"monthly_cents"`
	Basis        string      `json:"basis"` // fixed | usage_projection
	EgressNote   *string     `json:"egress_note"`
}

// ShapeError names the field a caller must fix (422 at the edge).
type ShapeError struct {
	Field, Detail string
	cause         error
}

func (e ShapeError) Error() string { return "estimates: " + e.Field + ": " + e.Detail }

// Unwrap exposes the arithmetic cause (money.ErrOverflow and friends) without
// putting it in the customer-facing Detail.
func (e ShapeError) Unwrap() error { return e.cause }

// tooLargeToPrice is the single 422 for any dimension whose arithmetic left the
// representable range. The field names what the caller must change; the money
// error is wrapped so logs can say which operation broke without leaking the
// platform's internal ceiling arithmetic into the response.
func tooLargeToPrice(field string, cause error) ShapeError {
	return ShapeError{
		Field: field,
		Detail: "too large to price — the monthly total would exceed what the platform can meter exactly (max " +
			strconv.FormatInt(money.MaxMonthly, 10) + " cents/month)",
		cause: cause,
	}
}

// boolToInt supports integer ceiling-division rounding.
func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

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
// catalogIntents mirrors the Intent enum in openapi.yaml (ADR-039/040/041, S11).
var catalogIntents = map[string]bool{
	"app": true, "database": true, "jobs": true, "search": true,
	"vector": true, "cache": true, "storage": true, "ai": true,
}

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
		default:
			// An unrecognised kind would otherwise DROP the field from the
			// resolved map when present — vanishing from both the price and the
			// contract identity, which is precisely the substitution this
			// schema exists to prevent. Fail loudly instead.
			return nil, "", fmt.Errorf("estimates: shape field %q of %s declares unknown kind %q", k, in.Product, spec.kind)
		}
	}
	intent := in.Intent
	if intent == "" {
		intent = defaultIntent(in.Product)
	}
	// Validate here, where the shape is validated, so an out-of-catalog intent
	// is a field error BEFORE the one-shot estimate is burned. It previously
	// reached the INSERT and violated the services.intent CHECK constraint —
	// surfacing as a 500 telling the customer to retry, with their estimate
	// already consumed and every retry returning 409 forever.
	if !catalogIntents[intent] {
		return nil, "", ShapeError{Field: "intent", Detail: "unknown intent " + intent + " — one of " + boolKeys(catalogIntents)}
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
		cents, err := money.FromInt(sz.BaseCents)
		if err != nil {
			return Line{}, fmt.Errorf("estimates: pricing table base for postgres/%s is unrepresentable: %w", size, err)
		}
		storageGB, ok := shape["storage_gb"].(int)
		if !ok {
			return Line{}, fmt.Errorf("estimates: resolved storage_gb is %T, not int — shapeSchema and Price disagree", shape["storage_gb"])
		}
		if storageGB < 0 {
			return Line{}, ShapeError{Field: "shape.storage_gb", Detail: "must be >= 0"}
		}
		if extra := storageGB - sz.IncludedGB; extra > 0 {
			perGB, err := money.FromInt(table.Postgres.StorageCentsPerGB)
			if err != nil {
				return Line{}, fmt.Errorf("estimates: postgres storage rate is unrepresentable: %w", err)
			}
			cents, err = cents.AddMul(perGB, int64(extra))
			if err != nil {
				return Line{}, tooLargeToPrice("shape.storage_gb", err)
			}
		}
		ha, ok := shape["ha"].(bool)
		if !ok {
			return Line{}, fmt.Errorf("estimates: resolved ha is %T, not bool — shapeSchema and Price disagree", shape["ha"])
		}
		if ha {
			haCents, err := money.FromInt(table.Postgres.HACents)
			if err != nil {
				return Line{}, fmt.Errorf("estimates: postgres ha rate is unrepresentable: %w", err)
			}
			cents, err = cents.Add(haCents)
			if err != nil {
				return Line{}, tooLargeToPrice("shape.ha", err)
			}
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
		// Whole GB, rounded up, computed in integers: float64 loses precision
		// above 2^53 and memory_mb is caller-supplied.
		gb := int64(memMB)/1024 + boolToInt(int64(memMB)%1024 != 0)
		perGB, err := money.FromInt(table.Valkey.MemoryCentsPerGB)
		if err != nil {
			return Line{}, fmt.Errorf("estimates: valkey memory rate is unrepresentable: %w", err)
		}
		priced, err := money.Zero.AddMul(perGB, gb)
		if err != nil {
			return Line{}, tooLargeToPrice("shape.memory_mb", err)
		}
		line.MonthlyCents = priced
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
		base, err := money.FromInt(sz.ServiceBaseCents)
		if err != nil {
			return Line{}, fmt.Errorf("estimates: pricing table base for %s/%s is unrepresentable: %w", in.Product, size, err)
		}
		perInstance, err := money.FromInt(sz.InstanceCents)
		if err != nil {
			return Line{}, fmt.Errorf("estimates: pricing table instance rate for %s/%s is unrepresentable: %w", in.Product, size, err)
		}
		priced, err := base.AddMul(perInstance, int64(instances))
		if err != nil {
			return Line{}, tooLargeToPrice("shape.instances", err)
		}
		line.MonthlyCents = priced
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
	// Sorted: an unsorted error message lists the same set differently on every
	// run, which makes a field-error assertion flaky and a diff unreadable.
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ", ")
}

// PriceAll prices a set of shapes; the total is the exact sum of lines —
// one arithmetic, everywhere.
func PriceAll(in []ShapeInput) ([]Line, money.Cents, error) {
	lines := make([]Line, 0, len(in))
	total := money.Zero
	for _, s := range in {
		l, err := Price(s)
		if err != nil {
			return nil, money.Zero, err
		}
		lines = append(lines, l)
		// The TOTAL is bounded too, not just each line. Bounding only the lines
		// moved the wrap up one level rather than removing it: two individually
		// legal web lines produced `monthly_total_cents: -3016` over HTTP, and
		// the total is the number the customer accepts, the number persisted,
		// and the number the estimate gate compares. Reachable through
		// `shape.instances` with no override anywhere.
		v, err := total.Add(l.MonthlyCents)
		if err != nil {
			return nil, money.Zero, ShapeError{
				Field: "services",
				Detail: "the estimate total is too large to price — the combined monthly cost would exceed " +
					"what the platform can meter exactly (max " + strconv.FormatInt(money.MaxMonthly, 10) + " cents/month)",
				cause: err,
			}
		}
		total = v
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

// PriceWithInstances prices a shape with its instance count replaced by a
// manual pin (D22).
//
// It REFUSES for a product whose shape declares no `instances` — postgres and
// valkey today. Their pins provision replicas the catalog has no price for, and
// the founder ruling is that pinned capacity is metered: capacity we cannot
// price is capacity we must not provision. Inventing a rate here would be worse
// than refusing, because it would be a number nobody ratified appearing on an
// invoice.
func PriceWithInstances(in ShapeInput, instances int) (Line, error) {
	fields, ok := shapeSchema[in.Product]
	if !ok {
		return Line{}, ShapeError{Field: "product", Detail: "unknown product " + in.Product}
	}
	if _, priced := fields["instances"]; !priced {
		return Line{}, ShapeError{
			Field:  "override.instances",
			Detail: "a manual instance pin is not priceable for " + in.Product + " — its catalog shape has no instance count, so the extra capacity could not be metered",
		}
	}
	pinned := map[string]any{}
	for k, v := range in.Shape {
		pinned[k] = v
	}
	pinned["instances"] = instances
	line, err := Price(ShapeInput{Product: in.Product, Intent: in.Intent, Name: in.Name, Shape: pinned})
	// Report the field the CALLER sent. Price validates the merged shape and so
	// names `shape.instances`, but on this path the number came from
	// `override.instances` — telling a client to fix a field it did not send is
	// the same class of unhelpful as the 500 this task already replaced with a
	// field error.
	var se ShapeError
	if errors.As(err, &se) && se.Field == "shape.instances" {
		se.Field = "override.instances"
		return Line{}, se
	}
	return line, err
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
