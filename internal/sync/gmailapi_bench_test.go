package sync

import (
	"fmt"
	"slices"
	"testing"
)

func gmailIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%016x", uint64(i)*2654435761)
	}
	return ids
}

func TestLocalOnly(t *testing.T) {
	for _, tc := range []struct {
		name          string
		local, remote []string
		want          []string
	}{
		{"empty", nil, nil, nil},
		{"all remote", []string{"a", "b"}, []string{"b", "a"}, nil},
		{"none remote", []string{"a", "b"}, nil, []string{"a", "b"}},
		{"mixed with duplicates", []string{"a", "b", "c"}, []string{"c", "c", "a", "z"}, []string{"b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := localOnly(tc.local, tc.remote); !slices.Equal(got, tc.want) {
				t.Fatalf("localOnly = %v, want %v", got, tc.want)
			}
		})
	}
}

// BenchmarkGmailDiff measures the reconcile diff over a 1M-message mailbox where
// 1% of local IDs vanished (SY4): the map approach it replaced against localOnly.
func BenchmarkGmailDiff(b *testing.B) {
	remote := gmailIDs(1_000_000)
	local := slices.Clone(remote[:990_000])
	local = append(local, gmailIDs(1_010_000)[1_000_000:]...)
	b.Run("map", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			in := make(map[string]bool, len(remote))
			for _, id := range remote {
				in[id] = true
			}
			var out []string
			for _, id := range local {
				if !in[id] {
					out = append(out, id)
				}
			}
			_ = out
		}
	})
	b.Run("sorted", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = localOnly(local, slices.Clone(remote))
		}
	})
}
