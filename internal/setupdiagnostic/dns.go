// Package setupdiagnostic contains read-only helpers for account setup.
package setupdiagnostic

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/idna"
)

type DNSRequest struct {
	Domain    string
	Owned     bool
	Selectors []string
}

type DNSStatus string

const (
	DNSPassed    DNSStatus = "passed"
	DNSFailed    DNSStatus = "failed"
	DNSUnknown   DNSStatus = "unknown"
	DNSSkipped   DNSStatus = "skipped"
	DNSCancelled DNSStatus = "cancelled"
)

type DNSCheck struct {
	Query              string    `json:"query"`
	Resolver           string    `json:"resolver"`
	Observed           time.Time `json:"observedAt"`
	EvidenceAgeSeconds int       `json:"evidenceAgeSeconds"`
	TTL                *int64    `json:"ttlSeconds"`
	Status             DNSStatus `json:"status"`
	Evidence           string    `json:"evidence"`
}

type DNSResult struct {
	Domain string     `json:"domain"`
	Checks []DNSCheck `json:"checks"`
}

type txtRecord struct {
	value    string
	ttl      uint32
	ttlKnown bool
}

var errNXDomain = errors.New("DNS name does not exist")
var errNoData = errors.New("DNS TXT data absent")
var errAmbiguousNotFound = errors.New("system resolver cannot distinguish NXDOMAIN from NODATA")
var errCNAMEOnly = errors.New("DNS response contained a CNAME but no target TXT record")

type txtResolver interface {
	lookupTXT(context.Context, string) ([]txtRecord, error)
	resolverName() string
}

const dnsLookupTimeout = 5 * time.Second

func RunDNS(ctx context.Context, request DNSRequest) DNSResult {
	return runDNSWithResolver(ctx, request, systemTXTResolver())
}

func runDNSWithResolver(ctx context.Context, request DNSRequest, resolver txtResolver) DNSResult {
	domain, err := normalizeDomain(request.Domain)
	result := DNSResult{Domain: domain}
	name := resolver.resolverName()
	if err != nil {
		result.Domain = ""
	}
	if !request.Owned || err != nil {
		result.Checks = append(result.Checks, skippedCheck("_", name, "domain must be explicitly selected and valid"))
		return result
	}
	if providerManaged(domain) {
		result.Checks = append(result.Checks, skippedCheck(domain, name, "provider-managed consumer domain"))
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, dnsLookupTimeout)
	defer cancel()
	for _, item := range []struct{ query, kind string }{
		{domain, "spf"}, {"_dmarc." + domain, "dmarc"},
	} {
		result.Checks = append(result.Checks, lookupCheck(ctx, resolver, item.query, item.kind))
	}
	for i, selector := range request.Selectors {
		if i >= 10 {
			result.Checks = append(result.Checks, skippedCheck("_domainkey."+domain, name, "selector limit reached (10)"))
			break
		}
		selector = strings.TrimSpace(selector)
		if !validSelector(selector) {
			result.Checks = append(result.Checks, skippedCheck("<invalid-selector>._domainkey."+domain, name, "supplied selector is invalid"))
			continue
		}
		result.Checks = append(result.Checks, lookupCheck(ctx, resolver, selector+"._domainkey."+domain, "dkim"))
	}
	return result
}

