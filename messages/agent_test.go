package messages_test

import (
	"testing"

	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/stretchr/testify/require"
)

func TestParseAgentRef(t *testing.T) {
	cases := []struct {
		in      string
		wantOK  bool
		name    string
		version string
	}{
		{"echo", true, "echo", ""},
		{"code-refactor", true, "code-refactor", ""},
		{"code-refactor@2.0.0", true, "code-refactor", "2.0.0"},
		{"code-refactor@2.0.0-beta+sha.abc", true, "code-refactor", "2.0.0-beta+sha.abc"},
		{"Foo", false, "", ""},
		{"foo@", false, "", ""},
		{"foo@bad/version", false, "", ""},
		{"", false, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			ref, err := messages.ParseAgentRef(tc.in)
			if !tc.wantOK {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.name, ref.Name)
			require.Equal(t, tc.version, ref.Version)
		})
	}
}

func FuzzParseAgentRef(f *testing.F) {
	seeds := []string{"echo", "code-refactor@1.0.0", "Foo", "foo@", "@1", "x", ""}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		_, _ = messages.ParseAgentRef(in)
	})
}
