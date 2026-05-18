package commands

import (
	"strings"

	"miren.dev/mflags"
)

type section struct {
	name        string
	help        string
	desc        string
	description string
	group       string
	fs          *mflags.FlagSet
}

var _ mflags.Command = &section{}

type SectionOption func(*section)

// WithSectionDescription sets an extended markdown description for the section.
func WithSectionDescription(desc string) SectionOption {
	return func(s *section) {
		s.description = desc
	}
}

// WithSectionGroup assigns this section to a named group for help rendering.
func WithSectionGroup(group string) SectionOption {
	return func(s *section) {
		s.group = group
	}
}

func Section(name, desc, help string, opts ...SectionOption) mflags.Command {
	help = strings.TrimSpace(help)

	if help == "" {
		help = desc
	}

	if desc == "" {
		desc = help
	}

	s := &section{
		name: name,
		desc: desc,
		help: help,
		fs:   mflags.NewFlagSet(name),
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

func (s *section) FlagSet() *mflags.FlagSet {
	return s.fs
}

func (s *section) Run(fs *mflags.FlagSet, args []string) error {
	// Return ErrShowHelp to signal the dispatcher to show help with sub-commands
	return mflags.ErrShowHelp
}

func (s *section) Usage() string {
	return s.desc
}

// Help returns the help text
func (s *section) Help() string {
	return s.help
}

// Synopsis returns a short description
func (s *section) Synopsis() string {
	return s.desc
}

// Description implements mflags.DescriptionProvider.
func (s *section) Description() string {
	return s.description
}

// CommandGroup implements mflags.CommandGroupProvider.
func (s *section) CommandGroup() string {
	return s.group
}
