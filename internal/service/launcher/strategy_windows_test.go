//go:build windows

package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"yukihub/internal/appconf"
	"yukihub/internal/common/enums"
	"yukihub/internal/models"
)

func TestWindowsLauncherStrategyNativePlan(t *testing.T) {
	game := &models.Game{Path: `C:\Games\Game.exe`}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.File != game.Path {
		t.Fatalf("expected file %q, got %q", game.Path, plan.File)
	}
	if plan.Dir != filepath.Dir(game.Path) || plan.DetectionDir != filepath.Dir(game.Path) {
		t.Fatalf("unexpected dirs: dir=%q detection=%q", plan.Dir, plan.DetectionDir)
	}
	if plan.DetectionMode != DetectionStaged {
		t.Fatalf("expected staged detection, got %v", plan.DetectionMode)
	}
}

func TestWindowsLauncherStrategyUsesGameDirectoryForDetection(t *testing.T) {
	gameDirectory := t.TempDir()
	launchDirectory := filepath.Join(gameDirectory, "launcher", "bin")
	if err := os.MkdirAll(launchDirectory, 0o755); err != nil {
		t.Fatalf("create launch directory: %v", err)
	}
	game := &models.Game{
		Path:          filepath.Join(launchDirectory, "Game.exe"),
		GameDirectory: gameDirectory,
	}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.Dir != launchDirectory {
		t.Fatalf("expected launch dir %q, got %q", launchDirectory, plan.Dir)
	}
	if plan.DetectionDir != gameDirectory {
		t.Fatalf("expected detection dir %q, got %q", gameDirectory, plan.DetectionDir)
	}
}

func TestWindowsLauncherStrategyLocaleEmulatorPlan(t *testing.T) {
	gameDirectory := t.TempDir()
	launchDirectory := filepath.Join(gameDirectory, "bin")
	if err := os.MkdirAll(launchDirectory, 0o755); err != nil {
		t.Fatalf("create launch directory: %v", err)
	}
	game := &models.Game{
		Path:              filepath.Join(launchDirectory, "Game.exe"),
		GameDirectory:     gameDirectory,
		UseLocaleEmulator: true,
	}
	cfg := &appconf.AppConfig{LocaleEmulatorPath: `C:\Tools\LEProc.exe`}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, cfg)
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if plan.File != cfg.LocaleEmulatorPath {
		t.Fatalf("expected LE file %q, got %q", cfg.LocaleEmulatorPath, plan.File)
	}
	if len(plan.Args) != 1 || plan.Args[0] != game.Path {
		t.Fatalf("unexpected args: %#v", plan.Args)
	}
	if plan.Dir != launchDirectory {
		t.Fatalf("expected launch dir %q, got %q", launchDirectory, plan.Dir)
	}
	if plan.DetectionDir != gameDirectory {
		t.Fatalf("expected detection dir %q, got %q", gameDirectory, plan.DetectionDir)
	}
	if plan.DetectionMode != DetectionStaged {
		t.Fatalf("expected staged detection, got %v", plan.DetectionMode)
	}
}

func TestEffectiveProcessDetectionDirFallsBackForUnrelatedDirectory(t *testing.T) {
	gameDirectory := t.TempDir()
	launchDirectory := filepath.Join(t.TempDir(), "bin")

	if got := EffectiveProcessDetectionDir(gameDirectory, launchDirectory); got != launchDirectory {
		t.Fatalf("expected unrelated game directory to fall back to %q, got %q", launchDirectory, got)
	}
	if got := EffectiveProcessDetectionDir(filepath.Join(t.TempDir(), "missing"), launchDirectory); got != launchDirectory {
		t.Fatalf("expected missing game directory to fall back to %q, got %q", launchDirectory, got)
	}
}

func TestWindowsLauncherStrategyAdminPlan(t *testing.T) {
	admin := true
	game := &models.Game{Path: `C:\Games\Game.exe`}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{RunAsAdmin: &admin}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{RunAsAdmin: &admin})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.RunAsAdmin {
		t.Fatalf("expected RunAsAdmin=true")
	}
}

func TestWindowsLauncherStrategyUsesPersistedAdminLaunchMode(t *testing.T) {
	game := &models.Game{
		Path:       `C:\Games\Game.exe`,
		LaunchMode: enums.LaunchModeAdmin,
	}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.RunAsAdmin {
		t.Fatal("expected persisted admin launch mode to enable RunAsAdmin")
	}
}

