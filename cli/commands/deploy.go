package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"

	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/api/build/build_v1alpha"
	"miren.dev/runtime/api/deployment/deployment_v1alpha"
	"miren.dev/runtime/appconfig"
	"miren.dev/runtime/clientconfig"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/deploygating"
	ephemeralx "miren.dev/runtime/pkg/ephemeral"
	"miren.dev/runtime/pkg/git"
	"miren.dev/runtime/pkg/otelproxy"
	"miren.dev/runtime/pkg/progress/upload"
	"miren.dev/runtime/pkg/rpc/standard"
	"miren.dev/runtime/pkg/rpc/stream"
	"miren.dev/runtime/pkg/tarx"
	"miren.dev/runtime/pkg/ui"
)

var deployTracer = otel.Tracer("miren.dev/runtime/cli/deploy")

func Deploy(ctx *Context, opts struct {
	AppCentric

	Version       string   `short:"V" long:"version" description:"Deploy an existing version (skip build)"`
	Analyze       bool     `long:"analyze" description:"Analyze the app without building (show detected stack, services, etc.)"`
	Explain       bool     `short:"x" long:"explain" description:"Explain the build process"`
	ExplainFormat string   `long:"explain-format" description:"Explain format" choice:"auto" choice:"plain" choice:"tty" choice:"rawjson" default:"auto"` //nolint
	Force         bool     `short:"f" long:"force" description:"Skip confirmation prompt"`
	Env           []string `short:"e" long:"env" description:"Set environment variable (KEY=VALUE, KEY=@file, or KEY to prompt)"`
	Sensitive     []string `short:"s" long:"sensitive" description:"Set sensitive environment variable (masked in output)"`
	Ephemeral     string   `long:"ephemeral" description:"Deploy as ephemeral preview with this label (e.g. feat-login)"`
	TTL           string   `long:"ttl" description:"TTL for ephemeral version (e.g. 48h)" default:"24h"`
}) error {
	name := opts.App
	dir := opts.ResolvedDir()

	// Normalize and validate ephemeral label
	var ephemeralLabel, ephemeralTTL string
	if opts.Ephemeral != "" {
		normalized, err := ephemeralx.NormalizeLabel(opts.Ephemeral)
		if err != nil {
			return fmt.Errorf("invalid ephemeral label: %w", err)
		}
		ephemeralLabel = normalized

		if _, err := time.ParseDuration(opts.TTL); err != nil {
			return fmt.Errorf("invalid TTL %q: %w", opts.TTL, err)
		}
		ephemeralTTL = opts.TTL
	}

	if ctx.ClientConfig == nil {
		return fmt.Errorf("no client configuration available; run `miren login` to authenticate or install a server locally")
	}

	isInteractive := term.IsTerminal(int(os.Stdin.Fd()))

	// Check that we have at least one cluster configured
	// Check if we have an identity - if so, offer to add a cluster
	if isInteractive &&
		ctx.ClientConfig.GetClusterCount() == 0 &&
		ctx.ClientConfig.HasIdentities() {
		confirmed, err := ui.Confirm(
			ui.WithMessage("No clusters configured. Would you like to add one now?"),
			ui.WithDefault(true),
			ui.WithIndent("  "),
		)
		if err != nil || !confirmed {
			return fmt.Errorf("no clusters configured; run 'miren login' to authenticate and 'miren cluster add' to configure a cluster, or install a server locally")
		}

		if err := AddClusterInteractive(ctx); err != nil {
			return fmt.Errorf("failed to add cluster: %w", err)
		}

		// Reload the client config
		cfg, err := clientconfig.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to reload config after adding cluster: %w", err)
		}
		ctx.ClientConfig = cfg

		// Get the active cluster (the one we just added)
		clusterName := cfg.ActiveCluster()
		if clusterName == "" {
			return fmt.Errorf("no active cluster set after adding cluster")
		}

		clusterCfg, err := cfg.GetCluster(clusterName)
		if err == nil && clusterCfg != nil {
			ctx.ClusterConfig = clusterCfg
			ctx.ClusterName = clusterName
		}

		ctx.Info("")
	}

	// Re-check after potential cluster add
	if ctx.ClientConfig.GetClusterCount() == 0 {
		return fmt.Errorf("no clusters configured; run 'miren login' to authenticate and configure a cluster, or install a server locally")
	}

	// Handle --analyze flag: analyze the app without building
	if opts.Analyze {
		cl, err := ctx.RPCClient("dev.miren.runtime/build")
		if err != nil {
			return err
		}

		bc := build_v1alpha.NewBuilderClient(cl)
		return analyzeApp(ctx, bc, dir)
	}

	// Handle --version flag: deploy an existing version (skip build)
	if opts.Version != "" {
		depCl, err := ctx.RPCClient("dev.miren.runtime/deployment")
		if err != nil {
			return fmt.Errorf("failed to connect to deployment service: %w", err)
		}
		depClient := deployment_v1alpha.NewDeploymentClient(depCl)

		var envVars []*deployment_v1alpha.EnvironmentVariable
		if len(opts.Env) > 0 || len(opts.Sensitive) > 0 {
			specs, err := ParseEnvVarSpecs(opts.Env, opts.Sensitive)
			if err != nil {
				return err
			}
			for _, spec := range specs {
				ev := &deployment_v1alpha.EnvironmentVariable{}
				ev.SetKey(spec.Key)
				ev.SetValue(spec.Value)
				ev.SetSensitive(spec.Sensitive)
				envVars = append(envVars, ev)
			}
		}

		result, err := depClient.DeployVersion(ctx, name, ctx.ClusterName, opts.Version, false, envVars, ephemeralLabel, ephemeralTTL)
		if err != nil {
			return fmt.Errorf("failed to deploy version: %w", err)
		}

		if result.HasError() && result.Error() != "" {
			if result.HasLockInfo() && result.LockInfo() != nil {
				lockInfo := result.LockInfo()
				ctx.Printf("\n❌ Deployment blocked:\n\n")
				ctx.Printf("Another deployment is already in progress for app '%s' on cluster '%s'.\n\n",
					lockInfo.AppName(), lockInfo.ClusterId())
				ctx.Printf("Existing deployment details:\n")
				ctx.Printf("  • Deployment ID: %s\n", ui.DisplayShortID(lockInfo.BlockingDeploymentShortId(), lockInfo.BlockingDeploymentId()))
				ctx.Printf("  • Started by: %s\n", lockInfo.StartedBy())
				if lockInfo.HasStartedAt() && lockInfo.StartedAt() != nil {
					startedAt := time.Unix(lockInfo.StartedAt().Seconds(), 0)
					ctx.Printf("  • Started at: %s (%s ago)\n",
						startedAt.Format("2006-01-02 15:04:05 MST"),
						time.Since(startedAt).Round(time.Second))
				}
				ctx.Printf("  • Current phase: %s\n", lockInfo.CurrentPhase())
				if lockInfo.HasLockExpiresAt() && lockInfo.LockExpiresAt() != nil {
					expiresAt := time.Unix(lockInfo.LockExpiresAt().Seconds(), 0)
					ctx.Printf("  • Lock expires in: %s\n\n", time.Until(expiresAt).Round(time.Second))
				}
			} else {
				ctx.Printf("\n❌ Deploy failed: %s\n", result.Error())
			}
			return fmt.Errorf("deploy version failed")
		}

		if result.HasDeployment() && result.Deployment() != nil {
			dep := result.Deployment()
			deployedVersion := dep.AppVersionId()
			if deployedVersion == "" {
				deployedVersion = opts.Version
			}
			versionDisplay := ui.DisplayShortID(dep.AppVersionShortId(), deployedVersion)

			if ephemeralLabel != "" {
				// Ephemeral deploy via --version: show ephemeral info, skip activation wait
				ctx.Printf("Ephemeral version %s created.\n", versionDisplay)
				ctx.Printf("  Label: %s\n", ephemeralLabel)
				ctx.Printf("  TTL:   %s\n", ephemeralTTL)
				if result.HasAccessInfo() && result.AccessInfo() != nil {
					info := result.AccessInfo()
					if info.HasHostnames() {
						for _, h := range *info.Hostnames() {
							ctx.Printf("  URL:   https://%s\n", h)
						}
					}
					if info.HasClusterHostname() && info.ClusterHostname() != "" {
						ctx.Printf("  URL:   https://%s.%s\n", ephemeralLabel, info.ClusterHostname())
					}
				}
			} else {
				ctx.Printf("✓ Deployed version %s to %s\n", versionDisplay, ctx.ClusterName)

				appCl, appErr := ctx.RPCClient(rpcAppStatus)
				if appErr == nil {
					appStatusClient := app_v1alpha.NewAppStatusClient(appCl)
					waitForActivation(ctx, appStatusClient, name, deployedVersion, versionDisplay)
				}

				if result.HasAccessInfo() && result.AccessInfo() != nil {
					displayDeployVersionAccessInfo(ctx, name, result.AccessInfo())
				}
			}
		}
		return nil
	}

	// Set up OTel tracing via the server's telemetry proxy.
	// Spans are shipped through the existing RPC connection — no client-side
	// OTLP config needed. If the server isn't reachable, tracing is a no-op.
	if proxyClient, err := ctx.RPCClient("dev.miren.runtime/telemetry"); err == nil {
		shutdown, setupErr := otelproxy.SetupProxyTracing(ctx.Context, proxyClient, ctx.Log, semconv.ServiceName("miren-cli"))
		if setupErr != nil {
			ctx.Log.Debug("failed to set up proxy tracing", "error", setupErr)
		} else {
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = shutdown(shutdownCtx)
			}()
		}
	} else {
		ctx.Log.Debug("telemetry proxy unavailable", "error", err)
	}

	// Start root deploy span
	deployCtxTraced, deploySpan := deployTracer.Start(ctx.Context, "deploy",
		trace.WithAttributes(
			attribute.String("miren.app.name", name),
			attribute.String("miren.cluster", ctx.ClusterName),
		),
	)
	defer deploySpan.End()

	// Replace the context so downstream calls inherit the trace
	ctx.Context = deployCtxTraced

	// Confirm deployment unless --force is used or stdin is not a TTY.
	// Always confirm when the app config was found in a parent directory,
	// otherwise skip confirmation when only one cluster is configured.
	hasSingleCluster := ctx.ClientConfig != nil && ctx.ClientConfig.GetClusterCount() == 1
	needsConfirm := opts.foundInParent || !hasSingleCluster
	if !opts.Force && isInteractive && needsConfirm {
		if opts.foundInParent {
			ctx.Printf("  ℹ App config found in parent directory %s\n", dir)
		}
		message := fmt.Sprintf("Deploy app '%s' to cluster '%s'?", name, ctx.ClusterName)
		confirmed, err := ui.Confirm(
			ui.WithMessage(message),
			ui.WithDefault(true),
			ui.WithIndent("  "),
		)
		if err != nil {
			return fmt.Errorf("confirmation cancelled: %w", err)
		}
		if !confirmed {
			ctx.Printf("  deployment cancelled\n")
			return nil
		}
	}

	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	faintStyle := lipgloss.NewStyle().Faint(true)
	ctx.Printf("  ✓ %s: %s %s %s\n", greenStyle.Render("Deploying"), name, faintStyle.Render("→"), ctx.ClusterName)

	cl, err := ctx.RPCClient("dev.miren.runtime/build")
	if err != nil {
		return err
	}

	bc := build_v1alpha.NewBuilderClient(cl)

	// Check if deployment is allowed before proceeding
	remedy, err := deploygating.CheckDeployAllowed(dir)
	if err != nil {
		if remedy != "" {
			ctx.Printf("Error: %s\n%s\n\n", err, remedy)
		}
		return fmt.Errorf("deploy gate check failed: %w", err)
	}

	ctx.Log.Info("building code", "name", name, "dir", dir)

	// Capture git information before creating deployment record
	var gitInfo *git.Info
	gitInfo, gitErr := git.GetInfo(dir)
	if gitErr != nil {
		ctx.Log.Debug("Failed to get git info", "error", gitErr)
		// Don't fail deployment if git info is unavailable
	}

	// Create deployment record early in the process
	depCl, err := ctx.RPCClient("dev.miren.runtime/deployment")
	if err != nil {
		return fmt.Errorf("failed to connect to deployment service: %w", err)
	}
	depClient := deployment_v1alpha.NewDeploymentClient(depCl)

	// Convert git.Info to deployment GitInfo
	var deploymentGitInfo *deployment_v1alpha.GitInfo
	if gitInfo != nil {
		deploymentGitInfo = &deployment_v1alpha.GitInfo{}
		deploymentGitInfo.SetSha(gitInfo.SHA)
		deploymentGitInfo.SetBranch(gitInfo.Branch)
		deploymentGitInfo.SetIsDirty(gitInfo.IsDirty)
		deploymentGitInfo.SetWorkingTreeHash(gitInfo.WorkingTreeHash)
		deploymentGitInfo.SetCommitMessage(gitInfo.CommitMessage)
		deploymentGitInfo.SetCommitAuthorName(gitInfo.CommitAuthor)
		deploymentGitInfo.SetCommitAuthorEmail(gitInfo.CommitEmail)
		deploymentGitInfo.SetRepository(gitInfo.RemoteURL)

		// Convert timestamp string to standard.Timestamp if available
		if gitInfo.CommitTimestamp != "" {
			if ts, err := time.Parse(time.RFC3339, gitInfo.CommitTimestamp); err == nil {
				deploymentGitInfo.SetCommitTimestamp(standard.ToTimestamp(ts))
			}
		}
	}

	// Create deployment record for non-ephemeral deploys only.
	// Ephemeral deploys don't participate in deployment tracking or locking.
	var deploymentId string
	if ephemeralLabel == "" {
		createDepCtx, createDepSpan := deployTracer.Start(ctx.Context, "deploy.create_deployment")
		createResult, err := depClient.CreateDeployment(createDepCtx, name, ctx.ClusterName, "pending-build", deploymentGitInfo)
		if err != nil {
			createDepSpan.RecordError(err)
			createDepSpan.SetStatus(codes.Error, err.Error())
			createDepSpan.End()
			return fmt.Errorf("failed to create deployment record: %w", err)
		}
		createDepSpan.End()

		if createResult.HasError() && createResult.Error() != "" {
			// Check if we have structured lock info
			if createResult.HasLockInfo() && createResult.LockInfo() != nil {
				lockInfo := createResult.LockInfo()

				// Format the deployment lock message
				ctx.Printf("\n❌ Deployment blocked:\n\n")
				ctx.Printf("Another deployment is already in progress for app '%s' on cluster '%s'.\n\n",
					lockInfo.AppName(), lockInfo.ClusterId())

				ctx.Printf("Existing deployment details:\n")
				ctx.Printf("  • Deployment ID: %s\n", ui.DisplayShortID(lockInfo.BlockingDeploymentShortId(), lockInfo.BlockingDeploymentId()))
				ctx.Printf("  • Started by: %s\n", lockInfo.StartedBy())

				if lockInfo.HasStartedAt() && lockInfo.StartedAt() != nil {
					startedAt := time.Unix(lockInfo.StartedAt().Seconds(), 0)
					ctx.Printf("  • Started at: %s (%s ago)\n",
						startedAt.Format("2006-01-02 15:04:05 MST"),
						time.Since(startedAt).Round(time.Second))
				}

				ctx.Printf("  • Current phase: %s\n", lockInfo.CurrentPhase())

				if lockInfo.HasLockExpiresAt() && lockInfo.LockExpiresAt() != nil {
					expiresAt := time.Unix(lockInfo.LockExpiresAt().Seconds(), 0)
					timeRemaining := time.Until(expiresAt).Round(time.Second)
					ctx.Printf("  • Lock expires in: %s\n\n", timeRemaining)
				}

				// Build contact message
				if lockInfo.StartedBy() != "-" {
					ctx.Printf("Please wait for it to complete or contact %s to coordinate.\n", lockInfo.StartedBy())
				} else {
					ctx.Printf("Please wait for it to complete.\n")
				}
			} else {
				// Fall back to plain error message
				ctx.Printf("\n❌ Deployment blocked:\n\n%s\n", createResult.Error())
			}
			return fmt.Errorf("deployment blocked by lock")
		}

		if !createResult.HasDeployment() || createResult.Deployment() == nil {
			return fmt.Errorf("deployment creation returned no deployment")
		}

		deploymentId = createResult.Deployment().Id()
		ctx.Log.Info("Created deployment record", "deployment_id", deploymentId)
	}

	// Create a cancellable context for the build that can be cancelled externally
	buildCtx, cancelBuild := context.WithCancel(ctx.Context)
	defer cancelBuild()

	// Start goroutine to poll for external cancellation using the cancellationPoller
	statusGetter := newDepClientStatusGetter(depClient, ctx.Log)
	poller := newCancellationPoller(deploymentId, statusGetter, 2*time.Second)
	go func() {
		poller.Start(buildCtx, func() {
			ctx.Log.Info("Deployment cancelled externally, stopping build")
			cancelBuild()
		})
	}()

	// Helper to check if we were externally cancelled
	wasExternallyCancelled := poller.WasExternallyCancelled

	// Parse environment variables to pass to build server
	var envVars []*build_v1alpha.EnvironmentVariable
	if len(opts.Env) > 0 || len(opts.Sensitive) > 0 {
		envSpecs, err := ParseEnvVarSpecs(opts.Env, opts.Sensitive)
		if err != nil {
			return err
		}

		// Convert to build_v1alpha.EnvironmentVariable for RPC
		envVars = make([]*build_v1alpha.EnvironmentVariable, len(envSpecs))
		for i, spec := range envSpecs {
			ev := &build_v1alpha.EnvironmentVariable{}
			ev.SetKey(spec.Key)
			ev.SetValue(spec.Value)
			ev.SetSensitive(spec.Sensitive)
			envVars[i] = ev
		}

		ctx.Printf("  Setting %d environment variable(s)...\n", len(envVars))
	}

	// Initialize build error/log/warning tracking. buildStateMu guards the
	// three slices below; the build status callback appends to them from
	// RPC stream-handler goroutines, and updateDeploymentOnError reads them.
	var buildStateMu sync.Mutex
	var buildErrors []string
	var buildLogs []string
	var deployWarnings []*build_v1alpha.LogEntry

	// Helper function to update deployment phase
	updateDeploymentPhase := func(phase string) {
		if deploymentId != "" {
			_, updateErr := depClient.UpdateDeploymentPhase(ctx, deploymentId, phase)
			if updateErr != nil {
				ctx.Log.Error("Failed to update deployment phase", "error", updateErr, "phase", phase)
			}
		}
	}

	// snapshotBuildState returns copies of the build state slices under the
	// mutex so readers don't race with append in createBuildStatusCallback's
	// stream-handler goroutines.
	snapshotBuildState := func() ([]string, []string, []*build_v1alpha.LogEntry) {
		buildStateMu.Lock()
		defer buildStateMu.Unlock()
		return slices.Clone(buildErrors), slices.Clone(buildLogs), slices.Clone(deployWarnings)
	}

	// Helper function to update deployment status on failure
	updateDeploymentOnError := func(errMsg string) {
		if deploymentId != "" {
			// Use a separate context with timeout for status updates to ensure they complete
			// even if the main context is canceled
			statusCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			errsSnap, logsSnap, _ := snapshotBuildState()
			logs := strings.Join(logsSnap, "\n")
			if logs == "" && len(errsSnap) > 0 {
				logs = strings.Join(errsSnap, "\n")
			}

			_, updateErr := depClient.UpdateFailedDeployment(statusCtx, deploymentId, errMsg, logs)
			if updateErr != nil {
				// Fallback to simple status update
				_, updateErr = depClient.UpdateDeploymentStatus(statusCtx, deploymentId, "failed", errMsg)
				if updateErr != nil {
					ctx.Log.Error("Failed to update deployment status to failed", "error", updateErr)
				}
			}
		}
	}

	// If the deploy panics during the build phase (e.g. a stream-callback
	// race), mark the deployment failed so the server-side lock is released
	// immediately instead of waiting for its 30-minute TTL. Once the
	// deployment has been marked active, we no longer want to flip it back to
	// failed on panic — the build succeeded and traffic is already moving.
	var deploymentFinalized bool
	defer func() {
		if r := recover(); r != nil {
			if !deploymentFinalized {
				updateDeploymentOnError(fmt.Sprintf("CLI panic: %v", r))
			}
			panic(r)
		}
	}()

	// Load AppConfig to get include patterns
	var includePatterns []string
	ac, err := appconfig.LoadAppConfigUnder(dir)
	if err != nil {
		updateDeploymentOnError(fmt.Sprintf("Failed to load app config: %v", err))
		return err
	}
	if ac != nil && ac.Include != nil {
		// Validate patterns before using them
		for _, pattern := range ac.Include {
			if err := tarx.ValidatePattern(pattern); err != nil {
				updateDeploymentOnError(fmt.Sprintf("Invalid include pattern: %v", err))
				return fmt.Errorf("invalid include pattern %q: %w", pattern, err)
			}
		}
		includePatterns = ac.Include
	}

	// Update phase to building
	updateDeploymentPhase("building")

	// Start upload span covering tar creation + build
	_, uploadSpan := deployTracer.Start(buildCtx, "deploy.upload")

	// Try optimized delta upload: compute manifest, ask server what's cached
	var (
		sessionID    string
		useOptimized bool
		totalFiles   int
		cachedFiles  int32
		neededPaths  map[string]bool
	)

	manifest, manifestErr := tarx.ComputeManifest(dir, includePatterns)
	if manifestErr == nil {
		totalFiles = len(manifest)

		// Convert to RPC manifest entries
		rpcManifest := make([]*build_v1alpha.FileManifestEntry, len(manifest))
		for i, m := range manifest {
			entry := &build_v1alpha.FileManifestEntry{}
			entry.SetPath(m.Path)
			entry.SetHash(m.Hash)
			entry.SetSize(m.Size)
			entry.SetMode(m.Mode)
			rpcManifest[i] = entry
		}

		prepResult, prepErr := bc.PrepareUpload(buildCtx, name, rpcManifest)
		if prepErr == nil && prepResult.Result() != nil {
			result := prepResult.Result()
			sessionID = result.SessionId()
			cachedFiles = result.CachedCount()
			useOptimized = true

			if result.HasNeededPaths() && result.NeededPaths() != nil {
				neededPaths = make(map[string]bool)
				for _, p := range *result.NeededPaths() {
					neededPaths[p] = true
				}
			}
		} else {
			ctx.Log.Debug("prepareUpload unavailable, falling back to full upload", "error", prepErr)
		}
	} else {
		ctx.Log.Debug("manifest computation failed, falling back to full upload", "error", manifestErr)
	}

	// Compute uncompressed totals for progress estimation and cached bytes for the summary.
	var totalUncompressed int64
	var cachedBytes int64
	if manifest != nil {
		var totalManifestBytes int64
		for _, m := range manifest {
			totalManifestBytes += m.Size
		}
		if useOptimized && neededPaths != nil {
			for _, m := range manifest {
				if neededPaths[m.Path] {
					totalUncompressed += m.Size
				}
			}
			cachedBytes = totalManifestBytes - totalUncompressed
		} else {
			totalUncompressed = totalManifestBytes
		}
	}

	var uncompressedWritten atomic.Int64
	var r io.ReadCloser
	if useOptimized && len(neededPaths) > 0 {
		r, err = tarx.MakeFilteredTar(dir, includePatterns, neededPaths, &uncompressedWritten)
	} else if useOptimized {
		r = tarx.MakeEmptyTar()
	} else {
		r, err = tarx.MakeTar(dir, includePatterns, &uncompressedWritten)
	}
	if err != nil {
		uploadSpan.RecordError(err)
		uploadSpan.SetStatus(codes.Error, err.Error())
		uploadSpan.End()
		updateDeploymentOnError(fmt.Sprintf("Failed to create tar: %v", err))
		return err
	}

	defer r.Close()

	// buildCall wraps either BuildFromTar or BuildFromPrepared
	type buildResults interface {
		Version() string
		HasVersionShortId() bool
		VersionShortId() string
		HasAccessInfo() bool
		AccessInfo() *build_v1alpha.AccessInfo
	}
	buildCall := func(callCtx context.Context, tarReader io.ReadCloser, cb stream.SendStream[*build_v1alpha.Status]) (buildResults, error) {
		if useOptimized {
			tarStream := stream.ServeReader(callCtx, tarReader, stream.WithBulkBatching())
			return bc.BuildFromPrepared(callCtx, sessionID, tarStream, cb, envVars, ephemeralLabel, ephemeralTTL)
		}
		return bc.BuildFromTar(callCtx, name, stream.ServeReader(callCtx, tarReader, stream.WithBulkBatching()), cb, envVars, ephemeralLabel, ephemeralTTL)
	}

	var (
		cb      stream.SendStream[*build_v1alpha.Status]
		results buildResults
	)

	// Detect if we have a TTY - if not, force explain mode
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	useExplainMode := opts.Explain || !isTTY

	if useExplainMode {
		if useOptimized && cachedFiles > 0 {
			faintStyle := lipgloss.NewStyle().Faint(true)
			ctx.Printf("  %s\n", faintStyle.Render(fmt.Sprintf("Reused %d/%d files from previous deploy, uploading %d", cachedFiles, totalFiles, len(neededPaths))))
		}

		// In explain mode, write to stderr
		pw, err := progresswriter.NewPrinter(ctx, os.Stderr, opts.ExplainFormat)
		if err != nil {
			return err
		}
		safeStatus := newSafeStatusCh(pw.Status())
		defer safeStatus.Close()

		// Add upload progress tracking in explain mode
		uploadStartTime := time.Now()
		var uploadBytes int64
		var lastPrintTime time.Time

		progressReader := upload.NewProgressReader(r, func(progress upload.Progress) {
			enrichUploadProgress(&progress, &uncompressedWritten, totalUncompressed)
			uploadBytes = progress.BytesRead
			// Print progress every 500ms to avoid spamming
			if progress.Fraction > 0 && time.Since(lastPrintTime) >= 500*time.Millisecond {
				lastPrintTime = time.Now()
				fmt.Fprintf(os.Stderr, "\r\033[K") // Clear to end of line
				line := fmt.Sprintf("Uploading artifacts: %d%% — %s at %s",
					int(progress.Fraction*100),
					upload.FormatBytes(progress.BytesRead),
					upload.FormatSpeed(progress.BytesPerSecond))
				if progress.ETA > 0 {
					line += fmt.Sprintf(" (eta ~%s)", upload.FormatDuration(progress.ETA))
				}
				fmt.Fprint(os.Stderr, line)
			}
		})
		r = progressReader

		// Progress handler for explain mode
		progressHandler := func(status *client.SolveStatus) error {
			// Clear the upload progress line when buildkit starts
			if uploadBytes > 0 {
				uploadDuration := time.Since(uploadStartTime)
				avgSpeed := float64(uploadBytes) / uploadDuration.Seconds()
				summary := fmt.Sprintf("\rUpload complete: %s in %.1fs at %s",
					upload.FormatBytes(uploadBytes),
					uploadDuration.Seconds(),
					upload.FormatSpeed(avgSpeed))
				if useOptimized && cachedFiles > 0 {
					summary += fmt.Sprintf(", reused %d/%d files", cachedFiles, totalFiles)
					if cachedBytes > 0 && avgSpeed > 0 {
						savedSec := float64(cachedBytes) / avgSpeed
						summary += fmt.Sprintf(" (saved %s, ~%s)", upload.FormatBytes(cachedBytes), upload.FormatDuration(time.Duration(savedSec*float64(time.Second))))
					} else if cachedBytes > 0 {
						summary += fmt.Sprintf(" (saved %s)", upload.FormatBytes(cachedBytes))
					}
				}
				fmt.Fprintf(os.Stderr, "%s\n", summary)
				uploadBytes = 0 // Only print once
			}

			return safeStatus.Send(buildCtx, status)
		}

		cb = createBuildStatusCallback(buildCtx, nil, nil, &buildStateMu, &buildErrors, nil, &deployWarnings, progressHandler)

		results, err = buildCall(buildCtx, r, cb)
		if err != nil {
			uploadSpan.RecordError(err)
			uploadSpan.SetStatus(codes.Error, err.Error())
			uploadSpan.End()

			// Check if this was a context cancellation
			if buildCtx.Err() != nil {
				ctx.Printf("\n\n❌ Deploy cancelled.\n")
				// Don't update deployment status - it's already cancelled
				if !wasExternallyCancelled() {
					updateDeploymentOnError("Deploy cancelled by user")
				}
				return buildCtx.Err()
			}

			// Check if this was a server panic
			var panicErr cond.ErrPanic
			if errors.As(err, &panicErr) {
				ctx.Printf("\n\n❌ Build failed due to a server panic.\n")
				ctx.Printf("The server encountered a panic: %s\n", panicErr.Message)
				ctx.Printf("Check the server logs for more details.\n")
				updateDeploymentOnError(fmt.Sprintf("Server panic: %s", panicErr.Message))
				return err
			}

			ctx.Printf("\n\nBuild failed with the following errors:\n")
			errsSnap, _, _ := snapshotBuildState()
			printBuildErrors(ctx, errsSnap, nil)
			updateDeploymentOnError(fmt.Sprintf("Build failed: %v", err))
			return err
		}

		safeStatus.Close()
		<-pw.Done()

		if pw.Err() != nil {
			uploadSpan.RecordError(pw.Err())
			uploadSpan.SetStatus(codes.Error, pw.Err().Error())
			uploadSpan.End()
			return pw.Err()
		}
	} else {
		var (
			updateCh         = make(chan string, 1)
			buildCh          = make(chan buildProgress, 1)
			uploadProgressCh = make(chan upload.Progress, 1)
			wg               sync.WaitGroup
		)

		defer wg.Wait()

		progressReader := upload.NewProgressReader(r, func(progress upload.Progress) {
			enrichUploadProgress(&progress, &uncompressedWritten, totalUncompressed)
			select {
			case uploadProgressCh <- progress:
			default:
			}
		})
		r = progressReader

		// Create a context that can be cancelled by UI (child of buildCtx for external cancellation)
		deployCtx, cancelDeploy := context.WithCancel(buildCtx)
		defer cancelDeploy()

		model := initialModel(updateCh, buildCh, uploadProgressCh, cachedFiles, totalFiles, cachedBytes)
		p := tea.NewProgram(model)

		var finalModel tea.Model
		var runErr error

		wg.Add(1)
		go func() {
			defer wg.Done()
			finalModel, runErr = p.Run()
			if runErr == nil {
				if dm, ok := finalModel.(*deployInfo); ok && dm.interrupted {
					cancelDeploy()
				}
			} else {
				// UI died; ensure we don't keep uploading/building
				cancelDeploy()
			}
		}()

		defer p.Quit()

		// Progress handler for interactive mode
		progressHandler := func(status *client.SolveStatus) error {
			p.Send(status)
			return nil
		}

		cb = createBuildStatusCallback(deployCtx, updateCh, buildCh, &buildStateMu, &buildErrors, &buildLogs, &deployWarnings, progressHandler)

		results, err = buildCall(deployCtx, r, cb)

		// Ensure the progress UI is shut down before printing
		p.Quit()
		wg.Wait()

		// Get the final model to extract phase summaries
		if m, ok := finalModel.(*deployInfo); ok && m.currentPhase == "buildkit" && err == nil {
			// Complete the buildkit phase if it's still running and we succeeded
			duration := time.Since(m.phaseStart)
			buildPhase := phaseSummary{
				name:     "Build & push image",
				duration: duration,
				details:  buildStepsSummary(m.buildSteps),
			}

			// Only print the final build phase summary (TEA UI already showed the others)
			ctx.Printf("%s\n", renderPhaseSummary(buildPhase))

			// Update phase to pushing (build includes push in buildkit)
			updateDeploymentPhase("pushing")
		}

		if err != nil {
			uploadSpan.RecordError(err)
			uploadSpan.SetStatus(codes.Error, err.Error())
			uploadSpan.End()

			// Check if this was a user interruption (via UI flag or context cancellation)
			dm, isDeploy := finalModel.(*deployInfo)
			if (isDeploy && dm.interrupted) || deployCtx.Err() != nil {
				ctx.Printf("\n\n❌ Deploy cancelled.\n")
				// Don't update deployment status if externally cancelled - it's already cancelled
				if !wasExternallyCancelled() {
					updateDeploymentOnError("Deploy cancelled by user")
				}
				if deployCtx.Err() != nil {
					return deployCtx.Err()
				}
				if buildCtx.Err() != nil {
					return buildCtx.Err()
				}
				return context.Canceled
			}

			// Check if this was a server panic
			var panicErr cond.ErrPanic
			if errors.As(err, &panicErr) {
				ctx.Printf("\n\n❌ Build failed due to a server panic.\n")
				ctx.Printf("The server encountered a panic: %s\n", panicErr.Message)
				ctx.Printf("Check the server logs for more details.\n")
				updateDeploymentOnError(fmt.Sprintf("Server panic: %s", panicErr.Message))
				return err
			}

			ctx.Printf("\n\nBuild failed.\n")
			errsSnap, logsSnap, _ := snapshotBuildState()
			printBuildErrors(ctx, errsSnap, logsSnap)
			updateDeploymentOnError(fmt.Sprintf("Build failed: %v", err))
			return err
		}

	}

	if results.Version() == "" {
		noVersionErr := fmt.Errorf("build failed: no version returned")
		uploadSpan.RecordError(noVersionErr)
		uploadSpan.SetStatus(codes.Error, noVersionErr.Error())
		uploadSpan.End()
		ctx.Printf("\n\nError detected in building %s. No version returned.\n", name)
		errsSnap, logsSnap, _ := snapshotBuildState()
		printBuildErrors(ctx, errsSnap, logsSnap)
		updateDeploymentOnError("Build failed: no version returned")
		return noVersionErr
	}

	// Upload + build completed successfully
	uploadSpan.End()

	ctx.Log.Debug("Build completed", "version", results.Version())

	appVersionId := results.Version()
	if appVersionId == "" {
		updateDeploymentOnError("Build did not return a version")
		return fmt.Errorf("build did not return a version")
	}

	ctx.Log.Debug("Build completed with version", "version", appVersionId)

	if ephemeralLabel != "" {
		// Ephemeral deploy: no deployment record to update, just show info
		versionDisplay := ui.DisplayShortID(results.VersionShortId(), results.Version())
		ctx.Printf("\n\nEphemeral version %s created.\n", versionDisplay)
		ctx.Printf("  Label: %s\n", ephemeralLabel)
		ctx.Printf("  TTL:   %s\n", ephemeralTTL)

		// Show ephemeral access URLs (server returns resolved hostnames)
		if results.HasAccessInfo() && results.AccessInfo() != nil {
			info := results.AccessInfo()
			if info.HasHostnames() {
				for _, h := range *info.Hostnames() {
					ctx.Printf("  URL:   https://%s\n", h)
				}
			}
			if info.HasClusterHostname() && info.ClusterHostname() != "" {
				ctx.Printf("  URL:   https://%s.%s\n", ephemeralLabel, info.ClusterHostname())
			}
		}
	} else {
		// Normal deploy: update deployment tracking
		// Update phase to pushing (build completed, now pushing)
		updateDeploymentPhase("pushing")

		// Update deployment with actual app version ID
		_, err = depClient.UpdateDeploymentAppVersion(ctx, deploymentId, appVersionId)
		if err != nil {
			ctx.Log.Error("Failed to update deployment app version", "error", err)
			// Continue anyway - the deployment is proceeding
		}

		// Update phase to activating
		updateDeploymentPhase("activating")

		// Wrap finalization in a span
		finalizeCtx, finalizeSpan := deployTracer.Start(ctx.Context, "deploy.finalize")

		// Mark deployment as active
		_, err = depClient.UpdateDeploymentStatus(finalizeCtx, deploymentId, "active", "")
		if err != nil {
			// Log error but don't fail - deployment is already done
			ctx.Log.Error("Failed to update deployment status", "error", err)
		}
		finalizeSpan.End()
		deploymentFinalized = true

		versionDisplay := ui.DisplayShortID(results.VersionShortId(), results.Version())
		ctx.Printf("\n\nUpdated version %s deployed. All traffic moved to new version.\n", versionDisplay)
	}

	_, _, warnsSnap := snapshotBuildState()
	if len(warnsSnap) > 0 {
		warnHeaderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
		ctx.Printf("\n%s\n", warnHeaderStyle.Render("Warnings:"))
		for _, entry := range warnsSnap {
			renderDeployWarning(ctx, entry)
		}
	}

	// Show route/access information using server-provided data
	displayAccessInfo(ctx, name, results)

	return nil
}

