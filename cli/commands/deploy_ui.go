package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"miren.dev/runtime/pkg/color"
	"miren.dev/runtime/pkg/colortheory"
	"miren.dev/runtime/pkg/progress/progressui"
	"miren.dev/runtime/pkg/progress/upload"
)

// UI Styles
var (
	liveFaint         lipgloss.Style
	deployPrefixStyle = lipgloss.NewStyle().Faint(true)
	phaseSummaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green for completed phases
	phaseTimeStyle    = lipgloss.NewStyle().Faint(true)
)

func init() {
	lf := color.LiveFaint()
	if lf == "" {
		liveFaint = lipgloss.NewStyle().Faint(true)
	} else {
		liveFaint = lipgloss.NewStyle().Foreground(lipgloss.Color(lf))
	}
}

// Custom spinner styles
var (
	mirenBlue = "#3E53FB"
	lightBlue = colortheory.ChangeLightness(mirenBlue, -10)

	spinStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#3E53FB",
		Dark:  lightBlue,
	})
)

func line(str string) string {
	var sb strings.Builder

	sb.WriteString(spinStyle.Render(str))

	return sb.String()
}

// Meter is the custom spinner animation
var Meter = spinner.Spinner{
	Frames: []string{
		line("⠋"),
		line("⠙"),
		line("⠹"),
		line("⠸"),
		line("⠼"),
		line("⠴"),
		line("⠦"),
		line("⠧"),
		line("⠇"),
		line("⠏"),
	},
	FPS: time.Second / 10, //nolint:mnd
}

// Data types for UI
type buildProgress struct {
	total     int
	completed int
}

type phaseSummary struct {
	name     string
	duration time.Duration
	details  string
}

// TEA message types
type updateMsg struct {
	msg string
}

type timeoutCheckMsg struct{}

// deployInfo is the TEA model for the deploy UI
type deployInfo struct {
	spinner spinner.Model

	message string
	update  chan string

	buildCh    chan buildProgress
	prog       progress.Model
	buildSteps int
	buildPct   float64

	uploadProgress chan upload.Progress
	uploadSpin     spinner.Model
	uploadSpeed    string
	isUploading    bool

	// Phase tracking
	phaseStart       time.Time
	currentPhase     string
	completedPhases  []phaseSummary
	uploadBytes      int64
	uploadDuration   time.Duration
	finalUploadSpeed float64

	// Source cache info
	cachedFiles int32
	totalFiles  int

	// Timeout and interrupt handling
	lastActivity    time.Time
	buildkitTimeout time.Duration
	buildkitStarted bool // Track if buildkit has shown any activity
	interrupted     bool

	showProgress bool
	bp           tea.Model
}

func initialModel(update chan string, buildCh chan buildProgress, uploadProgress chan upload.Progress, cachedFiles int32, totalFiles int) *deployInfo {
	s := spinner.New()
	s.Spinner = Meter
	s.Style = lipgloss.NewStyle()

	p := progress.New(progress.WithWidth(20), progress.WithGradient(
		colortheory.ChangeLightness("#3E53FB", -10),
		colortheory.ChangeLightness("#3E53FB", 20),
	))

	uploadS := spinner.New()
	uploadS.Spinner = Meter
	uploadS.Style = lipgloss.NewStyle()

	return &deployInfo{
		spinner:         s,
		message:         "Reading application data",
		update:          update,
		buildCh:         buildCh,
		prog:            p,
		uploadProgress:  uploadProgress,
		uploadSpin:      uploadS,
		isUploading:     true,
		uploadSpeed:     "calculating...",
		phaseStart:      time.Now(),
		currentPhase:    "upload",
		cachedFiles:     cachedFiles,
		totalFiles:      totalFiles,
		lastActivity:    time.Now(),
		buildkitTimeout: 60 * time.Second, // 60 second timeout for buildkit to start
		buildkitStarted: false,
		bp:              progressui.TeaModel(),
	}
}

func (m *deployInfo) uploadDetails() string {
	var parts []string
	if m.cachedFiles > 0 {
		parts = append(parts, fmt.Sprintf("reused %d/%d files", m.cachedFiles, m.totalFiles))
	}
	if m.uploadBytes > 0 {
		parts = append(parts, fmt.Sprintf("%s at %s",
			upload.FormatBytes(m.uploadBytes),
			upload.FormatSpeed(m.finalUploadSpeed)))
	}
	return strings.Join(parts, ", ")
}

func (m *deployInfo) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.bp.Init(),
		m.spinner.Tick,
		m.uploadSpin.Tick,
		m.checkTimeout(), // Start timeout monitoring
	}

	// Only wait for channels that are expected to have data
	if m.update != nil {
		cmds = append(cmds, func() tea.Msg {
			return updateMsg{msg: <-m.update}
		})
	}

	if m.buildCh != nil {
		cmds = append(cmds, func() tea.Msg {
			return <-m.buildCh
		})
	}

	// Start listening for upload progress
	if m.uploadProgress != nil {
		cmds = append(cmds, func() tea.Msg {
			return <-m.uploadProgress
		})
	}

	return tea.Batch(cmds...)
}

func (m *deployInfo) checkTimeout() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return timeoutCheckMsg{}
	})
}