func TestWindowsLauncherStrategyLaunchOptionCanDisablePersistedAdmin(t *testing.T) {
	admin := false
	game := &models.Game{
		Path:       `C:\Games\Game.exe`,
		LaunchMode: enums.LaunchModeAdmin,
	}

	strategy, err := SelectLauncherStrategy(game, LaunchOptions{RunAsAdmin: &admin}, &appconf.AppConfig{})
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, LaunchOptions{RunAsAdmin: &admin})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.RunAsAdmin {
		t.Fatal("expected explicit RunAsAdmin=false to disable persisted admin mode")
	}
}

func planForGame(t *testing.T, game *models.Game, opts LaunchOptions, cfg *appconf.AppConfig) LaunchPlan {
	t.Helper()
	strategy, err := SelectLauncherStrategy(game, opts, cfg)
	if err != nil {
		t.Fatalf("select strategy: %v", err)
	}
	plan, err := strategy.Plan(context.Background(), game, opts)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return plan
}

// Magpie（超分）是 Windows 独有能力：游戏字段开启后必须进入启动计划。
func TestWindowsLauncherStrategyCarriesMagpieFromGameField(t *testing.T) {
	game := &models.Game{Path: `C:\Games\Game.exe`, UseMagpie: true}

	plan := planForGame(t, game, LaunchOptions{}, &appconf.AppConfig{})
	if !plan.Magpie {
		t.Fatal("expected game-level UseMagpie to enable Magpie in the launch plan")
	}
}

// 单次启动的 Magpie 覆盖优先于游戏字段，且能反向关闭。
func TestWindowsLauncherStrategyMagpieLaunchOptionOverride(t *testing.T) {
	enable := true
	disable := false

	enabled := planForGame(t, &models.Game{Path: `C:\Games\Game.exe`}, LaunchOptions{UseMagpie: &enable}, &appconf.AppConfig{})
	if !enabled.Magpie {
		t.Fatal("expected UseMagpie=true override to enable Magpie")
	}

	disabled := planForGame(t, &models.Game{Path: `C:\Games\Game.exe`, UseMagpie: true}, LaunchOptions{UseMagpie: &disable}, &appconf.AppConfig{})
	if disabled.Magpie {
		t.Fatal("expected UseMagpie=false override to disable game-level Magpie")
	}
}

// 未配置 Locale Emulator 路径时，即使游戏要求转区也必须回落到原生启动，
// 避免计划里出现一个不存在的可执行文件。
func TestWindowsLauncherStrategyFallsBackToNativeWhenLocaleEmulatorUnconfigured(t *testing.T) {
	game := &models.Game{Path: `C:\Games\Game.exe`, UseLocaleEmulator: true}

	plan := planForGame(t, game, LaunchOptions{}, &appconf.AppConfig{})
	if plan.File != game.Path {
		t.Fatalf("expected native fallback to %q, got %q", game.Path, plan.File)
	}
	if len(plan.Args) != 0 {
		t.Fatalf("expected no wrapper args on native fallback, got %#v", plan.Args)
	}
}

// 单次启动的转区开关可以在游戏字段未开启时临时启用 Locale Emulator。
func TestWindowsLauncherStrategyLocaleEmulatorLaunchOptionOverride(t *testing.T) {
	useLE := true
	game := &models.Game{Path: `C:\Games\Game.exe`}
	cfg := &appconf.AppConfig{LocaleEmulatorPath: `C:\Tools\LEProc.exe`}

	plan := planForGame(t, game, LaunchOptions{UseLocaleEmulator: &useLE}, cfg)
	if plan.File != cfg.LocaleEmulatorPath {
		t.Fatalf("expected LE file %q, got %q", cfg.LocaleEmulatorPath, plan.File)
	}
	if len(plan.Args) != 1 || plan.Args[0] != game.Path {
		t.Fatalf("unexpected args: %#v", plan.Args)
	}
}

// 转区与超分可叠加：经 Locale Emulator 启动时仍须保留 Magpie 标记。
func TestWindowsLauncherStrategyLocaleEmulatorCarriesMagpie(t *testing.T) {
	game := &models.Game{
		Path:              `C:\Games\Game.exe`,
		UseLocaleEmulator: true,
		UseMagpie:         true,
	}
	cfg := &appconf.AppConfig{LocaleEmulatorPath: `C:\Tools\LEProc.exe`}

	plan := planForGame(t, game, LaunchOptions{}, cfg)
	if plan.File != cfg.LocaleEmulatorPath {
		t.Fatalf("expected LE file %q, got %q", cfg.LocaleEmulatorPath, plan.File)
	}
	if !plan.Magpie {
		t.Fatal("expected Magpie to survive the Locale Emulator strategy")
	}
}
