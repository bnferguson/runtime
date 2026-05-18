package coordinate

import (
	"net"
	"strconv"

	"miren.dev/runtime/pkg/cloudauth"
)

// SourcedIP is an IP address tagged with how it was obtained. Explicit IPs
// (user-configured via AdditionalIPs or the server config) always pass
// through to the advertised list. Discovered IPs (auto-scanned from local
// interfaces) are subject to netcheck pruning, CGNAT filtering, etc.
type SourcedIP struct {
	IP       net.IP
	Explicit bool // true = user-configured, false = auto-discovered
}

// IPSet is an ordered, de-duplicated collection of SourcedIP entries.
// When a duplicate IP is added, the Explicit flag is sticky: adding an
// IP as explicit promotes a previously-discovered entry, but adding it
// as discovered never demotes an explicit one. Iteration order matches
// first-insertion order.
type IPSet struct {
	entries []SourcedIP
	index   map[string]int // IP string → index into entries
}

// NewIPSet creates an empty IPSet.
func NewIPSet() *IPSet {
	return &IPSet{index: make(map[string]int)}
}

// Add inserts an IP. If the IP already exists and the new entry is
// explicit, it promotes the existing entry. Discovered duplicates are
// silently ignored.
func (s *IPSet) Add(sip SourcedIP) {
	if sip.IP == nil {
		return
	}
	key := sip.IP.String()
	if i, ok := s.index[key]; ok {
		if sip.Explicit && !s.entries[i].Explicit {
			s.entries[i].Explicit = true
		}
		return
	}
	s.index[key] = len(s.entries)
	s.entries = append(s.entries, sip)
}

// AddDiscovered is a convenience for Add(SourcedIP{IP: ip, Explicit: false}).
func (s *IPSet) AddDiscovered(ip net.IP) {
	s.Add(SourcedIP{IP: ip, Explicit: false})
}

// AddExplicit is a convenience for Add(SourcedIP{IP: ip, Explicit: true}).
func (s *IPSet) AddExplicit(ip net.IP) {
	s.Add(SourcedIP{IP: ip, Explicit: true})
}

// All returns the entries in insertion order. The returned slice is a
// copy — callers may not modify it. Safe to call on a nil receiver.
func (s *IPSet) All() []SourcedIP {
	if s == nil {
		return nil
	}
	out := make([]SourcedIP, len(s.entries))
	copy(out, s.entries)
	return out
}

// RawIPs extracts just the net.IP values in insertion order.
// Safe to call on a nil receiver.
func (s *IPSet) RawIPs() []net.IP {
	if s == nil {
		return nil
	}
	out := make([]net.IP, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e.IP)
	}
	return out
}

// Len returns the number of unique IPs in the set.
// Safe to call on a nil receiver.
func (s *IPSet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.entries)
}

// AdvertiseInput is the raw input for computing the set of API addresses
// the server should advertise to clients and to miren.cloud.
type AdvertiseInput struct {
	// ListenAddr is the server's own listen address (e.g. "0.0.0.0:8443").
	// Included in the advertised list only if it has a literal,
	// non-loopback, non-unspecified IP.
	ListenAddr string

	// IPs is the unified list of all candidate IP addresses, each
	// tagged as explicit (user-configured) or discovered (interface scan).
	// Explicit IPs bypass all filtering except loopback / unspecified.
	// Discovered IPs are subject to CGNAT, netcheck, and other pruning.
	IPs []SourcedIP

	// Netcheck is the result of the dual-stack netcheck, if one has run.
	// A nil pointer means netcheck never ran / failed entirely.
	Netcheck *cloudauth.NetcheckDualStackResult

	// Port is the port to append to bare IPs (defaults to 8443).
	Port int
}

// AdvertiseCandidate describes one candidate address the advertise logic
// considered, and whether it ended up in the final advertised set. Used by
// both production (building the final list) and debug tooling (explaining
// the decision for every IP).
type AdvertiseCandidate struct {
	Source         string // "listen", "explicit", "discovered", "netcheck"
	HostPort       string
	IP             net.IP
	Classification string // loopback / link-local / private / global-unicast / other
	Included       bool
	Reason         string
}

