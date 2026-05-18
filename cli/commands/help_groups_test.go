package commands

import (
	"testing"

	"miren.dev/mflags"
	"miren.dev/runtime/pkg/labs"
)

// TestAllTopLevelCommandsHaveKnownGroup fails if any top-level command is
// missing a CommandGroup or has a CommandGroup not in HelpGroupOrder (or the
// Hidden group). This prevents:
//   - Forgetting to tag a new command with WithGroup/WithSectionGroup.
//   - Typo'd group strings that would silently create a phantom group.
//   - Implicit namespaces at the top level, which cannot carry group metadata
//     and should be made explicit with a Section registration.
func TestAllTopLevelCommandsHaveKnownGroup(t *testing.T) {
	labs.EnableAll()

	d := mflags.NewDispatcher("miren")
	RegisterAll(d)

	known := make(map[string]bool)
	for _, g := range HelpGroupOrder {
		known[g] = true
	}
	known[GroupHidden] = true

	for _, child := range d.GetDirectChildren("") {
		if !child.IsEntry {
			t.Errorf("%q is an implicit namespace — register a parent Section so it can be grouped", child.Name)
			continue
		}
		if child.Group == "" {
			t.Errorf("%q has no CommandGroup — tag it with WithGroup/WithSectionGroup", child.Name)
			continue
		}
		if !known[child.Group] {
			t.Errorf("%q has group %q which is not in HelpGroupOrder (or GroupHidden)", child.Name, child.Group)
		}
	}
}