func (m *deployInfo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.bp, cmd = m.bp.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.uploadSpin, cmd = m.uploadSpin.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.interrupted = true
			return m, tea.Quit
		case tea.KeyEsc:
			m.showProgress = false
		case tea.KeyEnter:
			m.showProgress = !m.showProgress
		}
	case timeoutCheckMsg:
		// Only check for timeout if buildkit hasn't started yet
		if m.currentPhase == "buildkit" && !m.buildkitStarted && time.Since(m.lastActivity) > m.buildkitTimeout {
			m.currentPhase = "timeout"
			return m, tea.Sequence(
				tea.Println("\n\n❌ Buildkit failed to start after 60 seconds. This may indicate a server issue."),
				tea.Quit,
			)
		}
		// Continue checking
		cmds = append(cmds, m.checkTimeout())
	case updateMsg:
		prevMessage := m.message
		m.message = msg.msg
		m.lastActivity = time.Now() // Reset activity timer

		// Track phase transitions
		if prevMessage != msg.msg {
			// Complete the upload phase
			if m.isUploading && msg.msg == "Launching builder" {
				m.isUploading = false
				duration := time.Since(m.phaseStart)
				m.uploadDuration = duration

				m.completedPhases = append(m.completedPhases, phaseSummary{
					name:     "Upload artifacts",
					duration: duration,
					details:  m.uploadDetails(),
				})

				// Start tracking buildkit phase
				m.phaseStart = time.Now()
				m.currentPhase = "buildkit"
				m.buildkitStarted = true
			}

		}

		cmds = append(cmds, func() tea.Msg {
			return updateMsg{msg: <-m.update}
		})
	case upload.Progress:
		m.lastActivity = time.Now() // Reset activity timer
		if m.isUploading {
			m.uploadSpeed = upload.FormatSpeed(msg.BytesPerSecond)
			m.uploadBytes = msg.BytesRead
			m.finalUploadSpeed = msg.BytesPerSecond

			// Spinner tick is already handled in the Update method
			// Continue reading upload progress
			if m.uploadProgress != nil {
				cmds = append(cmds, func() tea.Msg {
					return <-m.uploadProgress
				})
			}
		}
	case buildProgress:
		m.lastActivity = time.Now() // Reset activity timer

		// Build progress means upload is complete (fallback if no "Launching builder" message)
		if m.isUploading {
			m.isUploading = false
			duration := time.Since(m.phaseStart)
			m.uploadDuration = duration

			// Only create upload summary if we haven't already from "Launching builder" message
			hasUploadPhase := false
			for _, phase := range m.completedPhases {
				if phase.name == "Upload artifacts" {
					hasUploadPhase = true
					break
				}
			}

			if !hasUploadPhase {
				m.completedPhases = append(m.completedPhases, phaseSummary{
					name:     "Upload artifacts",
					duration: duration,
					details:  m.uploadDetails(),
				})
			}

			// Always start tracking buildkit phase when transitioning
			m.phaseStart = time.Now()
			m.currentPhase = "buildkit"
		}
		m.buildkitStarted = true

		m.buildSteps = msg.total

		if msg.total > 0 {
			m.buildPct = float64(msg.completed) / float64(msg.total)
		}
		cmds = append(cmds, func() tea.Msg {
			return <-m.buildCh
		})
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *deployInfo) View() string {
	var lines []string

	// Show completed phase summaries
	for _, phase := range m.completedPhases {
		phaseStr := fmt.Sprintf("  ✓ %s", phaseSummaryStyle.Render(phase.name))
		timeStr := phaseTimeStyle.Render(fmt.Sprintf("(%s)", formatPhaseDuration(phase.duration)))

		if phase.details != "" {
			lines = append(lines, fmt.Sprintf("%s %s - %s", phaseStr, timeStr, phase.details))
		} else {
			lines = append(lines, fmt.Sprintf("%s %s", phaseStr, timeStr))
		}
	}

	// Show current progress
	var currentLine string
	if m.isUploading {
		bytesInfo := upload.FormatBytes(m.uploadBytes)
		speedInfo := deployPrefixStyle.Render(fmt.Sprintf("%s uploaded at %s", bytesInfo, m.uploadSpeed))
		currentLine = fmt.Sprintf("  %s %s...\n      %s %s",
			m.uploadSpin.View(), m.message, m.spinner.View(), speedInfo)
	} else if m.currentPhase != "completed" {
		if m.buildSteps > 0 {
			steps := deployPrefixStyle.Render(fmt.Sprintf("Building %d steps:", m.buildSteps))
			currentLine = fmt.Sprintf("  %s %s...\n      %s %s", m.spinner.View(), m.message, steps, m.prog.ViewAs(m.buildPct))
		} else {
			currentLine = fmt.Sprintf("  %s %s...", m.spinner.View(), m.message)
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	// Join all lines
	str := strings.Join(lines, "\n")

	if !m.showProgress {
		return lipgloss.JoinVertical(lipgloss.Top,
			str,
			liveFaint.Render("      [enter: explain]"),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		str,
		m.bp.View(),
		liveFaint.Render("      [enter: hide explain]"),
	)
}

func formatPhaseDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}

// Helper function to render a phase summary line
func renderPhaseSummary(phase phaseSummary) string {
	phaseStr := fmt.Sprintf("  ✓ %s", phaseSummaryStyle.Render(phase.name))
	timeStr := phaseTimeStyle.Render(fmt.Sprintf("(%s)", formatPhaseDuration(phase.duration)))
	if phase.details != "" {
		return fmt.Sprintf("%s %s - %s", phaseStr, timeStr, phase.details)
	}
	return fmt.Sprintf("%s %s", phaseStr, timeStr)
}
