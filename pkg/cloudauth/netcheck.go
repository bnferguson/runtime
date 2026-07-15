package cloudauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// NetcheckPort describes a port and protocol to check for reachability.
type NetcheckPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "http", "https", "http3"
}

// NetcheckRequest is the request body for the netcheck endpoint.
type NetcheckRequest struct {
	Ports []NetcheckPort `json:"ports"`
}

// NetcheckResult is the result of a single port check.
type NetcheckResult struct {
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Reachable bool   `json:"reachable"`
	LatencyMs int    `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// NetcheckResponse is the response from the netcheck endpoint.
type NetcheckResponse struct {
	SourceAddress string           `json:"source_address"`
	Results       []NetcheckResult `json:"results"`
	DurationMs    int              `json:"duration_ms"`
}

// NetcheckDualStackResult holds netcheck responses for both address families.
type NetcheckDualStackResult struct {
	IPv4 *NetcheckResponse
	IPv6 *NetcheckResponse
}

// transportForProtocol maps a netcheck protocol to the transport the dashboard
// should name. QUIC (http3) rides on UDP; plain http/https are TCP.
func transportForProtocol(protocol string) string {
	if protocol == "http3" {
		return "UDP"
	}
	return "TCP"
}

// ReachabilityVerdict synthesizes a compact reachability verdict from a
// netcheck result, for reporting to cloud. It returns nil when netcheck
// produced no usable public source address for either family (nil result,
// missing responses, or a private/invalid source) — in that case there is
// nothing proven to report and cloud should fall back to its generic copy.
//
// When a public source address is present, the verdict names it. If any port
// on that source was reachable, Reachable is true and no ports are listed;
// otherwise Reachable is false and every failed port is listed with its
// transport so the dashboard can say "UDP 8443 (QUIC) unreachable".
func (r *NetcheckDualStackResult) ReachabilityVerdict() *ReachabilityVerdict {
	if r == nil {
		return nil
	}

	// A reachable port on either family means the cluster is reachable, so we
	// scan both families before concluding otherwise. If none is reachable, the
	// first family with a usable public source becomes the not-reachable verdict.
	var notReachable *ReachabilityVerdict
	for _, resp := range []*NetcheckResponse{r.IPv4, r.IPv6} {
		if resp == nil {
			continue
		}
		src := net.ParseIP(resp.SourceAddress)
		if src == nil || !src.IsGlobalUnicast() || src.IsPrivate() {
			continue
		}

		var unreachable []UnreachablePort
		reachable := false
		for _, res := range resp.Results {
			if res.Reachable {
				reachable = true
				continue
			}
			unreachable = append(unreachable, UnreachablePort{
				Port:      res.Port,
				Protocol:  res.Protocol,
				Transport: transportForProtocol(res.Protocol),
			})
		}

		if reachable {
			return &ReachabilityVerdict{
				Reachable:     true,
				PublicAddress: resp.SourceAddress,
			}
		}
		if notReachable == nil {
			notReachable = &ReachabilityVerdict{
				Reachable:        false,
				PublicAddress:    resp.SourceAddress,
				UnreachablePorts: unreachable,
			}
		}
	}

	return notReachable
}

// ErrPrivateAddress is returned when the cloud rejects the request
// because the cluster's IP is private/loopback/link-local.
var ErrPrivateAddress = errors.New("client IP is not a public address")

// Netcheck calls the cloud's netcheck endpoint to determine whether the
// cluster is publicly reachable on the given ports. The endpoint requires no
// authentication — it uses the request's source IP for probing.
//
// The network parameter controls the address family: "tcp4" forces IPv4,
// "tcp6" forces IPv6, and "" uses the OS default.
func Netcheck(ctx context.Context, cloudURL string, ports []NetcheckPort, network string) (*NetcheckResponse, error) {
	body, err := json.Marshal(NetcheckRequest{Ports: ports})
	if err != nil {
		return nil, fmt.Errorf("marshal netcheck request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/netcheck", cloudURL)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create netcheck request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if network != "" {
		transport.DialContext = (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext
		// Wrap the dialer to force the address family
		baseDialContext := transport.DialContext
		transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
			return baseDialContext(ctx, network, addr)
		}
	}

	client := &http.Client{
		Transport: transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send netcheck request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read netcheck response: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrPrivateAddress
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netcheck failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result NetcheckResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode netcheck response: %w", err)
	}

	return &result, nil
}

// NetcheckDualStack calls the netcheck endpoint concurrently over IPv4 and
// IPv6 to discover public addresses on both address families. Each call gets
// its own timeout so a missing address family fails fast without blocking the
// other. Either result may be nil if the address family is unavailable or the
// check fails. Returns an error only if both checks fail.
func NetcheckDualStack(ctx context.Context, cloudURL string, ports []NetcheckPort) (*NetcheckDualStackResult, error) {
	type netcheckResult struct {
		resp *NetcheckResponse
		err  error
	}

	ipv4Ch := make(chan netcheckResult, 1)
	ipv6Ch := make(chan netcheckResult, 1)

	go func() {
		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		resp, err := Netcheck(callCtx, cloudURL, ports, "tcp4")
		ipv4Ch <- netcheckResult{resp, err}
	}()
	go func() {
		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		resp, err := Netcheck(callCtx, cloudURL, ports, "tcp6")
		ipv6Ch <- netcheckResult{resp, err}
	}()

	v4 := <-ipv4Ch
	v6 := <-ipv6Ch

	if v4.err != nil && v6.err != nil {
		return nil, fmt.Errorf("netcheck failed on both address families: ipv4: %w, ipv6: %v", v4.err, v6.err)
	}

	return &NetcheckDualStackResult{
		IPv4: v4.resp,
		IPv6: v6.resp,
	}, nil
}
