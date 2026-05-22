package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/credentials"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/clock"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
	"github.com/stretchr/testify/require"
)

func credentialPair(t *testing.T, mem *credentials.Memory, fn server.AgentFunc) (*server.Server, *client.Client, func()) {
	t.Helper()
	srv := server.New(server.Options{Provisioner: mem})
	srv.RegisterAgent("echo", fn)
	a, b := transport.NewMemoryPair()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Accept(ctx, b) }()
	cli, err := client.Connect(ctx, a, client.Options{Token: "demo"})
	require.NoError(t, err)
	return srv, cli, func() {
		_ = cli.Close(context.Background())
		_ = srv.Close()
		cancel()
	}
}

func TestProvisionedCredentialsFeatureNegotiation(t *testing.T) {
	_, cli, cleanup := newPair(t)
	defer cleanup()
	require.False(t, cli.HasFeature("provisioned_credentials"))
	require.False(t, cli.HasFeature("model.use"))

	mem := credentials.NewMemory("feat-")
	_, cli2, cleanup2 := credentialPair(t, mem, func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	defer cleanup2()
	require.True(t, cli2.HasFeature("provisioned_credentials"))
	require.True(t, cli2.HasFeature("model.use"))
}

func TestCredentialsInJobAccepted(t *testing.T) {
	mem := credentials.NewMemory("cred-")
	_, cli, cleanup := credentialPair(t, mem, func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exp := time.Now().Add(time.Hour).UTC()

	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent: "echo",
		LeaseRequest: arcp.Lease{
			arcp.CapModelUse:   {"tier-fast/*"},
			arcp.CapCostBudget: {"USD:2"},
		},
		LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &exp},
	})
	require.NoError(t, err)
	require.Len(t, h.Accepted().Credentials, 1)
	cred := h.Accepted().Credentials[0]
	require.Equal(t, "cred-1", cred.ID)
	require.Equal(t, "bearer", cred.Scheme)
	require.Equal(t, []string{"tier-fast/*"}, cred.Constraints.ModelUse)
	require.Equal(t, []string{"USD:2"}, cred.Constraints.CostBudget)
	require.Len(t, mem.Issued(), 1)
	require.Equal(t, h.ID(), mem.Issued()[0].JobID)
}

func TestCredentialsRevokedOnSuccess(t *testing.T) {
	mem := credentials.NewMemory("ok-")
	_, cli, cleanup := credentialPair(t, mem, func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo"})
	require.NoError(t, err)
	_, err = h.Wait(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, mem.Outstanding())
	require.Equal(t, []string{"ok-1"}, mem.Revoked())
}

func TestCredentialsRevokedOnCancel(t *testing.T) {
	mem := credentials.NewMemory("cancel-")
	_, cli, cleanup := credentialPair(t, mem, func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo"})
	require.NoError(t, err)
	require.NoError(t, h.Cancel(ctx, "stop"))
	_, err = h.Wait(ctx)
	require.True(t, errors.Is(err, arcp.ErrCancelled), "got %v", err)
	require.Equal(t, 0, mem.Outstanding())
	require.Equal(t, []string{"cancel-1"}, mem.Revoked())
}

func TestCredentialsRevokedOnTimeout(t *testing.T) {
	mem := credentials.NewMemory("timeout-")
	_, cli, cleanup := credentialPair(t, mem, func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo", MaxRuntimeSec: 1})
	require.NoError(t, err)
	_, err = h.Wait(ctx)
	require.True(t, errors.Is(err, arcp.ErrTimeout), "got %v", err)
	require.Equal(t, 0, mem.Outstanding())
	require.Equal(t, []string{"timeout-1"}, mem.Revoked())
}

func TestCredentialsRevokedOnLeaseExpired(t *testing.T) {
	mem := credentials.NewMemory("lease-")
	now := time.Now().UTC()
	clk := clock.NewMock(now)
	srv := server.New(server.Options{Provisioner: mem, Clock: clk})
	srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	a, b := transport.NewMemoryPair()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = srv.Accept(ctx, b) }()
	cli, err := client.Connect(ctx, a, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer cli.Close(ctx)
	defer srv.Close()

	exp := now.Add(time.Second)
	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent:            "echo",
		LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &exp},
	})
	require.NoError(t, err)
	clk.Advance(2 * time.Second)
	_, err = h.Wait(ctx)
	require.True(t, errors.Is(err, arcp.ErrLeaseExpired), "got %v", err)
	require.Equal(t, 0, mem.Outstanding())
	require.Equal(t, []string{"lease-1"}, mem.Revoked())
}

func TestCredentialsRedactedForOtherPrincipal(t *testing.T) {
	mem := credentials.NewMemory("redact-")
	srv := server.New(server.Options{Provisioner: mem})
	srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a1, b1 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(ctx, b1) }()
	alice, err := client.Connect(ctx, a1, client.Options{ClientName: "alice", Token: "demo"})
	require.NoError(t, err)
	defer alice.Close(ctx)
	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(ctx, b2) }()
	bob, err := client.Connect(ctx, a2, client.Options{ClientName: "bob", Token: "demo"})
	require.NoError(t, err)
	defer bob.Close(ctx)

	h, err := alice.Submit(ctx, client.SubmitRequest{Agent: "echo"})
	require.NoError(t, err)
	require.NotEmpty(t, h.Accepted().Credentials)
	jobs, err := bob.ListJobs(ctx, client.ListJobsRequest{})
	require.NoError(t, err)
	raw, err := json.Marshal(jobs)
	require.NoError(t, err)
	require.False(t, strings.Contains(string(raw), "credentials"))
	require.Empty(t, jobs.Jobs)
}

func TestCredentialRotated(t *testing.T) {
	mem := credentials.NewMemory("rot-")
	_, cli, cleanup := credentialPair(t, mem, func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		require.NoError(t, jc.RotateCredential("rot-1", "rotated-secret"))
		return map[string]bool{"ok": true}, nil
	})
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo"})
	require.NoError(t, err)
	var rotated messages.StatusBody
	for ev := range h.Events() {
		if ev.Kind != messages.KindStatus {
			continue
		}
		require.NoError(t, json.Unmarshal(ev.Body, &rotated))
		if rotated.Phase == messages.PhaseCredentialRotated {
			break
		}
	}
	_, err = h.Wait(ctx)
	require.NoError(t, err)
	require.Equal(t, messages.PhaseCredentialRotated, rotated.Phase)
	require.Equal(t, "rot-1", rotated.Details["id"])
	require.Equal(t, "rotated-secret", rotated.Details["value"])
	require.Contains(t, mem.Revoked(), "rot-1")
}
