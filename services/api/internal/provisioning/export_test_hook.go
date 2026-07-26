package provisioning

import "context"

// ResolveNamespaceForTest exposes namespace derivation to integration tests in
// other packages (the reconcile package owns the real-Postgres harness).
func (s *Service) ResolveNamespaceForTest(ctx context.Context, envID string) (string, error) {
	return s.resolveNamespace(ctx, envID)
}