// ComputeAdvertise is the single source of truth for computing the addresses
// the server advertises. It returns the ordered list of candidates (including
// rejected ones, so callers can explain why) and the final list of advertised
// host:port strings.
//
// The returned list is intended for StatusReport.APIAddresses, i.e. the
// addresses miren.cloud hands out to clients that want to reach this
// cluster. Loopback and unspecified (0.0.0.0, ::) addresses are never
// included — a client coming in through miren.cloud is by definition not
// running on the same host, so those entries would only produce failed
// connection attempts.
//
// Filtering rules:
//
//  1. Listen address: included if it parses as host:port with a literal,
//     non-loopback, non-unspecified IP.
//
//  2. Explicit IPs (user-configured): always included, except loopback
//     and unspecified which are dropped with a reason.
//
//  3. Discovered IPs (auto-scanned from interfaces):
//     a. Loopback and unspecified are dropped.
//     b. CGNAT addresses (100.64.0.0/10) are dropped.
//     c. Public (global-unicast, non-private) IPs are dropped if netcheck
//     ran for that address family and proved the family unreachable
//     or found reachable addresses (replaced by netcheck-confirmed ones).
//     d. Otherwise kept as a fallback.
//
//  4. Netcheck public addresses: included when reachable on at least one port.
func ComputeAdvertise(in AdvertiseInput) ([]AdvertiseCandidate, []string) {
	port := in.Port
	if port == 0 {
		port = 8443
	}
	portStr := strconv.Itoa(port)

	var cands []AdvertiseCandidate
	var final []string
	seen := make(map[string]struct{})

	add := func(c AdvertiseCandidate) {
		cands = append(cands, c)
		if !c.Included {
			return
		}
		if _, ok := seen[c.HostPort]; ok {
			return
		}
		seen[c.HostPort] = struct{}{}
		final = append(final, c.HostPort)
	}

	// 1. Listen address.
	if in.ListenAddr != "" {
		host, _, err := net.SplitHostPort(in.ListenAddr)
		ip := net.ParseIP(host)
		switch {
		case err != nil || ip == nil:
			add(AdvertiseCandidate{
				Source:   "listen",
				HostPort: in.ListenAddr,
				Included: false,
				Reason:   "not a literal IP host",
			})
		case ip.IsUnspecified():
			add(AdvertiseCandidate{
				Source:         "listen",
				HostPort:       in.ListenAddr,
				IP:             ip,
				Classification: "unspecified",
				Included:       false,
				Reason:         "unspecified address (0.0.0.0 / ::) is not routable",
			})
		case ip.IsLoopback():
			add(AdvertiseCandidate{
				Source:         "listen",
				HostPort:       in.ListenAddr,
				IP:             ip,
				Classification: "loopback",
				Included:       false,
				Reason:         "loopback is not reachable from remote clients",
			})
		default:
			add(AdvertiseCandidate{
				Source:         "listen",
				HostPort:       in.ListenAddr,
				IP:             ip,
				Classification: classify(ip),
				Included:       true,
				Reason:         "server listen address",
			})
		}
	}

	// Compute per-family netcheck state.
	v4State := netcheckFamilyState(familyIPv4, in.Netcheck)
	v6State := netcheckFamilyState(familyIPv6, in.Netcheck)

	// 2 & 3. IPs — explicit pass through, discovered are filtered.
	for _, sip := range in.IPs {
		ip := sip.IP
		if ip == nil {
			continue
		}
		hp := net.JoinHostPort(ip.String(), portStr)

		source := "discovered"
		if sip.Explicit {
			source = "explicit"
		}

		cand := AdvertiseCandidate{
			Source:         source,
			HostPort:       hp,
			IP:             ip,
			Classification: classify(ip),
		}

		// Loopback / unspecified always rejected regardless of source.
		if ip.IsUnspecified() {
			cand.Included = false
			cand.Reason = "unspecified address is not routable"
			add(cand)
			continue
		}
		if ip.IsLoopback() {
			cand.Included = false
			cand.Reason = "loopback is not reachable from remote clients"
			add(cand)
			continue
		}

		// Explicit IPs pass through with no further filtering.
		if sip.Explicit {
			cand.Included = true
			cand.Reason = "user-configured"
			add(cand)
			continue
		}

		// --- Discovered IP filtering below ---

		if isCGNAT(ip) {
			cand.Included = false
			cand.Reason = "CGNAT 100.64.0.0/10 (e.g. tailscale)"
			add(cand)
			continue
		}

		isPublicCandidate := !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast()
		if !isPublicCandidate {
			cand.Included = true
			cand.Reason = "private/link-local, kept for LAN clients"
			add(cand)
			continue
		}

		state := v4State
		if ip.To4() == nil {
			state = v6State
		}
		switch state {
		case netcheckReachable:
			cand.Included = false
			cand.Reason = "replaced by netcheck-confirmed public address"
		case netcheckUnreachable:
			cand.Included = false
			cand.Reason = "address family proven unreachable by netcheck"
		default:
			cand.Included = true
			cand.Reason = "no netcheck override"
		}
		add(cand)
	}

	// 4. Netcheck public addresses.
	for _, hp := range publicAddressesFromNetcheck(in.Netcheck) {
		host, _, _ := net.SplitHostPort(hp)
		ip := net.ParseIP(host)
		add(AdvertiseCandidate{
			Source:         "netcheck",
			HostPort:       hp,
			IP:             ip,
			Classification: classify(ip),
			Included:       true,
			Reason:         "netcheck confirmed reachable",
		})
	}

	return cands, final
}