// enrichUploadProgress fills in Fraction and ETA on a Progress snapshot using
// the atomic uncompressed-byte counter and the known total uncompressed size.
//
// Fraction is computed against uncompressed source bytes (not compressed bytes
// sent over the wire). We deliberately avoid projecting a compressed total:
// the source-bytes counter and the network-bytes counter are sampled on
// different sides of a gzip buffer plus io.Pipe, so their ratio swings wildly
// under back-pressure and produces a misleading "estimated total."
//
// ETA is extrapolated in the time domain — elapsed * (1 - frac) / frac — which
// avoids the unit-mixing that broke the old compressed-total math. It's only
// emitted after a brief warmup so the first few ticks (when Fraction is near
// zero) don't produce nonsense.
func enrichUploadProgress(p *upload.Progress, written *atomic.Int64, totalUncompressed int64) {
	if totalUncompressed <= 0 {
		return
	}
	p.Fraction = float64(written.Load()) / float64(totalUncompressed)
	if p.Fraction > 0 && p.Fraction < 1 && p.Duration >= 5*time.Second {
		remaining := float64(p.Duration) * (1 - p.Fraction) / p.Fraction
		p.ETA = time.Duration(remaining)
	}
}

// Helper function to print build errors and logs
func printBuildErrors(ctx *Context, buildErrors []string, buildLogs []string) {
	if len(buildErrors) > 0 {
		ctx.Printf("\nErrors:\n")
		for _, errMsg := range buildErrors {
			ctx.Printf("  - %s\n", errMsg)
		}
	}

	if len(buildLogs) > 0 {
		ctx.Printf("\nBuild output:\n")
		for _, log := range buildLogs {
			ctx.Printf("%s\n", log)
		}
	}
}

