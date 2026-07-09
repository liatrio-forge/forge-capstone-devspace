package devspace

// interactive.go provides the interactive layer for setup confirmations
// (huh) and progress spinners (bubbles) used only when both stdin and stdout
// are real terminals. Every interactive path has a plain, script-safe
// fallback: confirmations fall back to the original bufio-based confirmSetup
// (byte-for-byte, so piped/CI flows are unaffected), and spinners fall back
// to a plain start/finish line pair.

import (
	"errors"
	"fmt"
	"io"
	"os"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"golang.org/x/term"
)

// isTerminalWriter reports whether w is a real terminal file descriptor.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// isTerminalReader reports whether r is a real terminal file descriptor.
func isTerminalReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// isInteractiveTerminal reports whether both in and out are real terminals,
// the condition under which huh forms and spinners render instead of falling
// back to plain, script-safe behavior.
func isInteractiveTerminal(in io.Reader, out io.Writer) bool {
	return isTerminalReader(in) && isTerminalWriter(out)
}

// confirmSetupRun asks the user to confirm running a single project's setup
// command. On a real terminal it uses a huh Confirm defaulting to "No"; when
// piped (or under --yes upstream, which skips this call entirely) it falls
// back to the original typed-phrase confirmSetup for byte-for-byte
// compatibility with existing scripts.
func confirmSetupRun(in io.Reader, out io.Writer, project, command, path string) error {
	if !isInteractiveTerminal(in, out) {
		return confirmSetup(in, out, fmt.Sprintf("Type %s to run `%s` in %s: ", project, command, path), project)
	}
	var confirmed bool
	// Deliberately pass out directly, not styledWriter(out): huh (via
	// bubbletea v2) already detects and downsamples color for its own
	// interactive renderer, and wrapping its output in a second
	// colorprofile.Writer corrupts the cursor-control escape sequences the
	// renderer depends on.
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Run `%s` in %s?", command, path)).
			Affirmative("Yes").
			Negative("No").
			Value(&confirmed),
	)).WithShowHelp(false).WithInput(in).WithOutput(out)
	if err := form.Run(); err != nil {
		// Keep the "nothing was run" message consistent with the
		// non-interactive path (e.g. the user pressing Esc/Ctrl-C to abort
		// the form) rather than surfacing huh's library-internal error text.
		return errors.New("confirmation did not match; no setup commands were run")
	}
	if !confirmed {
		return errors.New("confirmation did not match; no setup commands were run")
	}
	return nil
}

// confirmSetupApply asks the user to type expected to confirm running setup
// commands for every detected project. On a real terminal it uses a huh
// Input with the same exact-phrase validation the plain fallback enforces;
// when piped it falls back to the original confirmSetup.
func confirmSetupApply(in io.Reader, out io.Writer, prompt, expected string) error {
	if !isInteractiveTerminal(in, out) {
		return confirmSetup(in, out, prompt, expected)
	}
	var answer string
	// See the comment in confirmSetupRun: pass out directly, never
	// styledWriter(out), for an interactive huh/bubbletea program.
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title(prompt).
			Value(&answer).
			Validate(func(s string) error {
				if s != expected {
					return errors.New("confirmation did not match")
				}
				return nil
			}),
	)).WithShowHelp(false).WithInput(in).WithOutput(out)
	if err := form.Run(); err != nil {
		// See confirmSetupRun: keep the abort message consistent with the
		// non-interactive path instead of surfacing huh's raw error.
		return errors.New("confirmation did not match; no setup commands were run")
	}
	return nil
}

// spinnerModel is a minimal bubbletea program that shows a spinner next to a
// label until an external workDoneMsg arrives. The work result itself is
// reported to runSpinnerProgram's caller via a channel, not through this
// model, since the model's final state after program.Run() is only reliable
// when the program quit normally (via workDoneMsg), not when it exits early
// through an intercepted SIGINT/SIGTERM.
type spinnerModel struct {
	spinner  spinner.Model
	label    string
	quitting bool
}

type workDoneMsg struct{ err error }

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case workDoneMsg:
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m spinnerModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	return tea.NewView(m.spinner.View() + " " + m.label)
}

// runWithSpinner runs work while showing a spinner next to label on out when
// out is a real terminal, or prints a plain start line and runs work
// synchronously otherwise -- no TUI program is started in the
// non-interactive case. It only manages the in-progress indicator; the
// caller is responsible for printing its own completion message once
// runWithSpinner returns successfully.
//
// The gate checks stdin as well as out: since this call never passes
// tea.WithInput, bubbletea defaults to os.Stdin and, if that isn't a
// terminal, falls back to opening /dev/tty directly -- which hard-errors in
// an environment with no controlling terminal at all (some headless CI
// runners, certain container configurations). Checking stdin here avoids
// that failure mode by taking the plain path instead, the same symmetric
// check confirmSetupRun/confirmSetupApply already use.
func runWithSpinner(out io.Writer, label string, work func() error) error {
	if !isTerminalWriter(out) || !isTerminalReader(os.Stdin) {
		printLine(out, "%s...", label)
		return work()
	}

	// As above: bubbletea v2 handles its own color downsampling, so out is
	// passed directly rather than through styledWriter.
	m := spinnerModel{spinner: spinner.New(spinner.WithSpinner(spinner.Dot)), label: label}
	program := tea.NewProgram(m, tea.WithOutput(out))
	return runSpinnerProgram(program, work)
}

// runSpinnerProgram drives program while work runs in the background, and
// always waits for work to actually finish before returning -- even if the
// program itself exits early.
//
// bubbletea installs its own SIGINT/SIGTERM handler and turns those signals
// into a graceful InterruptMsg (program.Run returning ErrInterrupted)
// instead of letting the OS terminate the process outright, which is what
// happened before this interactive layer existed. Callers of runWithSpinner
// (project update, scan) run inside withAppLock, whose cross-process file
// lock must stay held for the entire mutating operation; work has no
// cancellation path of its own (UpdateProjects/ScanWorkspace run to
// completion once started), so returning as soon as the TUI quits would
// release the lock while work is still writing manifest/state files. Always
// waiting for the done channel here, regardless of why program.Run
// returned, keeps the lock held for work's true duration.
//
// The goroutine also recovers from a panic in work so an arbitrary caller
// closure (git clone, manifest hydration) can't crash the process before
// bubbletea's own terminal-restore deferred calls (which live on the
// program.Run goroutine, not this one) get a chance to run.
func runSpinnerProgram(program *tea.Program, work func() error) error {
	done := make(chan error, 1)
	go func() {
		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			return work()
		}()
		done <- err
		program.Send(workDoneMsg{err: err})
	}()

	_, runErr := program.Run()
	if workErr := <-done; workErr != nil {
		return workErr
	}
	return runErr
}