func lookupCheck(ctx context.Context, resolver txtResolver, query, kind string) DNSCheck {
	check := DNSCheck{Query: query, Resolver: resolver.resolverName(), Observed: time.Now().UTC()}
	records, err := resolver.lookupTXT(ctx, query)
	if err != nil {
		switch {
		case errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled):
			check.Status, check.Evidence = DNSCancelled, "lookup cancelled"
		case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
			check.Status, check.Evidence = DNSUnknown, "lookup timed out"
		case isTimeout(err):
			check.Status, check.Evidence = DNSUnknown, "lookup timed out"
		case errors.Is(err, errNXDomain):
			check.Status, check.Evidence = DNSFailed, "name does not exist (NXDOMAIN)"
		case errors.Is(err, errNoData):
			check.Status, check.Evidence = DNSFailed, "TXT record absent (NODATA)"
		case errors.Is(err, errAmbiguousNotFound):
			check.Status, check.Evidence = DNSUnknown, "system resolver reports not found; NXDOMAIN versus NODATA unavailable"
		case errors.Is(err, errCNAMEOnly):
			check.Status, check.Evidence = DNSUnknown, "CNAME target TXT data was not available in the DNS response"
		default:
			check.Status, check.Evidence = DNSUnknown, "DNS lookup failed"
		}
		return check
	}
	matching := make([]txtRecord, 0, len(records))
	for _, record := range records {
		if candidate(record.value, kind) {
			matching = append(matching, record)
		}
	}
	if len(matching) == 0 {
		check.Status, check.Evidence = DNSFailed, "no matching TXT policy record (NODATA)"
		return check
	}
	var minTTL uint32
	ttlKnown := false
	for _, record := range matching {
		if record.ttlKnown && (!ttlKnown || record.ttl < minTTL) {
			minTTL, ttlKnown = record.ttl, true
		}
	}
	if ttlKnown {
		seconds := int64(minTTL)
		check.TTL = &seconds
	}
	if len(matching) != 1 {
		check.Status, check.Evidence = DNSFailed, "multiple matching policy TXT records"
		return check
	}
	if kind == "dkim" && revokedDKIM(matching[0].value) {
		check.Status, check.Evidence = DNSFailed, "DKIM key is revoked (empty p=); publish a valid public key"
		return check
	}
	if !validPolicy(matching[0].value, kind) {
		check.Status, check.Evidence = DNSFailed, "matching TXT record is malformed"
		return check
	}
	check.Status, check.Evidence = DNSPassed, "one structurally valid policy record found; semantic delivery behavior not evaluated"
	return check
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func candidate(value, kind string) bool {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return false
	}
	first := strings.TrimSuffix(fields[0], ";")
	switch kind {
	case "spf":
		return strings.HasPrefix(strings.ToLower(first), "v=spf1")
	case "dmarc":
		return strings.HasPrefix(strings.ToLower(first), "v=dmarc1")
	case "dkim":
		tags := parseTags(value)
		_, hasKey := tags["p"]
		if strings.HasPrefix(strings.ToLower(first), "v=dkim1") || hasKey {
			return true
		}
		for _, part := range strings.Split(value, ";") {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(part)), "p=") {
				return true
			}
		}
		return false
	}
	return false
}

var spfModifier = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)
var spfAddressMechanism = regexp.MustCompile(`(?i)^(?:a|mx)(?::[^/ ]+)?(?:/([0-9]{1,2}))?(?://([0-9]{1,3}))?$`)

func validPolicy(value, kind string) bool {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return false
	}
	switch kind {
	case "spf":
		if !strings.EqualFold(fields[0], "v=spf1") {
			return false
		}

		for _, field := range fields[1:] {
			if field == "" {
				return false
			}
			term := field
			if strings.Contains(term, "=") {
				key, value, _ := strings.Cut(term, "=")
				if !spfModifier.MatchString(key) || value == "" {
					return false
				}
				continue
			}
			if strings.ContainsRune("+-~?", rune(term[0])) {
				term = term[1:]
			}
			if term == "" {
				return false
			}
			if m := spfAddressMechanism.FindStringSubmatch(term); m != nil {
				if m[1] != "" {
					n, _ := strconv.Atoi(m[1])
					if n > 32 {
						return false
					}
				}
				if m[2] != "" {
					n, _ := strconv.Atoi(m[2])
					if n > 128 {
						return false
					}
				}
				continue
			}
			name, arg, hasArg := strings.Cut(term, ":")
			name = strings.ToLower(name)
			switch name {
			case "all":
				if hasArg || strings.Contains(term, "/") {
					return false
				}
			case "include", "exists":
				if !hasArg || arg == "" {
					return false
				}
			case "ip4", "ip6":
				addr, err := netip.ParseAddr(arg)
				if strings.Contains(arg, "/") {
					var prefix netip.Prefix
					prefix, err = netip.ParsePrefix(arg)
					addr = prefix.Addr()
				}
				if !hasArg || err != nil || name == "ip4" && !addr.Is4() || name == "ip6" && !addr.Is6() {
					return false
				}
			case "a", "mx":
				if hasArg && arg == "" {
					return false
				}
			case "ptr":
				if hasArg && arg == "" {
					return false
				}
			default:
				return false
			}
		}
		return true
	case "dmarc":
		tags := parseTags(value)
		if !strings.EqualFold(tags["v"], "DMARC1") {
			return false
		}
		p := tags["p"]
		return strings.EqualFold(p, "none") || strings.EqualFold(p, "quarantine") || strings.EqualFold(p, "reject")
	case "dkim":
		tags := parseTags(value)
		if version, exists := tags["v"]; exists && !strings.EqualFold(version, "DKIM1") {
			return false
		}
		key, exists := tags["p"]
		return exists && key != "" && validBase64(key)
	}
	return false
}

