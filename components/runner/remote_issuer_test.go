package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoteIssuerCachesURL(t *testing.T) {
	ri := newRemoteIssuer(context.Background(), nil, "https://issuer.example")
	require.Equal(t, "https://issuer.example", ri.IssuerURL())
}
