// Package idempotency implements the ratified S7 ruling: a mutating POST may
// carry an `Idempotency-Key`, and the server dedupes on (principal, endpoint,
// key) for 24h. A replay returns the ORIGINAL response with
// `Idempotent-Replay: true`; the same key with a different body is a 409.
//
// Why this is load-bearing for US-3.6 ("failed provisioning never bills"): the
// estimate-before-provision law (F2) makes an estimate ONE-SHOT. A client that
// times out mid-createService and retries would otherwise either burn a second
// estimate or create a second service — so "a failure never bills" is not
// credible until a retry is provably safe.
package idempotency

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

// Store is the persistence this package needs.
type Store interface {
	ClaimIdempotencyKey(ctx context.Context, arg store.ClaimIdempotencyKeyParams) (store.IdempotencyKey, error)
	GetIdempotencyKey(ctx context.Context, arg store.GetIdempotencyKeyParams) (store.IdempotencyKey, error)
	SaveIdempotentResponse(ctx context.Context, arg store.SaveIdempotentResponseParams) (int64, error)
	ReleaseIdempotencyKey(ctx context.Context, arg store.ReleaseIdempotencyKeyParams) (int64, error)
	DiscardUnreadableIdempotencyKey(ctx context.Context, arg store.DiscardUnreadableIdempotencyKeyParams) (int64, error)
	SweepExpiredIdempotencyKeys(ctx context.Context) (int64, error)
}

type Service struct {
	q   Store
	kek secrets.KEK
}

// New builds the service. kek is REQUIRED: a recorded response may contain
// credential material (US-3.6a), and there is no mode in which it is stored in
// plaintext — so the dependency is constructor-level, not a toggle.
func New(q Store, kek secrets.KEK) *Service { return &Service{q: q, kek: kek} }

// ErrLostClaim reports that this request no longer owns the key when it tried
// to RECORD its response: a successor took the claim over, so the response was
// not stored and a retry will re-execute it. Surfaced rather than swallowed
// because it is the state that precedes a duplicate. (A failed RELEASE is not
// reported — see Complete: it has a benign reading, a save does not.)
var ErrLostClaim = errors.New("idempotency: the claim was taken over by another request")

// ErrBodyMismatch is the same key reused for a DIFFERENT request body. That is
// a client bug: replaying the original response would answer the wrong
// question, so it is refused (409) rather than silently mis-answered.
var ErrBodyMismatch = errors.New("idempotency: key reused with a different request body")

// Replay is a previously-recorded response, reconstructed in full: status,
// headers and body. Headers matter as much as the body — a replayed signup
// without its Set-Cookie returns an identical body while leaving the client
// convinced it is signed in with no session.
type Replay struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// sealedResponse is what gets encrypted. Body is []byte, which JSON encodes as
// base64 and decodes byte-exact — the "replay the ORIGINAL response" promise
// survives the round trip.
type sealedResponse struct {
	Header http.Header `json:"header"`
	Body   []byte      `json:"body"`
}

// aadFor binds a sealed record to the exact key it belongs to, so a row copied
// to another principal, endpoint, or key fails authentication instead of
// decrypting into someone else's response.
func aadFor(principal, endpoint, key string) []byte {
	return []byte("idem:" + principal + "\x1f" + endpoint + "\x1f" + key)
}

