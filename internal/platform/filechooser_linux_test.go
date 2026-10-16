package platform

import (
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestPortalFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters []FileFilter
		want    []portalFilter
	}{
		{"none", nil, nil},
		{
			"single glob",
			[]FileFilter{{DisplayName: "JSON files", Pattern: "*.json"}},
			[]portalFilter{{Name: "JSON files", Rules: []portalFilterRule{{0, "*.json"}}}},
		},
		{
			"semicolon list with blanks",
			[]FileFilter{{DisplayName: "Certs", Pattern: "*.pem; *.crt;;"}},
			[]portalFilter{{Name: "Certs", Rules: []portalFilterRule{{0, "*.pem"}, {0, "*.crt"}}}},
		},
		{
			"empty pattern dropped",
			[]FileFilter{{DisplayName: "Nothing", Pattern: " ; "}},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := portalFilters(tt.filters); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("portalFilters() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// The FileChooser portal rejects filters unless they marshal as a(sa(us)).
func TestPortalFiltersSignature(t *testing.T) {
	pf := portalFilters([]FileFilter{{DisplayName: "JSON", Pattern: "*.json"}})
	if got := dbus.SignatureOf(pf).String(); got != "a(sa(us))" {
		t.Errorf("filter signature = %q, want a(sa(us))", got)
	}
}