// safeStatusCh coordinates concurrent Send and Close on a buildkit status
// channel so the build status callback (invoked from RPC stream-handler
// goroutines that can outlive the parent RPC call — see pkg/rpc/client.go
// callInline) cannot race with the deploy command closing the channel.
//
// Close uses a stop channel to wake any in-flight Send rather than holding a
// mutex across the blocking channel send, so Close cannot deadlock even if
// the channel's consumer has stopped draining.
type safeStatusCh struct {
	ch     chan *client.SolveStatus
	stop   chan struct{}
	mu     sync.Mutex
	wg     sync.WaitGroup
	closed bool
}

func newSafeStatusCh(ch chan *client.SolveStatus) *safeStatusCh {
	return &safeStatusCh{ch: ch, stop: make(chan struct{})}
}

func (s *safeStatusCh) Send(ctx context.Context, v *client.SolveStatus) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()

	select {
	case <-s.stop:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case s.ch <- v:
		return nil
	}
}

func (s *safeStatusCh) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.stop)
	s.mu.Unlock()

	s.wg.Wait()
	close(s.ch)
}

// createBuildStatusCallback creates a callback for handling build status updates.
// stateMu must be non-nil and guards buildErrors, buildLogs, and deployWarnings
// — the callback runs from RPC stream-handler goroutines that race with
// readers in Deploy.
func createBuildStatusCallback(
	ctx context.Context,
	updateCh chan<- string,
	buildCh chan<- buildProgress,
	stateMu *sync.Mutex,
	buildErrors *[]string,
	buildLogs *[]string,
	deployWarnings *[]*build_v1alpha.LogEntry,
	progressHandler func(*client.SolveStatus) error,
) stream.SendStream[*build_v1alpha.Status] {
	vertices := map[string]bool{} // digest → completed
	return stream.Callback(func(su *build_v1alpha.Status) error {
		update := su.Update()

		switch update.Which() {
		case "buildkit":
			sj := update.Buildkit()

			var status client.SolveStatus
			if err := json.Unmarshal(sj, &status); err != nil {
				return err
			}

			// Track build step progress via vertices
			if buildCh != nil {
				var updated bool
				for _, v := range status.Vertexes {
					d := v.Digest.String()
					if _, seen := vertices[d]; !seen {
						updated = true
					}
					done := v.Completed != nil
					if done != vertices[d] {
						updated = true
					}
					vertices[d] = done
				}

				if updated {
					var completed int
					for _, done := range vertices {
						if done {
							completed++
						}
					}
					select {
					case <-ctx.Done():
					case buildCh <- buildProgress{total: len(vertices), completed: completed}:
					default:
					}
				}
			}

			// Call the progress handler if provided
			if progressHandler != nil {
				if err := progressHandler(&status); err != nil {
					return err
				}
			}

			stateMu.Lock()
			for _, vertex := range status.Vertexes {
				if vertex.Error != "" {
					*buildErrors = append(*buildErrors, vertex.Error)
				}
			}
			if buildLogs != nil {
				for _, log := range status.Logs {
					if log.Data != nil {
						logStr := strings.TrimSpace(string(log.Data))
						if logStr != "" {
							*buildLogs = append(*buildLogs, logStr)
						}
					}
				}
			}
			errCount := len(*buildErrors)
			stateMu.Unlock()

			if errCount > 0 {
				return fmt.Errorf("build failed with %d error(s)", errCount)
			}

			return nil
		case "message":
			msg := update.Message()
			if updateCh != nil {
				select {
				case updateCh <- msg:
					// sent successfully
				default:
					// drop if UI isn't consuming
				}
			}
		case "error":
			stateMu.Lock()
			*buildErrors = append(*buildErrors, update.Error())
			stateMu.Unlock()
		case "log":
			if entry := update.Log(); entry != nil {
				switch entry.Level() {
				case "warn":
					if deployWarnings != nil {
						stateMu.Lock()
						*deployWarnings = append(*deployWarnings, entry)
						stateMu.Unlock()
					}
				case "info":
					if updateCh != nil {
						select {
						case updateCh <- entry.Text():
						default:
						}
					}
				}
			}
		}

		return nil
	})
}