func parseTags(value string) map[string]string {
	tags := make(map[string]string)
	for _, part := range strings.Split(value, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			return nil
		}
		key, val = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(val)
		if _, exists := tags[key]; exists {
			return nil
		}
		tags[key] = val
	}
	return tags
}

func revokedDKIM(value string) bool {
	key, exists := parseTags(value)["p"]
	return exists && key == ""
}

func validBase64(s string) bool {
	if len(s) < 4 {
		return false
	}
	_, standardErr := base64.StdEncoding.DecodeString(s)
	_, rawErr := base64.RawStdEncoding.DecodeString(s)
	return standardErr == nil || rawErr == nil
}

func skippedCheck(query, resolver, evidence string) DNSCheck {
	return DNSCheck{Query: query, Resolver: resolver, Observed: time.Now().UTC(), Status: DNSSkipped, Evidence: evidence}
}

func normalizeDomain(domain string) (string, error) {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	ascii, err := idna.Lookup.ToASCII(domain)
	if err != nil || !validDomain(ascii) {
		return domain, errors.New("invalid domain")
	}
	return ascii, nil
}

func validDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 || strings.Contains(domain, "..") {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

func validSelector(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func providerManaged(domain string) bool {
	for _, suffix := range []string{"gmail.com", "googlemail.com", "outlook.com", "hotmail.com", "live.com", "msn.com", "outlook.fr", "outlook.de", "hotmail.co.uk", "hotmail.fr", "hotmail.de", "live.co.uk", "live.fr", "yahoo.com", "yahoo.co.uk", "ymail.com", "rocketmail.com", "aol.com", "icloud.com", "me.com", "mac.com", "proton.me", "protonmail.com", "pm.me"} {
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			return true
		}
	}
	return false
}

type wireTXTResolver struct{ servers []string }

func (r wireTXTResolver) resolverName() string {
	if len(r.servers) == 0 {
		return "system resolver unavailable"
	}
	return strings.Join(r.servers, ", ")
}

func systemTXTResolver() txtResolver {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return fallbackTXTResolver{}
	}
	var servers []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			servers = append(servers, net.JoinHostPort(strings.Trim(fields[1], "[]"), "53"))
		}
	}
	if len(servers) == 0 {
		return fallbackTXTResolver{}
	}
	return wireTXTResolver{servers: servers}
}

type fallbackTXTResolver struct{}

func (fallbackTXTResolver) resolverName() string {
	return "Go system resolver (TTL and NXDOMAIN/NODATA distinction unavailable)"
}

func (fallbackTXTResolver) lookupTXT(ctx context.Context, query string) ([]txtRecord, error) {
	values, err := net.DefaultResolver.LookupTXT(ctx, query)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return nil, errAmbiguousNotFound
		}
		return nil, err
	}
	records := make([]txtRecord, 0, len(values))
	for _, value := range values {
		records = append(records, txtRecord{value: value})
	}
	if len(records) == 0 {
		return nil, errAmbiguousNotFound
	}
	return records, nil
}

