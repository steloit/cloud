package kube

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Client is the real Applier: server-side-apply and status reads against a
// cell's Kubernetes API over stdlib net/http.
//
// WHY stdlib and not client-go (architecture §140's principle, recorded for
// review): the driver already renders finished YAML, and server-side apply IS a
// single PATCH with `Content-Type: application/apply-patch+yaml` carrying that
// YAML. client-go would require YAML → unstructured → typed apply-configuration
// conversions plus a scheme registration for CNPG's CRD — strictly more code and
// a very large dependency tree to send a byte slice we already have. Architecture
// §13 names client-go as *available* for the ecosystem; §140 says not to take a
// library that "would be a dependency doing nothing". If the agent later needs
// watches, informers, or leader election, that calculus flips and client-go is
// the right answer — this is a deliberate, revisitable choice, not an oversight.
//
// Auth is the in-cluster ServiceAccount (projected token + CA), the same
// identity the workload-identity binding grants. No kubeconfig, no static keys.
type Client struct {
	base string // https://kubernetes.default.svc
	// tokenFile is re-read per request: GKE projected ServiceAccount tokens
	// expire (~1h) and are ROTATED IN PLACE in the file. Caching the value once
	// at boot means every apply 401s after the TTL and never recovers without a
	// restart — the concrete cost of not taking client-go (whose
	// rest.InClusterConfig installs a reloader), paid here directly.
	tokenFile  string
	token      string // static token, tests only
	hc         *http.Client
	fieldOwner string
}

const saDir = "/var/run/secrets/kubernetes.io/serviceaccount"

// NewInCluster builds a Client from the pod's projected ServiceAccount. It
// returns an error (never a partially-configured client) when the agent is not
// running in a cluster, so main can fall back to the Ack renderer visibly.
func NewInCluster() (*Client, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("kube: not running in a cluster (KUBERNETES_SERVICE_HOST/PORT unset)")
	}
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return nil, fmt.Errorf("kube: read service-account token: %w", err)
	}
	ca, err := os.ReadFile(saDir + "/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("kube: read cluster CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("kube: cluster CA is not valid PEM")
	}
	_ = token // presence checked above; the value is re-read per request
	return &Client{
		base:       fmt.Sprintf("https://%s:%s", host, port),
		tokenFile:  saDir + "/token",
		fieldOwner: "steloit-cell-agent",
		hc: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
		},
	}, nil
}

// NewClientForTest builds a Client against an arbitrary base URL (httptest), so
// the real apply/observe HTTP shapes are provable without a cluster.
func NewClientForTest(base, token string, hc *http.Client) *Client {
	return &Client{base: base, token: token, hc: hc, fieldOwner: "steloit-cell-agent"}
}

// objMeta is the minimum we parse out of a rendered manifest to route its
// request: apiVersion + kind + name. The body is sent verbatim — we never
// re-marshal the driver's YAML, so what CNPG receives is exactly what T3.4
// rendered and the golden tests pinned.
type objMeta struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

// Apply server-side-applies each manifest. SSA is idempotent by construction:
// the same bytes reapplied converge to the same object, and `force=true` makes
// the agent the authoritative field manager for the fields it owns (so a manual
// kubectl edit is corrected on the next converge — level-triggered, §2).
func (c *Client) Apply(ctx context.Context, namespace string, manifests [][]byte) error {
	for _, m := range manifests {
		var meta objMeta
		if err := yaml.Unmarshal(m, &meta); err != nil {
			return fmt.Errorf("kube: parse manifest: %w", err)
		}
		// A manifest is routed by the CALLER's namespace, not by its own
		// metadata.namespace — so a manifest declaring a different namespace
		// would be written into the caller's. The API server rejects that with a
		// 400, which makes it fail-closed but leaves the tenant boundary being
		// enforced a network hop away, and only for namespaced kinds. Refuse
		// locally instead: a cross-namespace write is a bug in the renderer, and
		// the agent should not need a round trip to say so.
		if got := meta.Metadata.Namespace; got != "" {
			if clusterScoped[meta.Kind] {
				return fmt.Errorf("kube: %s/%s is cluster-scoped but declares namespace %q",
					meta.Kind, meta.Metadata.Name, got)
			}
			if got != namespace {
				return fmt.Errorf("kube: %s/%s declares namespace %q but is being applied to %q",
					meta.Kind, meta.Metadata.Name, got, namespace)
			}
		}
		path, err := resourcePath(meta.APIVersion, meta.Kind, namespace, meta.Metadata.Name)
		if err != nil {
			return err
		}
		url := c.base + path + "?fieldManager=" + c.fieldOwner + "&force=true"
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(m))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/apply-patch+yaml")
		req.Header.Set("Accept", "application/json")
		c.auth(req)
		resp, err := c.hc.Do(req)
		if err != nil {
			return fmt.Errorf("kube: apply %s/%s: %w", meta.Kind, meta.Metadata.Name, err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("kube: apply %s/%s: %d: %s", meta.Kind, meta.Metadata.Name, resp.StatusCode, bytes.TrimSpace(body))
		}
	}
	return nil
}

