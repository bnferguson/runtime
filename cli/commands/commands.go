package commands

import (
	"miren.dev/mflags"
	"miren.dev/runtime/pkg/labs"
)

func RegisterAll(d *mflags.Dispatcher) {
	// Core commands
	d.Dispatch("version", Infer("version", "Print the version", Version))
	d.Dispatch("login", Infer("login", "Authenticate with miren.cloud", Login))
	d.Dispatch("logout", Infer("logout", "Remove local authentication credentials", Logout))
	d.Dispatch("whoami", Infer("whoami", "Display information about the current authenticated user", Whoami))

	// Doctor commands
	d.Dispatch("doctor", Infer("doctor", "Diagnose miren environment and connectivity", Doctor))
	d.Dispatch("doctor config", Infer("doctor config", "Check configuration files", DoctorConfig))
	d.Dispatch("doctor server", Infer("doctor server", "Check server health and connectivity", DoctorServer))
	d.Dispatch("doctor auth", Infer("doctor auth", "Check authentication and user information", DoctorAuth))

	// App lifecycle commands
	d.Dispatch("init", Infer("init", "Initialize a new application", Init))
	d.Dispatch("deploy", Infer("deploy", "Deploy an application", Deploy))
	d.Dispatch("deploy cancel", Infer("deploy cancel", "Cancel an in-progress deployment", DeployCancel))
	d.Dispatch("rollback", Infer("rollback", "Roll back to a previous version", Rollback))
	d.Dispatch("logs", Infer("logs", "Get logs for an application", Logs))

	// App management commands
	d.Dispatch("app", Infer("app", "Get information about an application", App))
	d.Dispatch("app list", Infer("app list", "List all applications", AppList))
	d.Dispatch("app status", Infer("app status", "Show current status of an application", AppStatus))
	d.Dispatch("app history", Infer("app history", "Show deployment history for an application", AppHistory))
	d.Dispatch("app delete", Infer("app delete", "Delete an application and all its resources", AppDelete))
	d.Dispatch("app run", Infer("app run", "Open interactive shell in a new sandbox", AppRun))
	d.Dispatch("apps", Infer("apps", "List all applications (alias for 'app list')", AppList))

	// Sandbox commands
	d.Dispatch("sandbox list", Infer("sandbox list", "List sandboxes (excludes dead by default)", SandboxList))
	d.Dispatch("sandbox stop", Infer("sandbox stop", "Stop a sandbox", SandboxStop))
	d.Dispatch("sandbox delete", Infer("sandbox delete", "Delete a dead sandbox", SandboxDelete))
	d.Dispatch("sandbox exec", Infer("sandbox exec", "Open interactive shell in an existing sandbox", SandboxExec))

	// Sandbox pool commands
	d.Dispatch("sandbox-pool list", Infer("sandbox-pool list", "List all sandbox pools", SandboxPoolList))
	d.Dispatch("sandbox-pool set-desired", Infer("sandbox-pool set-desired", "Set desired instance count for a sandbox pool", SandboxPoolSetDesired))

	// Environment variable commands
	d.Dispatch("env", Section("env", "Environment variable management commands", ""))
	d.Dispatch("env set", Infer("env set", "Set environment variables for an application", EnvSet))
	d.Dispatch("env get", Infer("env get", "Get an environment variable value", EnvGet))
	d.Dispatch("env list", Infer("env list", "List all environment variables", EnvList))
	d.Dispatch("env delete", Infer("env delete", "Delete environment variables", EnvDelete))

	// Addon commands (gated behind labs flag)
	if labs.Addons() {
		d.Dispatch("addon", Section("addon", "Addon management commands", ""))
		d.Dispatch("addon list-available", Infer("addon list-available", "List available addons", AddonListAvailable))
		d.Dispatch("addon variants", Infer("addon variants", "Show variants for an addon", AddonVariants))
		d.Dispatch("addon create", Infer("addon create", "Attach an addon to an application", AddonCreate))
		d.Dispatch("addon list", Infer("addon list", "List addons attached to an application", AddonList))
		d.Dispatch("addon destroy", Infer("addon destroy", "Remove an addon from an application", AddonDestroy))
	}

	// Route commands
	d.Dispatch("route", Infer("route", "List all HTTP routes", Route))
	d.Dispatch("route list", Infer("route list", "List all HTTP routes", RouteList))
	d.Dispatch("route set", Infer("route set", "Create or update an HTTP route", RouteSet))
	d.Dispatch("route remove", Infer("route remove", "Remove an HTTP route", RouteRemove))
	d.Dispatch("route show", Infer("route show", "Show details of an HTTP route", RouteShow))
	d.Dispatch("route set-default", Infer("route set-default", "Set an app as the default route", RouteSetDefault))
	d.Dispatch("route unset-default", Infer("route unset-default", "Remove the default route", RouteUnsetDefault))

	// Route OIDC commands - behind feature flag
	if labs.RouteOIDC() {
		d.Dispatch("route oidc", Section("route oidc", "OIDC authentication management for routes", ""))
		d.Dispatch("route oidc enable", Infer("route oidc enable", "Enable OIDC authentication for a route", RouteOidcEnable))
		d.Dispatch("route oidc disable", Infer("route oidc disable", "Disable OIDC authentication for a route", RouteOidcDisable))
		d.Dispatch("route oidc show", Infer("route oidc show", "Show OIDC configuration for a route", RouteOidcShow))
	}

	// Config commands
	d.Dispatch("config", Section("config", "Configuration file management", ""))
	d.Dispatch("config info", Infer("config info", "Show configuration file locations and format", ConfigInfo))
	d.Dispatch("config load", Infer("config load", "Load config and merge it with your current config", ConfigLoad))

	// Cluster commands
	d.Dispatch("cluster", Infer("cluster", "List configured clusters", Cluster))
	d.Dispatch("cluster list", Infer("cluster list", "List all configured clusters", ClusterList))
	d.Dispatch("cluster switch", Infer("cluster switch", "Switch to a different cluster", ClusterSwitch))
	d.Dispatch("cluster add", Infer("cluster add", "Add a new cluster configuration", ClusterAdd))
	d.Dispatch("cluster remove", Infer("cluster remove", "Remove a cluster from the configuration", ClusterRemove))
	d.Dispatch("cluster current", Infer("cluster current", "Show the pinned cluster for this app", ClusterCurrent))

	// Runner commands (distributed runners) - behind feature flag
	if labs.DistributedRunners() {
		d.Dispatch("runner", Section("runner", "Runner management commands", ""))
		d.Dispatch("runner invite", Infer("runner invite", "Create a join code for a new runner", RunnerInvite))
		d.Dispatch("runner join", Infer("runner join", "Join this machine to a coordinator as a runner", RunnerJoin))
		d.Dispatch("runner start", Infer("runner start", "Start this machine as a distributed runner", RunnerStart))
		d.Dispatch("runner list", Infer("runner list", "List all registered runners", RunnerList))
		d.Dispatch("runner revoke", Infer("runner revoke", "Revoke a runner invitation", RunnerRevoke))
		d.Dispatch("runner invite list", Infer("runner invite list", "List all runner invitations", RunnerInviteList))
	}

	// Server commands
	d.Dispatch("server", Infer("server", "Start the miren server", Server))
	d.Dispatch("server config", Section("server config", "Server configuration management commands", ""))
	d.Dispatch("server config generate", Infer("server config generate", "Generate a server configuration file from current settings", ServerConfigGenerate))
	d.Dispatch("server config validate", Infer("server config validate", "Validate a server configuration file", ServerConfigValidate))
	d.Dispatch("server upgrade", Infer("server upgrade", "Upgrade miren server", ServerUpgrade))
	d.Dispatch("server upgrade rollback", Infer("server upgrade rollback", "Rollback server to previous version", ServerUpgradeRollback))
	d.Dispatch("server docker", Section("server docker", "Docker-based server management commands", ""))
	d.Dispatch("server docker install", Infer("server docker install", "Install miren server using Docker", ServerInstallDocker))
	d.Dispatch("server docker uninstall", Infer("server docker uninstall", "Uninstall miren server Docker container", ServerUninstallDocker))
	d.Dispatch("server docker status", Infer("server docker status", "Show status of miren server Docker container", ServerStatusDocker))

	// CLI management commands
	d.Dispatch("download release", Infer("download release", "Download and extract miren release", DownloadRelease))
	d.Dispatch("upgrade", Infer("upgrade", "Upgrade miren CLI to latest version", Upgrade))

	// Auth commands
	d.Dispatch("auth generate", Infer("auth generate", "Generate authentication config file", AuthGenerate))

	// Admin commansd
	d.Dispatch("admin", Infer("admin", "Call an admin method on an application", Admin))

	// Debug commands (unstable, may change without notice)
	d.Dispatch("debug", Section("debug", "Debug and troubleshooting commands", `
Use these commands to help diagnose issues with the miren runtime.

Warning: These commands are intended for advanced users and developers. They may change or be removed without notice.

`))
	d.Dispatch("debug connection", Infer("debug connection", "Test connectivity and authentication with a server", DebugConnection))
	d.Dispatch("debug reindex", Infer("debug reindex", "Rebuild all entity indexes from scratch", DebugReindex))
	d.Dispatch("debug test load", Infer("debug test load", "Loadtest a URL", TestLoad))
	d.Dispatch("debug ctr", Infer("debug ctr", "Run ctr with miren defaults", DebugCtr))
	d.Dispatch("debug ctr nuke", Infer("debug ctr nuke", "Nuke a containerd namespace", CtrNuke))
	d.Dispatch("debug colors", Infer("debug colors", "Print some colors", Colors))
	d.Dispatch("debug bundle", Infer("debug bundle", "Create a support bundle with system debug information", DebugBundle))

	// Debug RBAC commands
	d.Dispatch("debug rbac", Infer("debug rbac", "Fetch and display RBAC rules from miren.cloud", DebugRBAC))
	d.Dispatch("debug rbac test", Infer("debug rbac test", "Test RBAC evaluation with fetched rules", DebugRBACTest))

	// Debug entity commands
	d.Dispatch("debug entity get", Infer("debug entity get", "Get an entity", EntityGet))
	d.Dispatch("debug entity put", Infer("debug entity put", "Put an entity", EntityPut))
	d.Dispatch("debug entity delete", Infer("debug entity delete", "Delete an entity", EntityDelete))
	d.Dispatch("debug entity list", Infer("debug entity list", "List entities", EntityList))
	d.Dispatch("debug entity create", Infer("debug entity create", "Create a new entity", EntityCreate))
	d.Dispatch("debug entity replace", Infer("debug entity replace", "Replace an existing entity", EntityReplace))
	d.Dispatch("debug entity patch", Infer("debug entity patch", "Patch an existing entity", EntityPatch))
	d.Dispatch("debug entity ensure", Infer("debug entity ensure", "Ensure an entity exists", EntityEnsure))

	// Debug disk commands
	d.Dispatch("debug disk", Section("debug disk", "Disk entity debug commands", ""))
	d.Dispatch("debug disk create", Infer("debug disk create", "Create a disk entity for testing", DebugDiskCreate))
	d.Dispatch("debug disk list", Infer("debug disk list", "List all disk entities", DebugDiskList))
	d.Dispatch("debug disk delete", Infer("debug disk delete", "Delete a disk entity", DebugDiskDelete))
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