func renderDeployWarning(ctx *Context, entry *build_v1alpha.LogEntry) {
	orange := lipgloss.Color("208")
	headerStyle := lipgloss.NewStyle().Foreground(orange).Bold(true)
	linkStyle := lipgloss.NewStyle().Foreground(orange).Faint(true)

	// Compute wrap width: terminal width minus indent (4 chars), capped at 76
	const indent = 4
	const maxWidth = 76
	detailWidth := maxWidth
	if tw := ui.TerminalWidth(); tw > 0 {
		if available := tw - indent; available > 0 {
			detailWidth = min(available, maxWidth)
		}
	}
	detailStyle := lipgloss.NewStyle().Foreground(orange).Width(detailWidth).PaddingLeft(indent)

	ctx.Printf("  %s\n", headerStyle.Render("⚠ "+entry.Text()))

	// Index fields by key for controlled rendering order
	fields := make(map[string]string)
	for _, f := range entry.Fields() {
		fields[f.Key()] = f.Value()
	}

	if detail, ok := fields["detail"]; ok {
		ctx.Printf("%s\n", detailStyle.Render(detail))
	}
	if link, ok := fields["link"]; ok {
		ctx.Printf("    %s%s\n", linkStyle.Render("See: "), ui.RenderMarkdownLink(link, 208))
	}
}

