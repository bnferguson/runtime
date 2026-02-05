package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ValueType represents the type of a value for styling purposes
type ValueType int

const (
	ValueTypeString ValueType = iota
	ValueTypeNumber
	ValueTypeBool
	ValueTypeNull
	ValueTypeOther
)

// NamedValue represents a label-value pair for display
type NamedValue struct {
	Label     string
	Value     string
	ValueType ValueType
}

// NewNamedValue creates a NamedValue with automatic type detection
func NewNamedValue(label string, value any) NamedValue {
	nv := NamedValue{Label: label}

	switch v := value.(type) {
	case nil:
		nv.Value = "-"
		nv.ValueType = ValueTypeNull
	case bool:
		if v {
			nv.Value = "yes"
		} else {
			nv.Value = "no"
		}
		nv.ValueType = ValueTypeBool
	case float64:
		switch {
		case math.IsNaN(v):
			nv.Value = "NaN"
		case math.IsInf(v, 1):
			nv.Value = "+Inf"
		case math.IsInf(v, -1):
			nv.Value = "-Inf"
		case v == math.Trunc(v) && v >= math.MinInt64 && v <= math.MaxInt64:
			nv.Value = fmt.Sprintf("%.0f", v)
		default:
			nv.Value = fmt.Sprintf("%g", v)
		}
		nv.ValueType = ValueTypeNumber
	case int, int64, int32, float32:
		nv.Value = fmt.Sprintf("%v", v)
		nv.ValueType = ValueTypeNumber
	case string:
		nv.Value = v
		nv.ValueType = ValueTypeString
	default:
		nv.Value = fmt.Sprintf("%v", v)
		nv.ValueType = ValueTypeOther
	}

	return nv
}

// NamedValueList renders a list of named values with right-aligned labels
type NamedValueList struct {
	items  []NamedValue
	styles NamedValueStyles
}

// NamedValueStyles contains the styling configuration for named values
type NamedValueStyles struct {
	Label       lipgloss.Style
	Separator   string
	StringValue lipgloss.Style
	NumberValue lipgloss.Style
	BoolValue   lipgloss.Style
	NullValue   lipgloss.Style
	OtherValue  lipgloss.Style
}

// DefaultNamedValueStyles returns the default styling for named values
func DefaultNamedValueStyles() NamedValueStyles {
	return NamedValueStyles{
		Label:       lipgloss.NewStyle(),
		Separator:   ": ",
		StringValue: lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // Green
		NumberValue: lipgloss.NewStyle().Foreground(lipgloss.Color("14")), // Cyan
		BoolValue:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // Yellow
		NullValue:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),  // Gray
		OtherValue:  lipgloss.NewStyle(),
	}
}

// NamedValueOption is a function that configures a NamedValueList
type NamedValueOption func(*NamedValueList)

// WithNamedValueStyles sets custom styles for the named value list
func WithNamedValueStyles(styles NamedValueStyles) NamedValueOption {
	return func(n *NamedValueList) {
		n.styles = styles
	}
}

// NewNamedValueList creates a new named value list
func NewNamedValueList(items []NamedValue, opts ...NamedValueOption) *NamedValueList {
	n := &NamedValueList{
		items:  items,
		styles: DefaultNamedValueStyles(),
	}

	for _, opt := range opts {
		opt(n)
	}

	return n
}

// Render generates the string representation of the named value list
func (n *NamedValueList) Render() string {
	if len(n.items) == 0 {
		return ""
	}

	// Find the maximum label display width (handles multi-byte runes and ANSI)
	maxLabelWidth := 0
	for _, item := range n.items {
		if w := lipgloss.Width(item.Label); w > maxLabelWidth {
			maxLabelWidth = w
		}
	}

	var lines []string
	for _, item := range n.items {
		// Right-align the label by padding on the left
		paddedLabel := padLeft(item.Label, maxLabelWidth)
		styledLabel := n.styles.Label.Render(paddedLabel)
		styledValue := n.styleValue(item)
		lines = append(lines, styledLabel+n.styles.Separator+styledValue)
	}

	return strings.Join(lines, "\n")
}

// styleValue applies the appropriate style based on value type
func (n *NamedValueList) styleValue(item NamedValue) string {
	switch item.ValueType {
	case ValueTypeString:
		return n.styles.StringValue.Render(item.Value)
	case ValueTypeNumber:
		return n.styles.NumberValue.Render(item.Value)
	case ValueTypeBool:
		return n.styles.BoolValue.Render(item.Value)
	case ValueTypeNull:
		return n.styles.NullValue.Render(item.Value)
	default:
		return n.styles.OtherValue.Render(item.Value)
	}
}

// padLeft pads a string on the left to the specified display width
func padLeft(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}
