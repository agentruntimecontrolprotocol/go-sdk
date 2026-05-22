package credentials_test

import (
	"context"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/credentials"
	"github.com/stretchr/testify/require"
)

func TestMemoryIssueRevoke(t *testing.T) {
	exp := time.Now().Add(time.Hour).UTC()
	mem := credentials.NewMemory("test-")
	creds, err := mem.Issue(context.Background(), credentials.IssueRequest{
		JobID:     "job_1",
		Principal: "alice",
		Agent:     "echo",
		Lease: arcp.Lease{
			arcp.CapModelUse: {"tier-fast/*"},
		},
		Budget:    map[arcp.Currency]float64{"USD": 1.25},
		ExpiresAt: &exp,
	})
	require.NoError(t, err)
	require.Len(t, creds, 1)
	require.Equal(t, "test-1", creds[0].ID)
	require.Equal(t, "bearer", creds[0].Scheme)
	require.Equal(t, []string{"USD:1.25"}, creds[0].Constraints.CostBudget)
	require.Equal(t, []string{"tier-fast/*"}, creds[0].Constraints.ModelUse)
	require.Equal(t, 1, mem.Outstanding())

	require.NoError(t, mem.Revoke(context.Background(), "test-1"))
	require.NoError(t, mem.Revoke(context.Background(), "test-1"))
	require.Equal(t, 0, mem.Outstanding())
}

func TestBudgetExhaustedMaps(t *testing.T) {
	require.Equal(t, arcp.CodeBudgetExhausted, arcp.Code(credentials.BudgetExhausted))
}