func buildStepsSummary(count int) string {
	if count == 0 {
		return "cached"
	}
	if count == 1 {
		return "1 step completed"
	}
	return fmt.Sprintf("%d steps completed", count)
}

// deployAccessInfo provides access to build result access info for display purposes.
type deployAccessInfo interface {
	HasAccessInfo() bool
	AccessInfo() *build_v1alpha.AccessInfo
}

// displayAccessInfo shows how to access the deployed app using server-provided access info
func displayAccessInfo(ctx *Context, appName string, results deployAccessInfo) {
	// Check if we have access info from the server
	if !results.HasAccessInfo() {
		ctx.Log.Debug("No access info returned from server")
		return
	}

	accessInfo := results.AccessInfo()

	// Get hostnames and default route status from server
	var hostnames []string
	if accessInfo.HasHostnames() && accessInfo.Hostnames() != nil {
		hostnames = *accessInfo.Hostnames()
	}
	hasDefaultRoute := accessInfo.DefaultRoute()

	// Get cluster address for default route display
	// Prefer the cloud-provisioned DNS hostname from the server if available
	var clusterAddr string
	if accessInfo.ClusterHostname() != "" {
		// Use the cloud-provisioned DNS hostname (e.g., cluster-abc.org-123.miren.systems)
		clusterAddr = accessInfo.ClusterHostname()
	} else if ctx.ClusterConfig != nil && ctx.ClusterConfig.Hostname != "" {
		// Fall back to the client's cluster address
		// Strip any API port (e.g. :8443) since HTTP ingress is on 443
		clusterAddr = stripPort(ctx.ClusterConfig.Hostname)
	}

	// Display access information
	if len(hostnames) > 0 {
		ctx.Printf("\nYour app is available at:\n")
		for _, host := range hostnames {
			ctx.Printf("  https://%s\n", host)
		}
		if hasDefaultRoute {
			ctx.Printf("  (also the default route)\n")
		}
	} else if hasDefaultRoute {
		if clusterAddr != "" {
			ctx.Printf("\nYour app is the default route, available at:\n")
			ctx.Printf("  https://%s\n", clusterAddr)
		} else {
			ctx.Printf("\nYour app is the default route and will receive all unmatched traffic.\n")
		}
		suggestRoute(ctx, appName, accessInfo.ClusterHostname())
	} else {
		ctx.Printf("\nNo routes configured for this app.\n")
		suggestRoute(ctx, appName, accessInfo.ClusterHostname())
		ctx.Printf("To make it the default route: miren route set-default %s\n", appName)
	}
}