// Begin claims the key for this request.
//
// Returns (nil, token, nil) when the caller OWNS the key: it should execute the
// handler and pass `token` back to Complete. The token is the FENCE — only the
// current owner may record or release the claim, so an owner that stalled past
// the abandonment window cannot clobber the successor that took over.
//
// Returns (replay, "", nil) when this is a repeat of a completed request — the
// caller returns the replay verbatim with `Idempotent-Replay: true`. Returns
// ErrBodyMismatch for a reused key with a different body.
//
// A key claimed but not yet completed (an in-flight duplicate) reports a replay
// with StatusCode 0: the first request is still running, so there is no response
// to return yet and the caller must refuse rather than double-execute.
func (s *Service) Begin(ctx context.Context, principal, endpoint, key string, body []byte) (*Replay, string, error) {
	if key == "" {
		return nil, "", nil // no key supplied — not an idempotent request
	}
	if len(key) > 255 {
		return nil, "", fmt.Errorf("idempotency: key exceeds 255 characters")
	}
	sum := sha256.Sum256(body)
	fingerprint := hex.EncodeToString(sum[:])

	// One retry, for a narrow but real race: the claim can fail because someone
	// holds the key, and that holder can RELEASE it (a failed request) before
	// the follow-up read runs. Without the retry that legal sequence surfaces to
	// the client as a 500 on a perfectly valid retry.
	for attempt := 0; attempt < 2; attempt++ {
		token, err := newClaimToken()
		if err != nil {
			return nil, "", err
		}
		// Atomic claim: exactly one concurrent caller gets a row back.
		_, err = s.q.ClaimIdempotencyKey(ctx, store.ClaimIdempotencyKeyParams{
			Principal: principal, Endpoint: endpoint, Key: key,
			BodySha256: fingerprint, ClaimToken: token,
		})
		if err == nil {
			return nil, token, nil // we own it — execute the handler
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, "", err
		}

		// Someone else holds the key (or we already completed it).
		existing, err := s.q.GetIdempotencyKey(ctx, store.GetIdempotencyKeyParams{
			Principal: principal, Endpoint: endpoint, Key: key,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue // released between the claim and the read — try to claim it
		}
		if err != nil {
			return nil, "", err
		}
		if existing.BodySha256 != fingerprint {
			return nil, "", ErrBodyMismatch
		}
		r := &Replay{}
		if existing.StatusCode.Valid {
			r.StatusCode = int(existing.StatusCode.Int32)
		}
		// A row marked complete with NO payload is UNREADABLE, not a replay:
		// answering with the original status and a nil body — stamped
		// Idempotent-Replay — would be a lie sustained until the TTL. The
		// down/up migration cycle produces exactly this shape.
		if existing.StatusCode.Valid && len(existing.ResponseCiphertext) == 0 {
			err = fmt.Errorf("idempotency: record is marked complete but carries no payload")
		}
		if err == nil && len(existing.ResponseCiphertext) > 0 {
			resp, oerr := s.open(existing, principal, endpoint, key)
			err = oerr
			if err == nil {
				r.Header, r.Body = resp.Header, resp.Body
			}
		}
		if err != nil {
			// A record we cannot open (KEK rotated, or a row copied from
			// another key so its AAD no longer authenticates) is treated as
			// ABSENT. This is a TTL-bounded replay cache, not authoritative
			// state: erroring would 500 a legal retry, and returning the
			// status without its body would be a lie.
			//
			// It is DISCARDED rather than merely ignored, because nobody
			// can ever replay it — leaving it in place would strand the
			// client on a key it cannot use until the TTL expires, which is
			// exactly the stranding US-3.6 exists to prevent. The next loop
			// iteration then claims the key cleanly.
			// ERROR, not warn: every occurrence means a request that should
			// have been deduped will execute again. After a KEK rotation
			// this fires for every record in the table (see ADR-0013).
			slog.ErrorContext(ctx, "idempotency: recorded response is unreadable; discarding it so the key can be re-claimed — this request will re-execute",
				"endpoint", endpoint, "err", err)
			if _, derr := s.q.DiscardUnreadableIdempotencyKey(ctx, store.DiscardUnreadableIdempotencyKeyParams{
				Principal: principal, Endpoint: endpoint, Key: key,
				ClaimToken: existing.ClaimToken,
			}); derr != nil {
				return nil, "", derr
			}
			continue
		}
		return r, "", nil
	}
	// The key was released twice under us; treat it as in-flight rather than
	// executing, so a pathological loop can never double-provision.
	return &Replay{}, "", nil
}

// open decrypts a recorded response.
func (s *Service) open(row store.IdempotencyKey, principal, endpoint, key string) (sealedResponse, error) {
	sealed := secrets.Sealed{
		Ciphertext: row.ResponseCiphertext, Nonce: row.ResponseNonce,
		WrappedDEK: row.ResponseWrappedDek, DEKNonce: row.ResponseDekNonce,
		KEKID: row.ResponseKekID.String,
	}
	raw, err := secrets.Open(s.kek, sealed, aadFor(principal, endpoint, key))
	if err != nil {
		return sealedResponse{}, err
	}
	var out sealedResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return sealedResponse{}, fmt.Errorf("idempotency: decode recorded response: %w", err)
	}
	return out, nil
}