// Observe reads the CNPG Cluster's `.status.phase`. A 404 means "not created
// yet" and returns "" (the renderer maps that to provisioning) — an absent
// cluster is a normal state during convergence, not an error.
func (c *Client) Observe(ctx context.Context, namespace, name string) (string, error) {
	path, err := resourcePath("postgresql.cnpg.io/v1", "Cluster", namespace, name)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	c.auth(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("kube: observe %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil // not created yet
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("kube: observe %s: %d: %s", name, resp.StatusCode, bytes.TrimSpace(body))
	}
	var out struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("kube: decode %s status: %w", name, err)
	}
	return out.Status.Phase, nil
}

// Delete removes the CNPG Cluster (teardown). A 404 is success — idempotent, so
// a repeated teardown converges to the same absence.
func (c *Client) Delete(ctx context.Context, namespace, kind, name string) error {
	apiVersion := "postgresql.cnpg.io/v1"
	if kind == "VolumeSnapshot" {
		apiVersion = "snapshot.storage.k8s.io/v1"
	}
	path, err := resourcePath(apiVersion, kind, namespace, name)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+path, nil)
	if err != nil {
		return err
	}
	c.auth(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("kube: delete %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil // already gone
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("kube: delete %s: %d: %s", name, resp.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}

// auth attaches the bearer, re-reading the projected token each request so a
// rotated token is picked up without a restart.
func (c *Client) auth(req *http.Request) {
	tok := c.token
	if c.tokenFile != "" {
		if b, err := os.ReadFile(c.tokenFile); err == nil {
			tok = strings.TrimSpace(string(b))
		}
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
}

// resourcePath builds the REST path for an object. Core v1 lives under /api/v1;
// everything else under /apis/<group>/<version>. The plural is derived by the
// standard lowercase+s rule, which holds for every kind this agent applies
// (Cluster→clusters, ScheduledBackup→scheduledbackups, VolumeSnapshot→
// volumesnapshots); an unknown kind is an error rather than a wrong guess.
func resourcePath(apiVersion, kind, namespace, name string) (string, error) {
	if kind == "" || name == "" {
		return "", fmt.Errorf("kube: manifest missing kind/name")
	}
	plural, ok := plurals[kind]
	if !ok {
		return "", fmt.Errorf("kube: unknown kind %q — add it to the plural map rather than guessing", kind)
	}
	prefix := "/apis/" + apiVersion
	if !strings.Contains(apiVersion, "/") { // core group, e.g. "v1"
		prefix = "/api/" + apiVersion
	}
	// CLUSTER-SCOPED kinds live at /<prefix>/<plural>/<name>. A Namespace nested
	// under /namespaces/<ns>/ is a 404 at apply time — the same class the plural
	// map exists to prevent, one level up. US-3.3a needs this because the agent
	// must create the env namespace itself, and a namespace has no namespace.
	if clusterScoped[kind] {
		return fmt.Sprintf("%s/%s/%s", prefix, plural, name), nil
	}
	if namespace == "" {
		return "", fmt.Errorf("kube: %s/%s is namespaced but no namespace was given", kind, name)
	}
	return fmt.Sprintf("%s/namespaces/%s/%s/%s", prefix, namespace, plural, name), nil
}

// clusterScoped is explicit for the same reason plurals is: guessing scope from
// the kind name is how a manifest silently applies to the wrong path.
var clusterScoped = map[string]bool{
	"Namespace": true,
}

// plurals is explicit, not inferred: a wrong pluralization is a 404 at apply
// time on a live cluster, which is exactly the class of bug that is expensive to
// find there and free to prevent here.
var plurals = map[string]string{
	"Cluster":         "clusters",
	"ScheduledBackup": "scheduledbackups",
	"VolumeSnapshot":  "volumesnapshots",
	"Backup":          "backups",
	"Secret":          "secrets",
	"StatefulSet":     "statefulsets",
	// US-3.3a — the env namespace and its D7 isolation boundary.
	"Namespace":     "namespaces",
	"NetworkPolicy": "networkpolicies",
	"ResourceQuota": "resourcequotas",
	"LimitRange":    "limitranges",
}