// suggestRoute suggests a route command, using the cloud DNS hostname if available
func suggestRoute(ctx *Context, appName string, clusterHostname string) {
	if clusterHostname != "" {
		// Suggest a specific subdomain using the app name
		subdomain := sanitizeForSubdomain(appName)
		suggestedHost := subdomain + "." + clusterHostname
		ctx.Printf("To set a hostname, try: miren route set %s %s\n", suggestedHost, appName)
	} else {
		ctx.Printf("To set a hostname, try: miren route set <hostname> %s\n", appName)
	}
}

// sanitizeForSubdomain converts an app name to a valid subdomain label
func sanitizeForSubdomain(name string) string {
	// Convert to lowercase
	result := strings.ToLower(name)
	// Replace underscores with hyphens
	result = strings.ReplaceAll(result, "_", "-")
	// Replace any other non-alphanumeric chars with hyphens
	var sanitized strings.Builder
	for _, r := range result {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			sanitized.WriteRune(r)
		} else {
			sanitized.WriteRune('-')
		}
	}
	result = sanitized.String()
	// Remove leading/trailing hyphens
	result = strings.Trim(result, "-")
	// Collapse multiple hyphens
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	// Ensure it's not empty
	if result == "" {
		result = "app"
	}
	return result
}

// stripPort removes any port suffix from a host string
func stripPort(host string) string {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

// Styles for analyze output
var (
	analyzeTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("3")) // yellow

	analyzeLabelStyle = lipgloss.NewStyle().
				Faint(true).
				Width(12).
				Align(lipgloss.Right)

	analyzeValueStyle = lipgloss.NewStyle().
				Bold(true)

	// Badge styles for different event kinds
	badgeFile = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")). // blue
			Bold(true)
	badgePackage = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")). // green
			Bold(true)
	badgeFramework = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")). // magenta
			Bold(true)
	badgeConfig = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // yellow
			Bold(true)
	badgeDir = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")). // cyan
			Bold(true)
	badgeScript = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")). // orange
			Bold(true)
)