// newClaimToken identifies THIS claim so only its owner can record or release
// it. Random rather than sequential: it is a fence, and a guessable fence in a
// multi-tenant table is not a fence.
func newClaimToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("idempotency: claim token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Complete records the response so a later replay returns it verbatim. A
// non-2xx is deliberately NOT recorded: a failed create must be retryable with
// the same key (US-3.6 — a failure must never strand the client), so the key is
// released rather than pinning the failure forever.
// raw is the response body VERBATIM, not a value to re-marshal: S7 says a
// replay returns the original response, and re-marshalling could differ from
// what the client first received.
func (s *Service) Complete(ctx context.Context, principal, endpoint, key, claim string, status int, header http.Header, raw []byte) error {
	if key == "" || claim == "" {
		return nil
	}
	if status < 200 || status > 299 {
		// RELEASE the claim rather than merely not recording it. The row itself
		// is what blocks a retry (an unreleased claim reads as "still in
		// flight"), so leaving it behind would strand the client on its own
		// failed attempt until the claim aged out.
		// A 0-row release means a successor took the claim over mid-flight —
		// the SAME double-execution condition a 0-row save reports. Only the
		// recovery differs: the successor will complete the key, so this caller
		// is owed no error. It is still logged, because "my claim was taken
		// over" is the double-execution indicator whichever branch observes it.
		//
		// (The `status_code IS NULL` guard also makes a release a no-op against
		// an already-completed key, which protects the recorded response. The
		// middleware calls Complete exactly once per request, so that sequence
		// does not arise in production — it is reachable only by driving the
		// service directly, as TestIdempotencyReleaseCannotDestroyARecordedResponse
		// does.)
		n, err := s.q.ReleaseIdempotencyKey(ctx, store.ReleaseIdempotencyKeyParams{
			Principal: principal, Endpoint: endpoint, Key: key, ClaimToken: claim,
		})
		if err == nil && n == 0 {
			slog.WarnContext(ctx, "idempotency: claim was taken over before release; another execution of this request is live",
				"endpoint", endpoint)
		}
		return err
	}
	payload, err := json.Marshal(sealedResponse{Header: header, Body: raw})
	if err != nil {
		return fmt.Errorf("idempotency: encode response: %w", err)
	}
	sealed, err := secrets.Seal(s.kek, payload, aadFor(principal, endpoint, key))
	if err != nil {
		return fmt.Errorf("idempotency: seal response: %w", err)
	}
	n, err := s.q.SaveIdempotentResponse(ctx, store.SaveIdempotentResponseParams{
		Principal: principal, Endpoint: endpoint, Key: key, ClaimToken: claim,
		StatusCode:         pgInt4(status),
		ResponseCiphertext: sealed.Ciphertext, ResponseNonce: sealed.Nonce,
		ResponseWrappedDek: sealed.WrappedDEK, ResponseDekNonce: sealed.DEKNonce,
		ResponseKekID: pgText(sealed.KEKID),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		// A SAVE that matches no row IS dangerous: this request's response was
		// not recorded, so a retry will re-execute it. Unlike a release, there
		// is no benign reading — the row exists (we claimed it) and a
		// successor now owns it, meaning two executions of the same request.
		return ErrLostClaim
	}
	return nil
}

func pgInt4(v int) pgtype.Int4 { return pgtype.Int4{Int32: int32(v), Valid: true} }

func pgText(v string) pgtype.Text { return pgtype.Text{String: v, Valid: true} }

// RunSweeper deletes entries past the 24h window. The window is a PROMISE in
// the ruling, not merely a read filter: without this the table grows without
// bound and every recorded response body is retained indefinitely.
func (s *Service) RunSweeper(ctx context.Context, every time.Duration, log *slog.Logger) {
	sweep := func() {
		n, err := s.q.SweepExpiredIdempotencyKeys(ctx)
		if err != nil {
			log.Warn("idempotency sweep failed", "err", err)
			return
		}
		if n > 0 {
			log.Info("idempotency sweep", "expired", n)
		}
	}
	// Sweep ONCE at startup: a ticker does not fire immediately, so a service
	// that redeploys more often than the interval would otherwise never sweep
	// at all — the failure mode is invisible precisely because it looks idle.
	sweep()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sweep()
		}
	}
}

// fingerprintOf exposes the body fingerprint for tests that must construct a
// matching stored row.
func fingerprintOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
