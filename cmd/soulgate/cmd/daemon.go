package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// Paths under ~/.soulgate used by the daemon sub-system.
const (
	daemonLogName = "daemon.log"
	daemonPIDName = "daemon.pid"
)

// daemonDir returns the per-user .soulgate directory (not workspace-specific).
func daemonDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".soulgate"), nil
}

func daemonLogPath() (string, error) {
	d, err := daemonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, daemonLogName), nil
}

func daemonPIDPath() (string, error) {
	d, err := daemonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, daemonPIDName), nil
}

// daemonCmd is the top-level "daemon" command group.
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the SoulGate gateway daemon",
	Long: `Manage the SoulGate gateway as a background daemon process.

The daemon runs the gateway server in the background, writing logs to
~/.soulgate/daemon.log and its PID to ~/.soulgate/daemon.pid.

Subcommands:
  start   Start the gateway as a background daemon
  stop    Send SIGTERM to the running daemon
  status  Report whether the daemon is alive
  logs    Tail the daemon log file`,
}

// daemonStartCmd starts the gateway in the background.
var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the gateway as a background daemon",
	RunE:  runDaemonStart,
}

// daemonStopCmd sends SIGTERM to the daemon process.
var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE:  runDaemonStop,
}

// daemonStatusCmd prints whether the daemon is alive.
var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE:  runDaemonStatus,
}

// daemonLogsCmd tails the daemon log file.
var daemonLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail daemon logs",
	RunE:  runDaemonLogs,
}

var daemonLogsLines int

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonLogsCmd)

	daemonLogsCmd.Flags().IntVar(&daemonLogsLines, "lines", 50, "Number of log lines to display")
}

// runDaemonStart forks a new `soulgate gateway start` process, redirects its
// stdout/stderr to the daemon log file, and writes the PID.
func runDaemonStart(cmd *cobra.Command, args []string) error {
	// Ensure ~/.soulgate exists.
	dir, err := daemonDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create daemon directory %s: %w", dir, err)
	}

	// Check if a daemon is already running.
	if alive, pid, _ := daemonAlive(); alive {
		fmt.Printf("Daemon is already running (PID %d)\n", pid)
		return nil
	}

	logPath, err := daemonLogPath()
	if err != nil {
		return err
	}

	pidPath, err := daemonPIDPath()
	if err != nil {
		return err
	}

	// Open (or create) the log file for appending.
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open daemon log %s: %w", logPath, err)
	}
	defer logFile.Close()

	// Resolve the path to the current executable so the child is the same binary.
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Build the subprocess command. Inherit addr/port flags if they were set
	// on the parent "gateway start" invocation; otherwise use defaults.
	childArgs := []string{"gateway", "start"}
	if gatewayAddress != "0.0.0.0" {
		childArgs = append(childArgs, "--address", gatewayAddress)
	}
	if gatewayPort != 8080 {
		childArgs = append(childArgs, "--port", strconv.Itoa(gatewayPort))
	}

	child := exec.Command(self, childArgs...)
	child.Stdout = logFile
	child.Stderr = logFile
	// Detach from the current process group so the daemon keeps running after
	// the parent exits.
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := child.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Write PID file.
	pidData := []byte(strconv.Itoa(child.Process.Pid) + "\n")
	if err := os.WriteFile(pidPath, pidData, 0o644); err != nil {
		// Non-fatal: process is started, just log the warning.
		fmt.Fprintf(os.Stderr, "Warning: failed to write PID file %s: %v\n", pidPath, err)
	}

	fmt.Printf("Daemon started (PID %d)\n", child.Process.Pid)
	fmt.Printf("  Log:  %s\n", logPath)
	fmt.Printf("  PID:  %s\n", pidPath)
	return nil
}

// runDaemonStop reads the PID file and sends SIGTERM to the daemon process.
func runDaemonStop(cmd *cobra.Command, args []string) error {
	alive, pid, err := daemonAlive()
	if err != nil {
		return fmt.Errorf("could not read daemon PID: %w", err)
	}
	if !alive {
		fmt.Println("Daemon is not running")
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("could not find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to PID %d: %w", pid, err)
	}

	// Remove the stale PID file.
	pidPath, _ := daemonPIDPath()
	os.Remove(pidPath) //nolint:errcheck

	fmt.Printf("Sent SIGTERM to daemon (PID %d)\n", pid)
	return nil
}

// runDaemonStatus reports whether the daemon process is alive.
func runDaemonStatus(cmd *cobra.Command, args []string) error {
	logPath, _ := daemonLogPath()
	pidPath, _ := daemonPIDPath()

	alive, pid, err := daemonAlive()
	if err != nil {
		fmt.Printf("Status:  stopped (no PID file or unreadable)\n")
		fmt.Printf("PID file: %s\n", pidPath)
		fmt.Printf("Log:      %s\n", logPath)
		return nil
	}

	if alive {
		fmt.Printf("Status:   running\n")
		fmt.Printf("PID:      %d\n", pid)
	} else {
		fmt.Printf("Status:   stopped (stale PID %d)\n", pid)
	}
	fmt.Printf("PID file: %s\n", pidPath)
	fmt.Printf("Log:      %s\n", logPath)
	return nil
}

// runDaemonLogs prints the last N lines of the daemon log.
func runDaemonLogs(cmd *cobra.Command, args []string) error {
	logPath, err := daemonLogPath()
	if err != nil {
		return err
	}

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No daemon log found. Has the daemon been started?")
			return nil
		}
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	lines, err := tailLines(f, daemonLogsLines)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

// daemonAlive reads the PID file and checks whether the process is still alive.
// Returns (alive, pid, error).  If the PID file does not exist, alive=false and
// err=nil.
func daemonAlive() (alive bool, pid int, err error) {
	pidPath, err := daemonPIDPath()
	if err != nil {
		return false, 0, err
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err = strconv.Atoi(pidStr)
	if err != nil {
		return false, 0, fmt.Errorf("invalid PID in file: %q", pidStr)
	}

	// Signal 0 checks process existence without sending a real signal.
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, nil
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// ESRCH means no such process; EPERM means process exists but owned by another user.
		if err == syscall.ESRCH {
			return false, pid, nil
		}
		// EPERM → process exists but we cannot signal it; treat as alive.
	}

	return true, pid, nil
}

// tailLines returns the last n lines from r by reading the entire file.
// For daemon log files this is acceptable (logs are not expected to be enormous).
func tailLines(r io.Reader, n int) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var all []string
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if n <= 0 || n >= len(all) {
		return all, nil
	}
	return all[len(all)-n:], nil
}