func eventKindBadge(kind string) string {
	badge := fmt.Sprintf("[%s]", kind)
	switch kind {
	case "file":
		return badgeFile.Render(badge)
	case "package":
		return badgePackage.Render(badge)
	case "framework":
		return badgeFramework.Render(badge)
	case "config":
		return badgeConfig.Render(badge)
	case "dir":
		return badgeDir.Render(badge)
	case "script":
		return badgeScript.Render(badge)
	default:
		return lipgloss.NewStyle().Faint(true).Render(badge)
	}
}

// analyzeApp calls the AnalyzeApp API and displays the results
func analyzeApp(ctx *Context, bc *build_v1alpha.BuilderClient, dir string) error {
	if dir == "" || dir == "." {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	ctx.Printf("Analyzing app in %s...\n\n", dir)

	// Load AppConfig to get include patterns
	var includePatterns []string
	ac, err := appconfig.LoadAppConfigUnder(dir)
	if err != nil {
		return fmt.Errorf("failed to load app config: %w", err)
	}
	if ac != nil && ac.Include != nil {
		for _, pattern := range ac.Include {
			if err := tarx.ValidatePattern(pattern); err != nil {
				return fmt.Errorf("invalid include pattern %q: %w", pattern, err)
			}
		}
		includePatterns = ac.Include
	}

	r, err := tarx.MakeTar(dir, includePatterns, nil)
	if err != nil {
		return fmt.Errorf("failed to create tar: %w", err)
	}

	defer r.Close()

	result, err := bc.AnalyzeApp(ctx, stream.ServeReader(ctx, r, stream.WithBulkBatching()))
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	analysisResult := result.Result()
	if analysisResult == nil {
		return fmt.Errorf("no analysis result returned")
	}

	// Stack
	ctx.Printf("%s %s\n",
		analyzeLabelStyle.Render("Stack:"),
		analyzeValueStyle.Render(analysisResult.Stack()))

	// App name (if from app.toml)
	if analysisResult.AppName() != "" {
		ctx.Printf("%s %s\n",
			analyzeLabelStyle.Render("App Name:"),
			analyzeValueStyle.Render(analysisResult.AppName()))
	}

	// Working directory
	ctx.Printf("%s %s\n",
		analyzeLabelStyle.Render("Directory:"),
		analysisResult.WorkingDir())

	// Entrypoint
	if analysisResult.Entrypoint() != "" {
		ctx.Printf("%s %s\n",
			analyzeLabelStyle.Render("Entrypoint:"),
			analyzeValueStyle.Render(analysisResult.Entrypoint()))
	}

	// Dockerfile (if using dockerfile stack)
	if analysisResult.BuildDockerfile() != "" {
		ctx.Printf("%s %s\n",
			analyzeLabelStyle.Render("Dockerfile:"),
			analysisResult.BuildDockerfile())
	}

	// Services
	if analysisResult.HasServices() && analysisResult.Services() != nil {
		services := *analysisResult.Services()
		if len(services) > 0 {
			ctx.Printf("\n%s\n", analyzeTitleStyle.Render("Services"))
			for _, svc := range services {
				sourceInfo := ""
				if svc.Source() != "" {
					sourceInfo = lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf(" (%s)", svc.Source()))
				}

				command := svc.Command()
				if command == "" {
					// Service uses Dockerfile CMD (image default)
					command = lipgloss.NewStyle().Faint(true).Italic(true).Render("image default")
				}

				ctx.Printf("  %s: %s%s\n",
					analyzeValueStyle.Render(svc.Name()),
					command,
					sourceInfo)
			}
		}
	}

	// Environment variables with local detection
	if analysisResult.HasEnvVars() && analysisResult.EnvVars() != nil {
		envVars := *analysisResult.EnvVars()
		if len(envVars) > 0 {
			// Cross-reference with local environment
			localDetection := DetectLocalEnvVars(envVars)

			ctx.Printf("\n%s\n", analyzeTitleStyle.Render("Environment Variables"))

			// Show available (detected + found locally)
			if len(localDetection.Available) > 0 {
				ctx.Printf("  %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("Available locally:"))
				for _, ev := range localDetection.Available {
					valueDisplay := MaskValue(ev.Value, ev.Sensitive)
					if ev.Sensitive {
						ctx.Printf("    %s %s=%s\n",
							lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓"),
							ev.Key,
							lipgloss.NewStyle().Faint(true).Render(valueDisplay))
					} else {
						ctx.Printf("    %s %s=%s\n",
							lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓"),
							ev.Key,
							valueDisplay)
					}
				}
			}

			// Show missing (detected but not found locally)
			if len(localDetection.Missing) > 0 {
				ctx.Printf("  %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("Not set locally:"))
				for _, ev := range localDetection.Missing {
					ctx.Printf("    %s %s\n",
						lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("○"),
						ev.Key)
				}
			}

			// Show additional app-related env vars found locally
			if len(localDetection.Additional) > 0 {
				ctx.Printf("  %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("Also found locally (may be relevant):"))
				for _, ev := range localDetection.Additional {
					valueDisplay := MaskValue(ev.Value, ev.Sensitive)
					if ev.Sensitive {
						ctx.Printf("    %s %s=%s\n",
							lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("?"),
							ev.Key,
							lipgloss.NewStyle().Faint(true).Render(valueDisplay))
					} else {
						ctx.Printf("    %s %s=%s\n",
							lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("?"),
							ev.Key,
							valueDisplay)
					}
				}
			}
		}
	} else {
		// No detected env vars - still scan local environment for suggestions
		localDetection := DetectLocalEnvVars(nil)
		if len(localDetection.Additional) > 0 {
			ctx.Printf("\n%s\n", analyzeTitleStyle.Render("Environment Variables"))
			ctx.Printf("  %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("Found locally (may be relevant):"))
			for _, ev := range localDetection.Additional {
				valueDisplay := MaskValue(ev.Value, ev.Sensitive)
				if ev.Sensitive {
					ctx.Printf("    %s %s=%s\n",
						lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("?"),
						ev.Key,
						lipgloss.NewStyle().Faint(true).Render(valueDisplay))
				} else {
					ctx.Printf("    %s %s=%s\n",
						lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("?"),
						ev.Key,
						valueDisplay)
				}
			}
		}
	}

	// Detection events
	if analysisResult.HasEvents() && analysisResult.Events() != nil {
		events := *analysisResult.Events()
		if len(events) > 0 {
			ctx.Printf("\n%s\n", analyzeTitleStyle.Render("Detection"))
			for _, event := range events {
				badge := eventKindBadge(event.Kind())
				if event.Name() != "" {
					ctx.Printf("  %s %s: %s\n",
						badge,
						analyzeValueStyle.Render(event.Name()),
						event.Message())
				} else {
					ctx.Printf("  %s %s\n", badge, event.Message())
				}
			}
		}
	}

	return nil
}
