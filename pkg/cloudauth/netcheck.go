package cloudauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// ErrPrivateAddress is returned when the cloud rejects the request
// because the cluster's IP is private/loopback/link-local.
var ErrPrivateAddress = errors.New("client IP is not a public address")

// Netcheck calls the cloud's netcheck endpoint to determine whether the
// cluster is publicly reachable on the given ports. The endpoint requires no
// authentication — it uses the request's source IP for probing.
func Netcheck(ctx context.Context, cloudURL string, ports []NetcheckPort) (*NetcheckResponse, error) {
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

	client := &http.Client{Timeout: 30 * time.Second}
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
