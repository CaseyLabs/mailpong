package sync

import (
	"reflect"
	"testing"
)

func TestForwardingAddresses(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"no headers", "", nil},
		{"alias in X-Forwarded-For ahead of Delivered-To",
			"Delivered-To: seancasey@gmail.com\r\nX-Forwarded-For: sean@caseylabs.com seancasey@gmail.com\r\n\r\n",
			[]string{"sean@caseylabs.com", "seancasey@gmail.com"}},
		{"X-Original-To and angle brackets", "X-Original-To: <Alias@X.com>\r\n\r\n", []string{"alias@x.com"}},
		{"multiple Delivered-To deduplicated", "Delivered-To: a@x.com\r\nDelivered-To: A@x.com\r\nDelivered-To: b@x.com\r\n\r\n", []string{"a@x.com", "b@x.com"}},
		{"unrelated headers give empty list", "Subject: hi\r\n\r\n", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := forwardingAddresses([]byte(tc.raw))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("forwardingAddresses = %#v, want %#v", got, tc.want)
			}
		})
	}
}
