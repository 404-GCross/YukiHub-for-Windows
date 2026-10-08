//go:build linux

package launcher

import (
	"time"
	"yukihub/internal/utils/processutils"
)

// Timings are variables so tests can exercise the watch without waiting for
// the production grace windows.
var (
	linuxExitWatchCheckInterval = 2 * time.Second
	linuxExitWatchStartupGrace  = 2 * time.Minute
	linuxExitWatchMissingGrace  = 8 * time.Second
)

// StartExitWatch monitors the Linux process group associated with a game
// session and signals when no tracked game process remains.
func StartExitWatch(input ExitWatchInput, logger DetectionLogger) (<-chan struct{}, bool) {
	if input.Config.Mode != ExitWatchGameProcessPresence {
		return nil, false
	}

	result := make(chan struct{})
	go runLinuxExitWatch(input, logger, result)
	return result, true
}

func runLinuxExitWatch(input ExitWatchInput, logger DetectionLogger, result chan<- struct{}) {
	triggered := false
	defer func() {
		if triggered {
			close(result)
		}
	}()

	ticker := time.NewTicker(linuxExitWatchCheckInterval)
	defer ticker.Stop()

	startedAt := time.Now()
	var missingSince time.Time
	observedGameProcess := false
	processTracker := processutils.NewLinuxProcessTracker(input.RootPID)
	if snapshot, err := processutils.CaptureLinuxProcessSnapshot(); err == nil {
		processTracker.Observe(snapshot)
	}

	for {
		select {
		case <-input.Done:
			return
		case <-ticker.C:
			snapshot, err := processutils.CaptureLinuxProcessSnapshot()
			if err != nil {
				continue
			}
			// Exit decisions are identity-based only: tracked PIDs are validated
			// against their start time, so PID reuse and reparenting cannot keep
			// a dead game alive. Install-dir matching is deliberately not used
			// here because Proton/Wine games expose Windows-style paths that
			// never match the Linux install directory.
			tracked := processTracker.Observe(snapshot)
			if len(linuxExitWatchGameProcesses(input.RootPID, input.Config, tracked)) > 0 {
				observedGameProcess = true
				missingSince = time.Time{}
				continue
			}

			if !processTracker.RootPresent(snapshot) {
				logLinuxExitWatchRootGone(logger, input, processTracker, snapshot)
				triggered = true
				return
			}

			if !observedGameProcess && time.Since(startedAt) < linuxExitWatchStartupGrace {
				continue
			}
			if missingSince.IsZero() {
				missingSince = time.Now()
				continue
			}
			if time.Since(missingSince) < linuxExitWatchMissingGrace {
				continue
			}

			logInfo(
				logger,
				"Linux exit watch: no tracked game process remains for %s (root PID %d); ending session %s",
				input.ProcessName,
				input.RootPID,
				input.SessionID,
			)
			triggered = true
			return
		}
	}
}

// logLinuxExitWatchRootGone distinguishes a genuine process exit from PID
// reuse so the recorded runtime reason stays diagnosable.
func logLinuxExitWatchRootGone(logger DetectionLogger, input ExitWatchInput, processTracker *processutils.LinuxProcessTracker, snapshot *processutils.LinuxProcessSnapshot) {
	rootTicks, observed := processTracker.RootStartTicks()
	if !observed {
		logInfo(logger, "Linux exit watch: process %s (PID %d) exited before it could be observed; ending session %s", input.ProcessName, input.RootPID, input.SessionID)
		return
	}
	if ticks, ok := snapshot.ProcessStartTicks(input.RootPID); ok && ticks != rootTicks {
		logInfo(logger, "Linux exit watch: PID %d was reused by another process; ending session %s", input.RootPID, input.SessionID)
		return
	}
	logInfo(logger, "Linux exit watch: process %s (PID %d) exited; ending session %s", input.ProcessName, input.RootPID, input.SessionID)
}

// linuxExitWatchGameProcesses keeps the processes that count as the running
// game for the exit watch. The monitored root process is authoritative: while
// its PID (with the recorded start time) is alive the session must not end,
// even when its /proc paths do not match the configured install directory --
// the normal Proton/Wine case. It never filters the root by name for the same
// reason. Tracked descendants still count so a launcher hand-off keeps the
// session alive.
func linuxExitWatchGameProcesses(rootPID uint32, config ExitWatch, tracked []processutils.ProcessInfo) []processutils.ProcessInfo {
	processes := make([]processutils.ProcessInfo, 0, len(tracked))
	seen := make(map[uint32]bool, len(tracked))
	for _, proc := range tracked {
		if proc.PID == 0 || seen[proc.PID] {
			continue
		}
		if proc.PID == rootPID {
			if config.IgnoreRootProcess {
				continue
			}
		} else if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		seen[proc.PID] = true
		processes = append(processes, proc)
	}
	return processes
}
