package commands

// Help group constants — used both as the CommandGroup value on registered
// commands and as the display label in help output.
//
// Every top-level command should be tagged with one of these groups. The
// TestAllTopLevelCommandsHaveKnownGroup test catches drift.
const (
	GroupGettingStarted = "Getting started"
	GroupMonitoring     = "Monitoring your app"
	GroupConfiguring    = "Configuring your app"
	GroupClient         = "Client operations"
	GroupServer         = "Server operations"

	// GroupHidden commands are registered but filtered from help output.
	GroupHidden = "Hidden"
)

// HelpGroupOrder controls the order groups are rendered in top-level help.
// GroupHidden is intentionally absent — commands in that group are filtered.
var HelpGroupOrder = []string{
	GroupGettingStarted,
	GroupMonitoring,
	GroupConfiguring,
	GroupClient,
	GroupServer,
}
