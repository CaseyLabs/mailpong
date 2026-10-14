package setupdiagnostic

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

type fakeTXTResolver struct {
	values  map[string][]txtRecord
	errors  map[string]error
	queries []string
}

func (f *fakeTXTResolver) resolverName() string { return "test resolver" }
func (f *fakeTXTResolver) lookupTXT(_ context.Context, query string) ([]txtRecord, error) {
	f.queries = append(f.queries, query)
	return f.values[query], f.errors[query]
}

func TestRunDNSChecksOnlyRequestedRecordsAndRedactsValues(t *testing.T) {
	domain := "example.com"
	secret := "v=spf1 include:private-token.example ~all"
	r := &fakeTXTResolver{values: map[string][]txtRecord{
		domain:                      {{value: secret, ttl: 120, ttlKnown: true}, {value: "unrelated verification=opaque", ttl: 10, ttlKnown: true}},
		"_dmarc." + domain:          {{value: "v=DMARC1; p=reject", ttl: 180, ttlKnown: true}},
		"mail._domainkey." + domain: {{value: "v=DKIM1; p=QUJDRA==", ttl: 60, ttlKnown: true}},
	}}
	out := runDNSWithResolver(context.Background(), DNSRequest{Domain: domain, Owned: true, Selectors: []string{"mail"}}, r)
	if len(out.Checks) != 3 || len(r.queries) != 3 {
		t.Fatalf("expected SPF, DMARC, and one supplied DKIM query; got %#v", r.queries)
	}
	for i, check := range out.Checks {
		if check.Status != DNSPassed || check.Resolver != "test resolver" || check.TTL == nil || check.Observed.Location() != time.UTC {
			t.Errorf("unexpected check metadata: %+v", check)
		}
		if check.TTL != nil && *check.TTL != []int64{120, 180, 60}[i] {
			t.Errorf("TTL = %v", check.TTL)
		}
	}
	serialized := out.Checks[0].Evidence + out.Checks[2].Evidence
	if strings.Contains(serialized, "private-token") || strings.Contains(serialized, "QUJD") {
		t.Fatal("DNS evidence leaked TXT contents")
	}
}

func TestRunDNSClassifiesDNSOutcomesAndMalformedPolicies(t *testing.T) {
	domain := "example.com"
	r := &fakeTXTResolver{
		values: map[string][]txtRecord{
			domain:                         {{value: "v=spf1garbage include:x", ttl: 30, ttlKnown: true}},
			"_dmarc." + domain:             {{value: "v=DMARC1; p=bogus", ttl: 30, ttlKnown: true}},
			"missing._domainkey." + domain: {},
			"multi._domainkey." + domain:   {{value: "v=DKIM1; p=QUJD", ttl: 5, ttlKnown: true}, {value: "v=DKIM1; p=REVG", ttl: 5, ttlKnown: true}},
			"bad._domainkey." + domain:     {{value: "v=DKIM1; p=not!base64", ttl: 5, ttlKnown: true}},
			"revoked._domainkey." + domain: {{value: "v=DKIM1; p=", ttl: 5, ttlKnown: true}},
		},
		errors: map[string]error{
			"nx._domainkey." + domain:      errNXDomain,
			"timeout._domainkey." + domain: context.DeadlineExceeded,
		},
	}
	selectors := []string{"missing", "nx", "timeout", "multi", "bad", "revoked"}
	out := runDNSWithResolver(context.Background(), DNSRequest{Domain: domain, Owned: true, Selectors: selectors}, r)
	want := []DNSStatus{DNSFailed, DNSFailed, DNSFailed, DNSFailed, DNSUnknown, DNSFailed, DNSFailed, DNSFailed}
	for i := range want {
		if out.Checks[i].Status != want[i] {
			t.Errorf("check %d status = %q; want %q (%s)", i, out.Checks[i].Status, want[i], out.Checks[i].Evidence)
		}
	}
}

func TestCompactDMARCTagsAndSPFMalformedTail(t *testing.T) {
	domain := "example.com"
	r := &fakeTXTResolver{values: map[string][]txtRecord{
		domain:             {{value: "v=spf1 include:mail.example broken!", ttl: 10, ttlKnown: true}},
		"_dmarc." + domain: {{value: "v=DMARC1;p=reject", ttl: 20, ttlKnown: true}},
	}}
	out := runDNSWithResolver(context.Background(), DNSRequest{Domain: domain, Owned: true}, r)
	if out.Checks[0].Status != DNSFailed {
		t.Fatalf("malformed SPF tail status = %q", out.Checks[0].Status)
	}
	if out.Checks[1].Status != DNSPassed {
		t.Fatalf("compact DMARC status = %q", out.Checks[1].Status)
	}
}