func (r wireTXTResolver) lookupTXT(ctx context.Context, query string) ([]txtRecord, error) {
	if len(r.servers) == 0 {
		return nil, errors.New("system DNS resolver configuration unavailable")
	}
	name, err := dnsmessage.NewName(strings.TrimSuffix(query, ".") + ".")
	if err != nil {
		return nil, err
	}
	var randomID [2]byte
	if _, err = rand.Read(randomID[:]); err != nil {
		return nil, err
	}
	id := binary.BigEndian.Uint16(randomID[:])
	request := dnsmessage.Message{Header: dnsmessage.Header{ID: id, RecursionDesired: true}, Questions: []dnsmessage.Question{{Name: name, Type: dnsmessage.TypeTXT, Class: dnsmessage.ClassINET}}}
	packet, err := request.Pack()
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, server := range r.servers {
		response, err := exchangeDNS(ctx, server, packet, false)
		if err != nil {
			lastErr = err
			continue
		}
		message, err := parseResponse(response, id, name)
		if err != nil {
			lastErr = err
			continue
		}
		if message.Truncated {
			response, err = exchangeDNS(ctx, server, packet, true)
			if err != nil {
				lastErr = err
				continue
			}
			message, err = parseResponse(response, id, name)
			if err != nil {
				lastErr = err
				continue
			}
		}
		if message.RCode == dnsmessage.RCodeNameError {
			return nil, errNXDomain
		}
		if message.RCode != dnsmessage.RCodeSuccess {
			return nil, fmt.Errorf("DNS response code %s", message.RCode)
		}
		owners := map[string]bool{dnsNameKey(name): true}
		for i := 0; i < 8; i++ {
			changed := false
			for _, answer := range message.Answers {
				if answer.Header.Type != dnsmessage.TypeCNAME || !owners[dnsNameKey(answer.Header.Name)] {
					continue
				}
				if body, ok := answer.Body.(*dnsmessage.CNAMEResource); ok {
					key := dnsNameKey(body.CNAME)
					if !owners[key] {
						owners[key] = true
						changed = true
					}
				}
			}
			if !changed {
				break
			}
		}
		hasCNAME := len(owners) > 1
		records := make([]txtRecord, 0)
		for _, answer := range message.Answers {
			if answer.Header.Type != dnsmessage.TypeTXT || !owners[dnsNameKey(answer.Header.Name)] {
				continue
			}
			if body, ok := answer.Body.(*dnsmessage.TXTResource); ok {
				records = append(records, txtRecord{value: strings.Join(body.TXT, ""), ttl: answer.Header.TTL, ttlKnown: true})
			}
		}
		if len(records) == 0 {
			if hasCNAME {
				return nil, errCNAMEOnly
			}
			return nil, errNoData
		}
		return records, nil
	}
	if lastErr == nil {
		lastErr = errors.New("DNS query failed")
	}
	return nil, lastErr
}

func exchangeDNS(ctx context.Context, server string, packet []byte, tcp bool) ([]byte, error) {
	network := "udp"
	if tcp {
		network = "tcp"
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, network, server)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if tcp {
		if err := writeAll(conn, []byte{byte(len(packet) >> 8), byte(len(packet))}); err != nil {
			return nil, err
		}
		if err := writeAll(conn, packet); err != nil {
			return nil, err
		}
		var size [2]byte
		if _, err := io.ReadFull(conn, size[:]); err != nil {
			return nil, err
		}
		packet = make([]byte, int(binary.BigEndian.Uint16(size[:])))
		if _, err := io.ReadFull(conn, packet); err != nil {
			return nil, err
		}
		return packet, nil
	}
	if err := writeAll(conn, packet); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	readDeadline := time.Now().Add(2 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(readDeadline) {
		readDeadline = ctxDeadline
	}
	_ = conn.SetReadDeadline(readDeadline)
	n, err := conn.Read(buf)
	return buf[:n], err
}

func writeAll(conn net.Conn, b []byte) error {
	for len(b) > 0 {
		n, err := conn.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

func parseResponse(packet []byte, id uint16, question dnsmessage.Name) (dnsmessage.Message, error) {
	var message dnsmessage.Message
	if err := message.Unpack(packet); err != nil {
		return message, err
	}
	if !message.Response || message.ID != id || len(message.Questions) != 1 ||
		!strings.EqualFold(dnsNameKey(message.Questions[0].Name), dnsNameKey(question)) ||
		message.Questions[0].Type != dnsmessage.TypeTXT || message.Questions[0].Class != dnsmessage.ClassINET {
		return message, errors.New("invalid DNS response")
	}
	return message, nil
}

func dnsNameKey(name dnsmessage.Name) string {
	return strings.TrimSuffix(strings.ToLower(name.String()), ".")
}
