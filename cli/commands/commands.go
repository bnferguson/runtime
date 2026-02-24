package commands

import (
	"miren.dev/mflags"
	"miren.dev/runtime/pkg/labs"
)

func RegisterAll(d *mflags.Dispatcher) {
	// Core commands
	d.Dispatch("version", Infer("version", "Print the version", Version,
		WithExample(mflags.Example{
			Name: "Print version",
			Body: "miren version",
		}),
		WithExample(mflags.Example{
			Name: "JSON output",
			Body: "miren version --format json",
		}),
	))
	d.Dispatch("login", Infer("login", "Authenticate with miren.cloud", Login,
		WithExample(mflags.Example{
			Name: "Login",
			Body: "miren login",
		}),
		WithExample(mflags.Example{
			Name: "Login to a specific cloud instance",
			Body: "miren login --url https://cloud.example.com",
		}),
	))
	d.Dispatch("logout", Infer("logout", "Remove local authentication credentials", Logout,
		WithExample(mflags.Example{
			Name: "Logout",
			Body: "miren logout",
		}),
	))
	d.Dispatch("whoami", Infer("whoami", "Display information about the current authenticated user", Whoami,
		WithExample(mflags.Example{
			Name: "Show current user",
			Body: "miren whoami",
		}),
		WithExample(mflags.Example{
			Name: "JSON output",
			Body: "miren whoami --json",
		}),
	))

	// Doctor commands
	d.Dispatch("doctor", Infer("doctor", "Diagnose miren environment and connectivity", Doctor,
		WithExample(mflags.Example{
			Name: "Run all diagnostics",
			Body: "miren doctor",
		}),
	))
	d.Dispatch("doctor config", Infer("doctor config", "Check configuration files", DoctorConfig,
		WithExample(mflags.Example{
			Name: "Check config files",
			Body: "miren doctor config",
		}),
	))
	d.Dispatch("doctor server", Infer("doctor server", "Check server health and connectivity", DoctorServer,
		WithExample(mflags.Example{
			Name: "Check server connectivity",
			Body: "miren doctor server",
		}),
	))
	d.Dispatch("doctor auth", Infer("doctor auth", "Check authentication and user information", DoctorAuth,
		WithExample(mflags.Example{
			Name: "Check authentication",
			Body: "miren doctor auth",
		}),
	))

	// App lifecycle commands
	d.Dispatch("init", Infer("init", "Initialize a new application", Init,
		WithExample(mflags.Example{
			Name: "Initialize in current directory",
			Body: "miren init",
		}),
		WithExample(mflags.Example{
			Name: "Initialize with a specific name",
			Body: "miren init --name myapp",
		}),
	))
	d.Dispatch("deploy", Infer("deploy", "Deploy an application", Deploy,
		WithExample(mflags.Example{
			Name: "Basic",
			Body: "miren deploy",
		}),
		WithExample(mflags.Example{
			Name: "Analyze",
			Body: `Before deploying, the system can tell you how it's going
to treat your application by running:

miren deploy --analyze
`,
		}),
		WithExample(mflags.Example{
			Name: "Set environment variables during deploy",
			Body: "miren deploy -e DATABASE_URL=postgres://localhost/mydb",
		}),
		WithExample(mflags.Example{
			Name: "Deploy a previously built version",
			Body: "miren deploy --version v3",
		}),
	))
	d.Dispatch("deploy cancel", Infer("deploy cancel", "Cancel an in-progress deployment", DeployCancel,
		WithExample(mflags.Example{
			Name: "Cancel the current deployment",
			Body: "miren deploy cancel",
		}),
		WithExample(mflags.Example{
			Name: "Cancel a specific deployment",
			Body: "miren deploy cancel -d dep_abc123",
		}),
	))
	d.Dispatch("rollback", Infer("rollback", "Roll back to a previous version", Rollback,
		WithExample(mflags.Example{
			Name: "Rollback the app in the current directory",
			Body: "miren rollback",
		}),
		WithExample(mflags.Example{
			Name: "Rollback a specific app",
			Body: "miren rollback -a myapp",
		}),
	))
	d.Dispatch("logs", Infer("logs", "Get logs for an application", Logs,
		WithDescription(logsDescription),
		WithExample(mflags.Example{
			Name: "View logs for the current app",
			Body: "miren logs",
		}),
		WithExample(mflags.Example{
			Name: "Follow logs in real time",
			Body: "miren logs -f",
		}),
		WithExample(mflags.Example{
			Name: "Show logs from the last 5 minutes, filtered for errors",
			Body: "miren logs --last 5m -g error",
		}),
		WithExample(mflags.Example{
			Name: "Filter logs by service",
			Body: "miren logs --service web -f",
		}),
	))

	// App management commands
	d.Dispatch("app", Infer("app", "Get information about an application", App,
		WithExample(mflags.Example{
			Name: "Show app info for the current directory",
			Body: "miren app",
		}),
		WithExample(mflags.Example{
			Name: "Show info for a specific app",
			Body: "miren app -a myapp",
		}),
		WithExample(mflags.Example{
			Name: "Watch app stats in real time",
			Body: "miren app --watch",
		}),
		WithExample(mflags.Example{
			Name: "Show only the app configuration",
			Body: "miren app --config-only",
		}),
	))
	d.Dispatch("app list", Infer("app list", "List all applications", AppList,
		WithExample(mflags.Example{
			Name: "List all apps",
			Body: "miren app list",
		}),
		WithExample(mflags.Example{
			Name: "List apps as JSON",
			Body: "miren app list --format json",
		}),
	))
	d.Dispatch("app status", Infer("app status", "Show current status of an application", AppStatus,
		WithExample(mflags.Example{
			Name: "Show status for the current app",
			Body: "miren app status",
		}),
		WithExample(mflags.Example{
			Name: "Show status for a specific app",
			Body: "miren app status -a myapp",
		}),
	))
	d.Dispatch("app history", Infer("app history", "Show deployment history for an application", AppHistory,
		WithExample(mflags.Example{
			Name: "Show deployment history",
			Body: "miren app history",
		}),
		WithExample(mflags.Example{
			Name: "Show detailed history with git info",
			Body: "miren app history --detailed",
		}),
		WithExample(mflags.Example{
			Name: "Show only active deployments, limited to 5",
			Body: "miren app history --status active --limit 5",
		}),
	))
	d.Dispatch("app delete", Infer("app delete", "Delete an application and all its resources", AppDelete,
		WithExample(mflags.Example{
			Name: "Delete an app (with confirmation prompt)",
			Body: "miren app delete myapp",
		}),
		WithExample(mflags.Example{
			Name: "Delete without confirmation",
			Body: "miren app delete myapp --force",
		}),
	))
	d.Dispatch("app run", Infer("app run", "Open interactive shell in a new sandbox", AppRun,
		WithDescription(appRunDescription),
		WithExample(mflags.Example{
			Name: "Open a shell in your app's environment",
			Body: "miren app run",
		}),
		WithExample(mflags.Example{
			Name: "Run a specific command",
			Body: "miren app run -- bin/rails console",
		}),
		WithExample(mflags.Example{
			Name: "Run database migrations",
			Body: "miren app run -- bin/rails db:migrate",
		}),
	))
	d.Dispatch("apps", Infer("apps", "List all applications (alias for 'app list')", AppList,
		WithExample(mflags.Example{
			Name: "List all apps",
			Body: "miren apps",
		}),
	))

	// Sandbox commands
	d.Dispatch("sandbox", Section("sandbox", "Sandbox management commands", "", WithSectionDescription(sandboxSectionDescription)))
	d.Dispatch("sandbox list", Infer("sandbox list", "List sandboxes (excludes dead by default)", SandboxList,
		WithExample(mflags.Example{
			Name: "List running sandboxes",
			Body: "miren sandbox list",
		}),
		WithExample(mflags.Example{
			Name: "Include dead sandboxes",
			Body: "miren sandbox list --all",
		}),
		WithExample(mflags.Example{
			Name: "List as JSON",
			Body: "miren sandbox list --format json",
		}),
	))
	d.Dispatch("sandbox stop", Infer("sandbox stop", "Stop a sandbox", SandboxStop,
		WithExample(mflags.Example{
			Name: "Stop a sandbox by ID",
			Body: "miren sandbox stop sb_abc123",
		}),
	))
	d.Dispatch("sandbox delete", Infer("sandbox delete", "Delete a dead sandbox", SandboxDelete,
		WithExample(mflags.Example{
			Name: "Delete a sandbox",
			Body: "miren sandbox delete sb_abc123",
		}),
		WithExample(mflags.Example{
			Name: "Force delete without confirmation",
			Body: "miren sandbox delete sb_abc123 --force",
		}),
	))
	d.Dispatch("sandbox exec", Infer("sandbox exec", "Open interactive shell in an existing sandbox", SandboxExec,
		WithDescription(sandboxExecDescription),
		WithExample(mflags.Example{
			Name: "Open a shell in a running sandbox",
			Body: "miren sandbox exec --id sb_abc123",
		}),
		WithExample(mflags.Example{
			Name: "Run a command in a sandbox",
			Body: "miren sandbox exec --id sb_abc123 -- ls -la /app",
		}),
	))

	// Sandbox pool commands
	d.Dispatch("sandbox-pool", Section("sandbox-pool", "Sandbox pool management commands", ""))
	d.Dispatch("sandbox-pool list", Infer("sandbox-pool list", "List all sandbox pools", SandboxPoolList,
		WithExample(mflags.Example{
			Name: "List all pools",
			Body: "miren sandbox-pool list",
		}),
	))
	d.Dispatch("sandbox-pool set-desired", Infer("sandbox-pool set-desired", "Set desired instance count for a sandbox pool", SandboxPoolSetDesired,
		WithExample(mflags.Example{
			Name: "Scale a pool to 3 instances",
			Body: "miren sandbox-pool set-desired web 3",
		}),
	))

	// Environment variable commands
	d.Dispatch("env", Section("env", "Environment variable management commands", ""))
	d.Dispatch("env set", Infer("env set", "Set environment variables for an application", EnvSet,
		WithExample(mflags.Example{
			Name: "Set an environment variable",
			Body: "miren env set -e DATABASE_URL=postgres://localhost/mydb",
		}),
		WithExample(mflags.Example{
			Name: "Set a sensitive variable (prompted with masking)",
			Body: "miren env set -s SECRET_KEY",
		}),
		WithExample(mflags.Example{
			Name: "Set a variable from a file",
			Body: "miren env set -e CONFIG=@config.json",
		}),
		WithExample(mflags.Example{
			Name: "Set a variable for a specific service",
			Body: "miren env set -e WORKERS=4 --service worker",
		}),
	))
	d.Dispatch("env get", Infer("env get", "Get an environment variable value", EnvGet,
		WithExample(mflags.Example{
			Name: "Get a variable value",
			Body: "miren env get DATABASE_URL",
		}),
		WithExample(mflags.Example{
			Name: "Reveal a sensitive variable",
			Body: "miren env get SECRET_KEY --unmask",
		}),
	))
	d.Dispatch("env list", Infer("env list", "List all environment variables", EnvList,
		WithExample(mflags.Example{
			Name: "List all variables",
			Body: "miren env list",
		}),
		WithExample(mflags.Example{
			Name: "List as JSON",
			Body: "miren env list --format json",
		}),
	))
	d.Dispatch("env delete", Infer("env delete", "Delete environment variables", EnvDelete,
		WithExample(mflags.Example{
			Name: "Delete a variable",
			Body: "miren env delete DATABASE_URL",
		}),
		WithExample(mflags.Example{
			Name: "Delete without confirmation",
			Body: "miren env delete DATABASE_URL --force",
		}),
		WithExample(mflags.Example{
			Name: "Delete a service-specific variable",
			Body: "miren env delete WORKERS --service worker",
		}),
	))

	// Addon commands (gated behind labs flag)
	if labs.Addons() {
		d.Dispatch("addon", Section("addon", "Addon management commands", ""))
		d.Dispatch("addon list-available", Infer("addon list-available", "List available addons", AddonListAvailable,
			WithLabsFeature(labs.FeatureAddons),
			WithExample(mflags.Example{
				Name: "List available addons",
				Body: "miren addon list-available",
			}),
		))
		d.Dispatch("addon variants", Infer("addon variants", "Show variants for an addon", AddonVariants,
			WithLabsFeature(labs.FeatureAddons),
			WithExample(mflags.Example{
				Name: "Show variants for PostgreSQL",
				Body: "miren addon variants miren-postgresql",
			}),
		))
		d.Dispatch("addon create", Infer("addon create", "Attach an addon to an application", AddonCreate,
			WithLabsFeature(labs.FeatureAddons),
			WithExample(mflags.Example{
				Name: "Attach a PostgreSQL addon",
				Body: "miren addon create miren-postgresql:small",
			}),
		))
		d.Dispatch("addon list", Infer("addon list", "List addons attached to an application", AddonList,
			WithLabsFeature(labs.FeatureAddons),
			WithExample(mflags.Example{
				Name: "List addons for the current app",
				Body: "miren addon list",
			}),
		))
		d.Dispatch("addon destroy", Infer("addon destroy", "Remove an addon from an application", AddonDestroy,
			WithLabsFeature(labs.FeatureAddons),
			WithExample(mflags.Example{
				Name: "Remove an addon",
				Body: "miren addon destroy miren-postgresql",
			}),
			WithExample(mflags.Example{
				Name: "Remove without confirmation",
				Body: "miren addon destroy miren-postgresql --force",
			}),
		))
	}

	// Route commands
	d.Dispatch("route", Infer("route", "List all HTTP routes", Route,
		WithExample(mflags.Example{
			Name: "List all routes",
			Body: "miren route",
		}),
	))
	d.Dispatch("route list", Infer("route list", "List all HTTP routes", RouteList,
		WithExample(mflags.Example{
			Name: "List all routes",
			Body: "miren route list",
		}),
		WithExample(mflags.Example{
			Name: "List as JSON",
			Body: "miren route list --format json",
		}),
	))
	d.Dispatch("route set", Infer("route set", "Create or update an HTTP route", RouteSet,
		WithExample(mflags.Example{
			Name: "Route a domain to an app",
			Body: "miren route set example.com myapp",
		}),
	))
	d.Dispatch("route remove", Infer("route remove", "Remove an HTTP route", RouteRemove,
		WithExample(mflags.Example{
			Name: "Remove a route",
			Body: "miren route remove example.com",
		}),
	))
	d.Dispatch("route show", Infer("route show", "Show details of an HTTP route", RouteShow,
		WithExample(mflags.Example{
			Name: "Show route details",
			Body: "miren route show example.com",
		}),
	))
	d.Dispatch("route set-default", Infer("route set-default", "Set an app as the default route", RouteSetDefault,
		WithExample(mflags.Example{
			Name: "Set the default route",
			Body: "miren route set-default myapp",
		}),
	))
	d.Dispatch("route unset-default", Infer("route unset-default", "Remove the default route", RouteUnsetDefault,
		WithExample(mflags.Example{
			Name: "Remove the default route",
			Body: "miren route unset-default",
		}),
	))

	// Route OIDC commands - behind feature flag
	if labs.RouteOIDC() {
		d.Dispatch("route oidc", Section("route oidc", "OIDC authentication management for routes", ""))
		d.Dispatch("route oidc enable", Infer("route oidc enable", "Enable OIDC authentication for a route", RouteOidcEnable,
			WithLabsFeature(labs.FeatureRouteOIDC),
			WithExample(mflags.Example{
				Name: "Enable OIDC with an existing provider",
				Body: "miren route oidc enable example.com --provider my-google-oidc",
			}),
			WithExample(mflags.Example{
				Name: "Enable OIDC with inline provider creation",
				Body: `miren route oidc enable example.com \
  --provider-url https://accounts.google.com \
  --client-id my-client-id \
  --client-secret my-client-secret`,
			}),
		))
		d.Dispatch("route oidc disable", Infer("route oidc disable", "Disable OIDC authentication for a route", RouteOidcDisable,
			WithLabsFeature(labs.FeatureRouteOIDC),
			WithExample(mflags.Example{
				Name: "Disable OIDC on a route",
				Body: "miren route oidc disable example.com",
			}),
		))
		d.Dispatch("route oidc show", Infer("route oidc show", "Show OIDC configuration for a route", RouteOidcShow,
			WithLabsFeature(labs.FeatureRouteOIDC),
			WithExample(mflags.Example{
				Name: "Show OIDC config for a route",
				Body: "miren route oidc show example.com",
			}),
		))
	}

	// Config commands
	d.Dispatch("config", Section("config", "Configuration file management", ""))
	d.Dispatch("config info", Infer("config info", "Show configuration file locations and format", ConfigInfo,
		WithExample(mflags.Example{
			Name: "Show config info",
			Body: "miren config info",
		}),
	))
	d.Dispatch("config load", Infer("config load", "Load config and merge it with your current config", ConfigLoad,
		WithExample(mflags.Example{
			Name: "Load a config file",
			Body: "miren config load --input cluster-config.yaml",
		}),
		WithExample(mflags.Example{
			Name: "Load and set as active cluster",
			Body: "miren config load --input cluster-config.yaml --set-active",
		}),
	))

	// Cluster commands
	d.Dispatch("cluster", Infer("cluster", "List configured clusters", Cluster,
		WithExample(mflags.Example{
			Name: "List clusters",
			Body: "miren cluster",
		}),
	))
	d.Dispatch("cluster list", Infer("cluster list", "List all configured clusters", ClusterList,
		WithExample(mflags.Example{
			Name: "List all clusters",
			Body: "miren cluster list",
		}),
		WithExample(mflags.Example{
			Name: "List as JSON",
			Body: "miren cluster list --format json",
		}),
	))
	d.Dispatch("cluster switch", Infer("cluster switch", "Switch to a different cluster", ClusterSwitch,
		WithExample(mflags.Example{
			Name: "Switch to a cluster",
			Body: "miren cluster switch production",
		}),
	))
	d.Dispatch("cluster add", Infer("cluster add", "Add a new cluster configuration", ClusterAdd,
		WithExample(mflags.Example{
			Name: "Add a cluster interactively",
			Body: "miren cluster add",
		}),
		WithExample(mflags.Example{
			Name: "Add a cluster with a specific address",
			Body: "miren cluster add --cluster my-cluster --address 10.0.0.1:8443",
		}),
	))
	d.Dispatch("cluster remove", Infer("cluster remove", "Remove a cluster from the configuration", ClusterRemove,
		WithExample(mflags.Example{
			Name: "Remove a cluster",
			Body: "miren cluster remove my-cluster",
		}),
	))
	d.Dispatch("cluster current", Infer("cluster current", "Show the pinned cluster for this app", ClusterCurrent,
		WithExample(mflags.Example{
			Name: "Show current cluster",
			Body: "miren cluster current",
		}),
	))

	// Runner commands (distributed runners) - behind feature flag
	if labs.DistributedRunners() {
		d.Dispatch("runner", Section("runner", "Runner management commands", ""))
		d.Dispatch("runner invite", Infer("runner invite", "Create a join code for a new runner", RunnerInvite,
			WithLabsFeature(labs.FeatureDistributedRunners),
			WithExample(mflags.Example{
				Name: "Create an invite",
				Body: "miren runner invite",
			}),
			WithExample(mflags.Example{
				Name: "Create an invite with labels and custom expiry",
				Body: "miren runner invite -l region=us-east -e 24",
			}),
		))
		d.Dispatch("runner join", Infer("runner join", "Join this machine to a coordinator as a runner", RunnerJoin,
			WithLabsFeature(labs.FeatureDistributedRunners),
			WithExample(mflags.Example{
				Name: "Join using a coordinator address and invite code",
				Body: "miren runner join coordinator.example.com:8443 abc123",
			}),
		))
		d.Dispatch("runner start", Infer("runner start", "Start this machine as a distributed runner", RunnerStart,
			WithLabsFeature(labs.FeatureDistributedRunners),
			WithExample(mflags.Example{
				Name: "Start the runner",
				Body: "miren runner start",
			}),
		))
		d.Dispatch("runner list", Infer("runner list", "List all registered runners", RunnerList,
			WithLabsFeature(labs.FeatureDistributedRunners),
			WithExample(mflags.Example{
				Name: "List runners",
				Body: "miren runner list",
			}),
		))
		d.Dispatch("runner revoke", Infer("runner revoke", "Revoke a runner invitation", RunnerRevoke,
			WithLabsFeature(labs.FeatureDistributedRunners),
			WithExample(mflags.Example{
				Name: "Revoke an invite",
				Body: "miren runner revoke inv_abc123",
			}),
		))
		d.Dispatch("runner invite list", Infer("runner invite list", "List all runner invitations", RunnerInviteList,
			WithLabsFeature(labs.FeatureDistributedRunners),
			WithExample(mflags.Example{
				Name: "List invitations",
				Body: "miren runner invite list",
			}),
		))
	}

	// Server commands
	d.Dispatch("server", Infer("server", "Start the miren server", Server,
		WithExample(mflags.Example{
			Name: "Start in standalone mode",
			Body: "miren server --mode standalone",
		}),
	))
	d.Dispatch("server config", Section("server config", "Server configuration management commands", ""))
	d.Dispatch("server config generate", Infer("server config generate", "Generate a server configuration file from current settings", ServerConfigGenerate,
		WithExample(mflags.Example{
			Name: "Generate config with defaults",
			Body: "miren server config generate --defaults",
		}),
		WithExample(mflags.Example{
			Name: "Generate and save to file",
			Body: "miren server config generate --defaults --output server.toml",
		}),
	))
	d.Dispatch("server config validate", Infer("server config validate", "Validate a server configuration file", ServerConfigValidate,
		WithExample(mflags.Example{
			Name: "Validate a config file",
			Body: "miren server config validate --file server.toml",
		}),
	))
	d.Dispatch("server upgrade", Infer("server upgrade", "Upgrade miren server", ServerUpgrade,
		WithExample(mflags.Example{
			Name: "Upgrade to the latest version",
			Body: "miren server upgrade",
		}),
		WithExample(mflags.Example{
			Name: "Check for available updates",
			Body: "miren server upgrade --check",
		}),
		WithExample(mflags.Example{
			Name: "Upgrade to a specific version",
			Body: "miren server upgrade --version v0.2.0",
		}),
	))
	d.Dispatch("server upgrade rollback", Infer("server upgrade rollback", "Rollback server to previous version", ServerUpgradeRollback,
		WithExample(mflags.Example{
			Name: "Rollback to the previous version",
			Body: "miren server upgrade rollback",
		}),
	))
	d.Dispatch("server docker", Section("server docker", "Docker-based server management commands", ""))
	d.Dispatch("server docker install", Infer("server docker install", "Install miren server using Docker", ServerInstallDocker,
		WithExample(mflags.Example{
			Name: "Install with cloud registration",
			Body: "miren server docker install",
		}),
		WithExample(mflags.Example{
			Name: "Install without cloud (local only)",
			Body: "miren server docker install --without-cloud",
		}),
		WithExample(mflags.Example{
			Name: "Install with a custom HTTP port",
			Body: "miren server docker install --http-port 8080",
		}),
	))
	d.Dispatch("server docker uninstall", Infer("server docker uninstall", "Uninstall miren server Docker container", ServerUninstallDocker,
		WithExample(mflags.Example{
			Name: "Uninstall the container",
			Body: "miren server docker uninstall",
		}),
		WithExample(mflags.Example{
			Name: "Uninstall and remove all data",
			Body: "miren server docker uninstall --remove-volume",
		}),
	))
	d.Dispatch("server docker status", Infer("server docker status", "Show status of miren server Docker container", ServerStatusDocker,
		WithExample(mflags.Example{
			Name: "Show status",
			Body: "miren server docker status",
		}),
		WithExample(mflags.Example{
			Name: "Follow logs",
			Body: "miren server docker status --follow",
		}),
	))

	// CLI management commands
	d.Dispatch("download", Section("download", "Download management commands", ""))
	d.Dispatch("download release", Infer("download release", "Download and extract miren release", DownloadRelease,
		WithExample(mflags.Example{
			Name: "Download the latest release",
			Body: "miren download release",
		}),
	))
	d.Dispatch("upgrade", Infer("upgrade", "Upgrade miren CLI to latest version", Upgrade,
		WithExample(mflags.Example{
			Name: "Upgrade to latest",
			Body: "miren upgrade",
		}),
		WithExample(mflags.Example{
			Name: "Check for updates without installing",
			Body: "miren upgrade --check",
		}),
		WithExample(mflags.Example{
			Name: "Upgrade to a specific version",
			Body: "miren upgrade --version v0.2.0",
		}),
	))

	// Auth commands
	d.Dispatch("auth", Section("auth", "Authentication commands", ""))
	d.Dispatch("auth generate", Infer("auth generate", "Generate authentication config file", AuthGenerate,
		WithExample(mflags.Example{
			Name: "Generate auth config",
			Body: "miren auth generate",
		}),
	))

	// Admin commands
	d.Dispatch("admin", Infer("admin", "Call an admin method on an application", Admin,
		WithDescription(adminDescription),
		WithExample(mflags.Example{
			Name: "List available admin methods",
			Body: "miren admin --list -a myapp",
		}),
		WithExample(mflags.Example{
			Name: "Call an admin method",
			Body: "miren admin health -a myapp",
		}),
		WithExample(mflags.Example{
			Name: "Call a method with JSON output",
			Body: "miren admin stats -a myapp --json",
		}),
		WithExample(mflags.Example{
			Name: "Call a method with params from a file",
			Body: "miren admin migrate -a myapp -f params.json",
		}),
	))

	// Debug commands (unstable, may change without notice)
	d.Dispatch("debug", Section("debug", "Debug and troubleshooting commands", `
Use these commands to help diagnose issues with the miren runtime.

Warning: These commands are intended for advanced users and developers. They may change or be removed without notice.

`))
	d.Dispatch("debug connection", Infer("debug connection", "Test connectivity and authentication with a server", DebugConnection))
	d.Dispatch("debug reindex", Infer("debug reindex", "Rebuild all entity indexes from scratch", DebugReindex))
	d.Dispatch("debug test", Section("debug test", "Debug test commands", ""))
	d.Dispatch("debug test load", Infer("debug test load", "Loadtest a URL", TestLoad))
	d.Dispatch("debug ctr", Infer("debug ctr", "Run ctr with miren defaults", DebugCtr))
	d.Dispatch("debug ctr nuke", Infer("debug ctr nuke", "Nuke a containerd namespace", CtrNuke))
	d.Dispatch("debug colors", Infer("debug colors", "Print some colors", Colors))
	d.Dispatch("debug bundle", Infer("debug bundle", "Create a support bundle with system debug information", DebugBundle))

	// Debug RBAC commands
	d.Dispatch("debug rbac", Infer("debug rbac", "Fetch and display RBAC rules from miren.cloud", DebugRBAC))
	d.Dispatch("debug rbac test", Infer("debug rbac test", "Test RBAC evaluation with fetched rules", DebugRBACTest))

	// Debug entity commands
	d.Dispatch("debug entity", Section("debug entity", "Entity store debug commands", "", WithSectionDescription(entitySectionDescription)))
	d.Dispatch("debug entity get", Infer("debug entity get", "Get an entity", EntityGet))
	d.Dispatch("debug entity put", Infer("debug entity put", "Put an entity", EntityPut,
		WithDescription(entityPutDescription),
	))
	d.Dispatch("debug entity delete", Infer("debug entity delete", "Delete an entity", EntityDelete))
	d.Dispatch("debug entity list", Infer("debug entity list", "List entities", EntityList))
	d.Dispatch("debug entity create", Infer("debug entity create", "Create a new entity", EntityCreate))
	d.Dispatch("debug entity replace", Infer("debug entity replace", "Replace an existing entity", EntityReplace))
	d.Dispatch("debug entity patch", Infer("debug entity patch", "Patch an existing entity", EntityPatch))
	d.Dispatch("debug entity ensure", Infer("debug entity ensure", "Ensure an entity exists", EntityEnsure))

	// Debug disk commands
	d.Dispatch("debug disk", Section("debug disk", "Disk entity debug commands", "", WithSectionDescription(diskSectionDescription)))
	d.Dispatch("debug disk create", Infer("debug disk create", "Create a disk entity for testing", DebugDiskCreate,
		WithDescription(diskCreateDescription),
	))
	d.Dispatch("debug disk list", Infer("debug disk list", "List all disk entities", DebugDiskList))
	d.Dispatch("debug disk delete", Infer("debug disk delete", "Delete a disk entity", DebugDiskDelete,
		WithDescription(diskDeleteDescription),
	))
	d.Dispatch("debug disk status", Infer("debug disk status", "Show status of a disk entity", DebugDiskStatus))
	d.Dispatch("debug disk lease", Infer("debug disk lease", "Create a disk lease for testing", DebugDiskLease))
	d.Dispatch("debug disk lease-list", Infer("debug disk lease-list", "List all disk lease entities", DebugDiskLeaseList))
	d.Dispatch("debug disk lease-release", Infer("debug disk lease-release", "Release a disk lease", DebugDiskLeaseRelease))
	d.Dispatch("debug disk lease-delete", Infer("debug disk lease-delete", "Delete a disk lease entity", DebugDiskLeaseDelete))
	d.Dispatch("debug disk lease-status", Infer("debug disk lease-status", "Show detailed status of a disk lease", DebugDiskLeaseStatus))
	d.Dispatch("debug disk mounts", Infer("debug disk mounts", "List all mounted disks from /proc/mounts", DebugDiskMounts))

	// Debug LSVD commands
	d.Dispatch("debug lsvd", Section("debug lsvd", "LSVD server debug commands", ""))
	d.Dispatch("debug lsvd info", Infer("debug lsvd info", "Show combined LSVD server volumes, mounts, and metrics", DebugLsvdInfo))
	d.Dispatch("debug lsvd volumes", Infer("debug lsvd volumes", "List volumes managed by LSVD server", DebugLsvdVolumes))
	d.Dispatch("debug lsvd mounts", Infer("debug lsvd mounts", "List mounts managed by LSVD server", DebugLsvdMounts))
	d.Dispatch("debug lsvd metrics", Infer("debug lsvd metrics", "Show LSVD reconciliation metrics", DebugLsvdMetrics))

	// Debug outboard commands
	d.Dispatch("debug outboard", Section("debug outboard", "Outboard process debug commands", ""))
	d.Dispatch("debug outboard health", Infer("debug outboard health", "Check health of an outboard process", DebugOutboardHealth))

	// Debug netdb commands
	d.Dispatch("debug netdb", Section("debug netdb", "Network database debug commands", ""))
	d.Dispatch("debug netdb list", Infer("debug netdb list", "List all IP leases from netdb", DebugNetDBList))
	d.Dispatch("debug netdb status", Infer("debug netdb status", "Show IP allocation status by subnet", DebugNetDBStatus))
	d.Dispatch("debug netdb release", Infer("debug netdb release", "Manually release IP leases", DebugNetDBRelease))
	d.Dispatch("debug netdb gc", Infer("debug netdb gc", "Find and release orphaned IP leases", DebugNetDBGC))

	// Internal commands (hidden from help, used by miren internals)
	d.Dispatch("internal", Section("internal", "Internal commands used by miren components", ""))
	d.Dispatch("internal lsvd", Infer("internal lsvd", "Run LSVD server for disk management", ServerLsvd))

	addCommands(d)
}

func HiddenCommands() []string {
	return []string{
		"internal",
		"debug",
	}
}
