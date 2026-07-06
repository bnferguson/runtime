package build

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	buildkitclient "github.com/moby/buildkit/client"
	"github.com/stretchr/testify/require"

	"miren.dev/runtime/components/buildkit"
)

// The real component must satisfy the provider interface the builder depends on.
var _ BuildKitProvider = (*buildkit.Component)(nil)

// fakeBuildKitProvider is a BuildKitProvider whose Client always fails. It lets
// the build path be exercised without standing up a real containerd-managed
// buildkitd daemon.
type fakeBuildKitProvider struct {
	clientErr error
}

func (f fakeBuildKitProvider) Client(context.Context) (*buildkitclient.Client, error) {
	return nil, f.clientErr
}

func (f fakeBuildKitProvider) SocketPath() string { return "/nonexistent/buildkitd.sock" }

func (f fakeBuildKitProvider) IsRunning() bool { return true }

// When the daemon can't be reached, runBuildkitBuild must surface the error
// rather than proceed into the solve. This is the fast-fail behavior that keeps
// a stale/dead buildkitd from silently breaking builds.
func TestRunBuildkitBuildClientError(t *testing.T) {
	r := require.New(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	b := &Builder{
		Log:      log,
		BuildKit: fakeBuildKitProvider{clientErr: errors.New("dial unix: connection refused")},
	}

	_, _, _, err := b.runBuildkitBuild(context.Background(), runBuildkitBuildInputs{
		SourceDir:   t.TempDir(),
		AppName:     "testapp",
		VersionName: "v1",
		ImageURL:    "cluster.local:5000/testapp:v1",
	}, noopStatusSender{}, &buildLogWriter{log: log})

	r.Error(err)
	r.Contains(err.Error(), "connection refused")
}
