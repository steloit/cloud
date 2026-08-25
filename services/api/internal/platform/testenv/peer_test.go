package testenv_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/steloit/cloud/services/api/internal/platform/testenv"
)

// O34's guard must FIRE on a squatter. A check that can only stay quiet is the
// defect it exists to prevent, one level up.
//
// TWO stand-ins, because two different things were actually measured across 168
// containers: a peer that immediately writes `HTTP/` (seen twice), and one that
// accepts and then says nothing. The first is the case that produced the
// production symptom — pgx reads four of those bytes as an int32 message length
// and reports "invalid body length ... 1414811691".
func TestCheckPostgresPeerRejectsASquatter(t *testing.T) {
	cases := []struct {
		name string
		// onAccept is what the squatter does with the connection.
		onAccept func(net.Conn)
		wantIn   string
	}{
		{
			name: "answers HTTP, as two of the three measured collisions did",
			onAccept: func(c net.Conn) {
				_, _ = c.Write([]byte("HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n"))
			},
			wantIn: "is NOT this container's Postgres",
		},
		{
			name:     "accepts and says nothing",
			onAccept: func(c net.Conn) { time.Sleep(8 * time.Second) },
			wantIn:   "said nothing",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			go func() {
				for {
					c, err := ln.Accept()
					if err != nil {
						return
					}
					go func() { defer c.Close(); tc.onAccept(c) }()
				}
			}()

			port := ln.Addr().(*net.TCPAddr).Port
			err = testenv.CheckPostgresPeer(fmt.Sprintf("postgres://app:app@127.0.0.1:%d/app?sslmode=disable", port))
			if err == nil {
				t.Fatalf("a squatter on :%d was accepted as Postgres — the guard is inert, and O34 "+
					"would still surface as an unexplained \"invalid body length\"", port)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("the refusal does not mention %q, so it does not explain itself:\n%v", tc.wantIn, err)
			}
			// Whatever the branch, it must NAME the process holding the port —
			// "something else has it" leaves the next reader where this started.
			if !strings.Contains(err.Error(), "lsof -nP -iTCP:") {
				t.Fatalf("the refusal does not identify the port's holder:\n%v", err)
			}
		})
	}
}

// And it must be INERT against a real one, or every integration test fails.
func TestCheckPostgresPeerAcceptsARealPostgres(t *testing.T) {
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("app"), tcpostgres.WithUsername("app"), tcpostgres.WithPassword("app"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		testenv.SkipOrFail(t, err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = pg.Terminate(c)
	})
	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := testenv.CheckPostgresPeer(url); err != nil {
		t.Fatalf("a real Postgres was rejected — the guard would fail every integration test: %v", err)
	}
}