func TestRunDNSSkipsUnownedProviderAndCapsSelectors(t *testing.T) {
	r := &fakeTXTResolver{values: map[string][]txtRecord{}, errors: map[string]error{}}
	for _, tc := range []struct {
		request     DNSRequest
		wantQueries int
	}{
		{DNSRequest{Domain: "example.com", Owned: false}, 0},
		{DNSRequest{Domain: "mail.googlemail.com", Owned: true}, 0},
		{DNSRequest{Domain: "example.com", Owned: true, Selectors: []string{"bad.selector"}}, 2},
		{DNSRequest{Domain: "example.com", Owned: true, Selectors: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}}, 12},
	} {
		before := len(r.queries)
		_ = runDNSWithResolver(context.Background(), tc.request, r)
		if got := len(r.queries) - before; got != tc.wantQueries {
			t.Errorf("queries = %d; want %d", got, tc.wantQueries)
		}
	}
}

func TestWireTXTResolverDistinguishesNXDomainNODATAAndTTL(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rcode   dnsmessage.RCode
		txt     []string
		wantErr error
		wantTTL uint32
	}{
		{name: "nxdomain", rcode: dnsmessage.RCodeNameError, wantErr: errNXDomain},
		{name: "nodata", rcode: dnsmessage.RCodeSuccess, wantErr: errNoData},
		{name: "txt", rcode: dnsmessage.RCodeSuccess, txt: []string{"opaque"}, wantTTL: 77},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			go func() {
				buf := make([]byte, 2048)
				n, addr, readErr := server.ReadFrom(buf)
				if readErr != nil {
					return
				}
				var request dnsmessage.Message
				if request.Unpack(buf[:n]) != nil || len(request.Questions) != 1 {
					return
				}
				response := dnsmessage.Message{Header: dnsmessage.Header{ID: request.ID, Response: true, RCode: tc.rcode}, Questions: request.Questions}
				if len(tc.txt) > 0 {
					response.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: request.Questions[0].Name, Class: dnsmessage.ClassINET, TTL: tc.wantTTL}, Body: &dnsmessage.TXTResource{TXT: tc.txt}}}
				}
				wire, packErr := response.Pack()
				if packErr == nil {
					_, _ = server.WriteTo(wire, addr)
				}
			}()
			resolver := wireTXTResolver{servers: []string{server.LocalAddr().String()}}
			records, gotErr := resolver.lookupTXT(context.Background(), "example.com")
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("error = %v; want %v", gotErr, tc.wantErr)
			}
			if tc.wantTTL != 0 && (len(records) != 1 || records[0].ttl != tc.wantTTL) {
				t.Fatalf("records = %+v", records)
			}
		})
	}
}

func TestDNSConsumerDomainsAndPolicySyntax(t *testing.T) {
	for _, domain := range []string{"gmail.com", "hotmail.co.uk", "outlook.fr", "icloud.com", "yahoo.com", "proton.me"} {
		r := &fakeTXTResolver{}
		result := runDNSWithResolver(context.Background(), DNSRequest{Domain: domain, Owned: true}, r)
		if len(r.queries) != 0 || result.Checks[0].Status != DNSSkipped {
			t.Fatalf("queried provider domain %s", domain)
		}
	}
	for _, tc := range []struct {
		value, kind string
		valid       bool
	}{
		{"v=spf1 a/24 mx//64 -all", "spf", true},
		{"v=spf1 redirect=example.com", "spf", true},
		{"v=spf1 ip4:garbage -all", "spf", false},
		{"v=spf1 ip4:192.0.2.1/24 ip6:2001:db8::/32 -all", "spf", true},
		{"v=spf1 a/33 -all", "spf", false},
		{"v=spf1garbage", "spf", false},
		{"v=DMARC1;p=none;p=reject", "dmarc", false},
		{"v=DKIM1;p=YWJjZA==;p=", "dkim", false},
	} {
		if got := validPolicy(tc.value, tc.kind); got != tc.valid {
			t.Errorf("%s valid=%v", tc.value, got)
		}
	}
}

func TestDNSDuplicateTagsAreMalformedNotAbsent(t *testing.T) {
	for _, tc := range []struct{ kind, value string }{
		{"dmarc", "v=DMARC1;p=none;p=reject"},
		{"dkim", "v=DKIM1;p=YWJjZA==;p="},
	} {
		r := &fakeTXTResolver{values: map[string][]txtRecord{"example.com": {{value: tc.value}}}}
		c := lookupCheck(context.Background(), r, "example.com", tc.kind)
		if c.Status != DNSFailed || !strings.Contains(c.Evidence, "malformed") {
			t.Fatalf("%+v", c)
		}
	}
}
