//go:build linux

package launcher

import (
	"os/exec"
	"testing"
	"time"
	"yukihub/internal/utils/processutils"
)

// setLinuxExitWatchTimings shrinks the watch windows so tests can observe
// start-up grace and missing grace behaviour in milliseconds.
func setLinuxExitWatchTimings(t *testing.T, checkInterval, startupGrace, missingGrace time.Duration) {
	t.Helper()
	previousInterval := linuxExitWatchCheckInterval
	previousStartup := linuxExitWatchStartupGrace
	previousMissing := linuxExitWatchMissingGrace
	linuxExitWatchCheckInterval = checkInterval
	linuxExitWatchStartupGrace = startupGrace
	linuxExitWatchMissingGrace = missingGrace
	t.Cleanup(func() {
		linuxExitWatchCheckInterval = previousInterval
		linuxExitWatchStartupGrace = previousStartup
		linuxExitWatchMissingGrace = previousMissing
	})
}

func startLinuxExitWatchTestProcess(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", "exec sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start test process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd
}

// TestLinuxExitWatchKeepsSessionWhenPathsDoNotMatch is the regression test for
// Proton games ending their session after the start-up grace: the monitored PID
// was still alive, but its Linux paths did not match the configured install
// directory.
func TestLinuxExitWatchKeepsSessionWhenPathsDoNotMatch(t *testing.T) {
	setLinuxExitWatchTimings(t, 50*time.Millisecond, 150*time.Millisecond, 100*time.Millisecond)

	cmd := startLinuxExitWatchTestProcess(t)
	done := make(chan struct{})
	defer close(done)

	watch, ok := StartExitWatch(ExitWatchInput{
		RootPID:     uint32(cmd.Process.Pid),
		ProcessName: "Aokana.exe",
		SessionID:   "linux-exit-watch-regression",
		Config: ExitWatch{
			Mode: ExitWatchGameProcessPresence,
			// Intentionally unrelated: the Wine-visible paths of the game
			// process can never be resolved from this directory.
			DetectionDir: t.TempDir(),
		},
		Done: done,
	}, nil)
	if !ok {
		t.Fatal("expected Linux exit watch to start for game process presence mode")
	}

	// Longer than start-up grace + missing grace + one check interval, so the
	// old directory-based logic would have ended the session here.
	select {
	case <-watch:
		t.Fatal("exit watch ended the session while the monitored game process was still running")
	case <-time.After(600 * time.Millisecond):
	}
}

func TestLinuxExitWatchEndsSessionWhenGameProcessExits(t *testing.T) {
	setLinuxExitWatchTimings(t, 50*time.Millisecond, 150*time.Millisecond, 100*time.Millisecond)

	cmd := exec.Command("/bin/sh", "-c", "exec sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start test process: %v", err)
	}
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	done := make(chan struct{})
	defer close(done)

	watch, ok := StartExitWatch(ExitWatchInput{
		RootPID:     uint32(cmd.Process.Pid),
		ProcessName: "Game.exe",
		SessionID:   "linux-exit-watch-exit",
		Config: ExitWatch{
			Mode:         ExitWatchGameProcessPresence,
			DetectionDir: t.TempDir(),
		},
		Done: done,
	}, nil)
	if !ok {
		t.Fatal("expected Linux exit watch to start for game process presence mode")
	}

	select {
	case <-watch:
		t.Fatal("exit watch ended the session before the monitored process exited")
	case <-time.After(200 * time.Millisecond):
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill test process: %v", err)
	}
	_ = cmd.Wait()
	reaped = true

	select {
	case <-watch:
	case <-time.After(3 * time.Second):
		t.Fatal("exit watch did not end the session after the game process exited")
	}
}

func TestLinuxExitWatchGameProcessesCountsMonitoredRoot(t *testing.T) {
	tracked := []processutils.ProcessInfo{{Name: "Aokana.exe", PID: 4242}}
	processes := linuxExitWatchGameProcesses(4242, ExitWatch{
		Mode:         ExitWatchGameProcessPresence,
		DetectionDir: "/steam/steamapps/common/aokana",
	}, tracked)
	if len(processes) != 1 || processes[0].PID != 4242 {
		t.Fatalf("expected the monitored root to count as the running game, got %#v", processes)
	}
}

func TestLinuxExitWatchGameProcessesHonorsIgnoreRootProcess(t *testing.T) {
	ignored := ExitWatch{Mode: ExitWatchGameProcessPresence, IgnoreRootProcess: true}

	processes := linuxExitWatchGameProcesses(4242, ignored, []processutils.ProcessInfo{
		{Name: "Aokana.exe", PID: 4242},
		{Name: "wine64-preloader", PID: 4243},
	})
	if len(processes) != 0 {
		t.Fatalf("expected the ignored root and helper descendants to be filtered, got %#v", processes)
	}

	processes = linuxExitWatchGameProcesses(4242, ignored, []processutils.ProcessInfo{
		{Name: "Aokana.exe", PID: 4242},
		{Name: "Aokana.exe", PID: 5300},
	})
	if len(processes) != 1 || processes[0].PID != 5300 {
		t.Fatalf("expected the real game descendant to keep the session alive, got %#v", processes)
	}
}