type netcheckFamily int

const (
	familyIPv4 netcheckFamily = iota
	familyIPv6
)

type netcheckStatus int

const (
	netcheckNotRun netcheckStatus = iota
	netcheckUnreachable
	netcheckReachable
)

// netcheckFamilyState returns what we know about reachability for one address
// family. A nil NetcheckDualStackResult or a nil family response means "not
// run". A response with a non-public/invalid source address is also treated
// as not run (same rule runNetcheck applies). A response with a valid source
// but zero reachable ports is "proven unreachable".
func netcheckFamilyState(fam netcheckFamily, result *cloudauth.NetcheckDualStackResult) netcheckStatus {
	if result == nil {
		return netcheckNotRun
	}
	var resp *cloudauth.NetcheckResponse
	switch fam {
	case familyIPv4:
		resp = result.IPv4
	case familyIPv6:
		resp = result.IPv6
	}
	if resp == nil {
		return netcheckNotRun
	}
	src := net.ParseIP(resp.SourceAddress)
	if src == nil || !src.IsGlobalUnicast() || src.IsPrivate() {
		return netcheckNotRun
	}
	for _, r := range resp.Results {
		if r.Reachable {
			return netcheckReachable
		}
	}
	return netcheckUnreachable
}

// publicAddressesFromNetcheck returns netcheck-confirmed reachable host:port
// strings.
func publicAddressesFromNetcheck(result *cloudauth.NetcheckDualStackResult) []string {
	if result == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var addrs []string
	for _, resp := range []*cloudauth.NetcheckResponse{result.IPv4, result.IPv6} {
		if resp == nil || resp.SourceAddress == "" {
			continue
		}
		src := net.ParseIP(resp.SourceAddress)
		if src == nil || !src.IsGlobalUnicast() || src.IsPrivate() {
			continue
		}
		for _, r := range resp.Results {
			if !r.Reachable {
				continue
			}
			hp := net.JoinHostPort(resp.SourceAddress, strconv.Itoa(r.Port))
			if _, ok := seen[hp]; ok {
				continue
			}
			seen[hp] = struct{}{}
			addrs = append(addrs, hp)
		}
	}
	return addrs
}

// isCGNAT reports whether ip falls in the 100.64.0.0/10 Carrier-Grade NAT
// range (RFC 6598). Tailscale tailnet addresses also live in this range,
// so filtering CGNAT out of discovered-IP lists keeps them from being
// advertised to clients who aren't on the tailnet.
func isCGNAT(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 100 && ip4[1]&0xc0 == 0x40
}

// classify returns a short string describing the kind of address, for
// diagnostic output.
func classify(ip net.IP) string {
	if ip == nil {
		return "unknown"
	}
	switch {
	case ip.IsLoopback():
		return "loopback"
	case ip.IsLinkLocalUnicast():
		return "link-local"
	case ip.IsPrivate():
		return "private"
	case ip.IsGlobalUnicast():
		return "global-unicast"
	default:
		return "other"
	}
}
