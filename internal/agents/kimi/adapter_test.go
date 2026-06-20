package kimi

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/model"
	"github.com/gentleman-programming/gentle-ai/internal/system"
	"github.com/gentleman-programming/gentle-ai/internal/versions"
)

func TestNewAdapter(t *testing.T) {
	a := NewAdapter()
	if a == nil {
		t.Fatal("NewAdapter() returned nil")
	}
}

func TestAdapter_Agent(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentKimi {
		t.Errorf("Agent() = %v, want %v", got, model.AgentKimi)
	}
}

func TestAdapter_Tier(t *testing.T) {
	a := NewAdapter()
	if got := a.Tier(); got != model.TierFull {
		t.Errorf("Tier() = %v, want %v", got, model.TierFull)
	}
}

func TestAdapter_ConfigPaths(t *testing.T) {
	a := NewAdapter()
	homeDir := "/home/test"

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(homeDir), filepath.Join(homeDir, ".kimi-code")},
		{"SystemPromptDir", a.SystemPromptDir(homeDir), filepath.Join(homeDir, ".kimi-code")},
		{"SystemPromptFile", a.SystemPromptFile(homeDir), filepath.Join(homeDir, ".kimi-code", "AGENTS.md")},
		{"SkillsDir", a.SkillsDir(homeDir), filepath.Join(homeDir, ".kimi-code", "skills")},
		{"SettingsPath", a.SettingsPath(homeDir), filepath.Join(homeDir, ".kimi-code", "config.toml")},
		{"CommandsDir", a.CommandsDir(homeDir), ""},
		{"SubAgentsDir", a.SubAgentsDir(homeDir), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestAdapter_Strategies(t *testing.T) {
	a := NewAdapter()

	if got := a.SystemPromptStrategy(); got != model.StrategyFileReplace {
		t.Errorf("SystemPromptStrategy() = %v, want StrategyFileReplace", got)
	}

	if got := a.MCPStrategy(); got != model.StrategyMCPConfigFile {
		t.Errorf("MCPStrategy() = %v, want StrategyMCPConfigFile", got)
	}
}

func TestAdapter_Capabilities(t *testing.T) {
	a := NewAdapter()

	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{"SupportsSkills", a.SupportsSkills(), true},
		{"SupportsMCP", a.SupportsMCP(), true},
		{"SupportsSystemPrompt", a.SupportsSystemPrompt(), true},
		{"SupportsSlashCommands", a.SupportsSlashCommands(), false},
		{"SupportsOutputStyles", a.SupportsOutputStyles(), false},
		{"SupportsSubAgents", a.SupportsSubAgents(), false},
		{"SupportsAutoInstall", a.SupportsAutoInstall(), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestAdapterSubAgentsStayFalse is a REGRESSION GUARD.
//
// Kimi Code CLI v0.18.0+ removed the LaborMarket custom YAML agent spec system.
// Only built-in subagents (coder, explore, plan) exist. Flipping SupportsSubAgents()
// to true with an empty EmbeddedSubAgentsDir() would cause sdd/inject.go and
// uninstall/service.go to call assets.FS.ReadDir("") — returning the embedded
// root and copying the entire asset tree into ~/.kimi-code/agents/. Catastrophic.
// SupportsSubAgents() MUST remain false indefinitely.
func TestAdapterSubAgentsStayFalse(t *testing.T) {
	a := NewAdapter()

	if got := a.SupportsSubAgents(); got {
		t.Fatal("SupportsSubAgents() = true — MUST stay false for Kimi: Kimi Code CLI has no custom YAML subagent registry. Flipping this flag would copy the embedded asset root into ~/.kimi-code/agents/.")
	}
	if got := a.SubAgentsDir("/home/user"); got != "" {
		t.Fatalf("SubAgentsDir() = %q, want \"\" — must stay empty for Kimi", got)
	}
	if got := a.EmbeddedSubAgentsDir(); got != "" {
		t.Fatalf("EmbeddedSubAgentsDir() = %q, want \"\" — must stay empty for Kimi", got)
	}
}

func TestAdapter_MCPConfigPath(t *testing.T) {
	a := NewAdapter()
	homeDir := "/home/test"
	serverName := "test-server"

	got := a.MCPConfigPath(homeDir, serverName)
	expected := filepath.Join(homeDir, ".kimi-code", "mcp.json")

	if got != expected {
		t.Errorf("MCPConfigPath() = %v, want %v", got, expected)
	}
}

func TestAdapter_Detect_KimiInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	kimiDir := filepath.Join(tmpDir, ".kimi-code")
	if err := os.MkdirAll(kimiDir, 0755); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{
		lookPath: func(string) (string, error) {
			return "/usr/bin/kimi", nil
		},
		statPath: func(path string) statResult {
			info, err := os.Stat(path)
			return statResult{isDir: info != nil && info.IsDir(), err: err}
		},
		pathExists: func(string) bool { return false },
		userHomeDir: func() (string, error) {
			return tmpDir, nil
		},
	}

	installed, binaryPath, configPath, configFound, err := a.Detect(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if !installed {
		t.Error("Detect() installed = false, want true")
	}
	if binaryPath != "/usr/bin/kimi" {
		t.Errorf("Detect() binaryPath = %v, want /usr/bin/kimi", binaryPath)
	}
	if !configFound {
		t.Error("Detect() configFound = false, want true")
	}
	if configPath != filepath.Join(tmpDir, ".kimi-code") {
		t.Errorf("Detect() configPath = %v", configPath)
	}
}

func TestAdapter_Detect_KimiNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()

	a := &Adapter{
		lookPath: func(string) (string, error) {
			return "", os.ErrNotExist
		},
		statPath: func(path string) statResult {
			return statResult{err: os.ErrNotExist}
		},
		pathExists: func(string) bool { return false },
		userHomeDir: func() (string, error) {
			return tmpDir, nil
		},
	}

	installed, binaryPath, configPath, configFound, err := a.Detect(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if installed {
		t.Error("Detect() installed = true, want false")
	}
	if binaryPath != "" {
		t.Errorf("Detect() binaryPath = %v, want empty", binaryPath)
	}
	if configFound {
		t.Error("Detect() configFound = true, want false")
	}
	if configPath != filepath.Join(tmpDir, ".kimi-code") {
		t.Errorf("Detect() configPath wrong: %v", configPath)
	}
}

func TestAdapter_Detect_FallbackPaths(t *testing.T) {
	tmpDir := t.TempDir()
	kimiDir := filepath.Join(tmpDir, ".kimi-code")
	if err := os.MkdirAll(kimiDir, 0755); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{
		lookPath: func(string) (string, error) {
			return "", os.ErrNotExist // Not in PATH
		},
		statPath: func(path string) statResult {
			info, err := os.Stat(path)
			return statResult{isDir: info != nil && info.IsDir(), err: err}
		},
		pathExists: func(path string) bool {
			return path == filepath.Join(tmpDir, ".kimi-code", "bin", binaryName())
		},
		userHomeDir: func() (string, error) {
			return tmpDir, nil
		},
	}

	installed, binaryPath, _, _, err := a.Detect(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !installed {
		t.Fatal("Detect() installed = false, want true when fallback path exists")
	}
	if binaryPath != filepath.Join(tmpDir, ".kimi-code", "bin", binaryName()) {
		t.Fatalf("Detect() binaryPath = %q, want fallback path", binaryPath)
	}
}

func TestConfigPath(t *testing.T) {
	homeDir := "/home/test"
	got := ConfigPath(homeDir)
	expected := filepath.Join(homeDir, ".kimi-code")
	if got != expected {
		t.Errorf("ConfigPath() = %v, want %v", got, expected)
	}
}

func TestAdapter_PostInstallMessage(t *testing.T) {
	a := NewAdapter()
	msg := a.PostInstallMessage("/home/test")

	for _, want := range []string{
		"/home/test/.kimi-code/AGENTS.md",
		"/home/test/.kimi-code/skills",
		"/home/test/.kimi-code/mcp.json",
	} {
		if !contains(msg, want) {
			t.Errorf("PostInstallMessage() missing %q in:\n%s", want, msg)
		}
	}

	// Should NOT contain legacy paths
	for _, unwanted := range []string{
		".kimi/agents",
		"gentleman.yaml",
		"uv tool install",
	} {
		if contains(msg, unwanted) {
			t.Errorf("PostInstallMessage() contains legacy reference %q", unwanted)
		}
	}
}

func TestAdapterSystemPromptFile_UsesUppercaseAGENTSmd(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/user")
	const want = "AGENTS.md"
	if filepath.Base(got) != want {
		t.Fatalf("SystemPromptFile() base = %q, want %q (Kimi Code CLI requires uppercase AGENTS.md)", filepath.Base(got), want)
	}
}

func TestInstallCommand(t *testing.T) {
	a := NewAdapter()

	tests := []struct {
		name    string
		profile system.PlatformProfile
		want    [][]string
	}{
		{
			name:    "darwin uses npm without sudo",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    [][]string{{"npm", "install", "-g", "--ignore-scripts", "@moonshot-ai/kimi-code@" + versions.Kimi}},
		},
		{
			name:    "linux system npm uses sudo",
			profile: system.PlatformProfile{OS: "linux", LinuxDistro: system.LinuxDistroUbuntu, PackageManager: "apt"},
			want:    [][]string{{"sudo", "npm", "install", "-g", "--ignore-scripts", "@moonshot-ai/kimi-code@" + versions.Kimi}},
		},
		{
			name:    "linux nvm skips sudo",
			profile: system.PlatformProfile{OS: "linux", LinuxDistro: system.LinuxDistroUbuntu, PackageManager: "apt", NpmWritable: true},
			want:    [][]string{{"npm", "install", "-g", "--ignore-scripts", "@moonshot-ai/kimi-code@" + versions.Kimi}},
		},
		{
			name:    "windows uses npm without sudo",
			profile: system.PlatformProfile{OS: "windows", PackageManager: "winget", NpmWritable: true},
			want:    [][]string{{"npm", "install", "-g", "--ignore-scripts", "@moonshot-ai/kimi-code@" + versions.Kimi}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, err := a.InstallCommand(tt.profile)
			if err != nil {
				t.Fatalf("InstallCommand() returned error: %v", err)
			}

			if len(command) != len(tt.want) {
				t.Fatalf("InstallCommand() = %v, want %v", command, tt.want)
			}
			for i := range command {
				if len(command[i]) != len(tt.want[i]) {
					t.Fatalf("InstallCommand() cmd %d = %v, want %v", i, command[i], tt.want[i])
				}
				for j := range command[i] {
					if command[i][j] != tt.want[i][j] {
						t.Fatalf("InstallCommand() cmd %d arg %d = %q, want %q", i, j, command[i][j], tt.want[i][j])
					}
				}
			}
		})
	}
}

func TestConfigPathsCrossPlatform(t *testing.T) {
	a := NewAdapter()
	home := "/tmp/home"

	if got := a.GlobalConfigDir(home); got != filepath.Join(home, ".kimi-code") {
		t.Fatalf("GlobalConfigDir() = %q, want %q", got, filepath.Join(home, ".kimi-code"))
	}

	if got := a.SkillsDir(home); got != filepath.Join(home, ".kimi-code", "skills") {
		t.Fatalf("SkillsDir() = %q, want %q", got, filepath.Join(home, ".kimi-code", "skills"))
	}

	if got := a.SystemPromptFile(home); got != filepath.Join(home, ".kimi-code", "AGENTS.md") {
		t.Fatalf("SystemPromptFile() = %q, want %q", got, filepath.Join(home, ".kimi-code", "AGENTS.md"))
	}

	if got := a.SettingsPath(home); got != filepath.Join(home, ".kimi-code", "config.toml") {
		t.Fatalf("SettingsPath() = %q, want %q", got, filepath.Join(home, ".kimi-code", "config.toml"))
	}

	if got := a.MCPConfigPath(home, "engram"); got != filepath.Join(home, ".kimi-code", "mcp.json") {
		t.Fatalf("MCPConfigPath() = %q, want %q", got, filepath.Join(home, ".kimi-code", "mcp.json"))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
