package sandbox

import (
	"os"
	"strings"
	"testing"
)

func TestGetSandboxCommand(t *testing.T) {
	testCases := []struct {
		name          string
		sandboxOption any
		envVar        string
		goos          string
		mockCmdExists func(string) bool
		expectedCmd   string
		expectError   bool
	}{
		// Basic cases from flag
		{"Bool true", true, "", "linux", func(cmd string) bool { return cmd == "docker" }, "docker", false},
		{"Bool false", false, "", "linux", func(cmd string) bool { return true }, "", false},
		{"String 'true'", "true", "", "linux", func(cmd string) bool { return cmd == "docker" }, "docker", false},
		{"String 'false'", "false", "", "linux", func(cmd string) bool { return true }, "", false},
		{"String 'docker'", "docker", "", "linux", func(cmd string) bool { return cmd == "docker" }, "docker", false},
		{"String 'podman'", "podman", "", "linux", func(cmd string) bool { return cmd == "podman" }, "podman", false},
		{"String 'invalid'", "invalid", "", "linux", func(cmd string) bool { return true }, "", true},

		// Env var precedence
		{"Env var 'docker'", true, "docker", "linux", func(cmd string) bool { return cmd == "docker" }, "docker", false},
		{"Env var 'podman'", true, "podman", "linux", func(cmd string) bool { return cmd == "podman" }, "podman", false},
		{"Env var 'false'", true, "false", "linux", func(cmd string) bool { return true }, "", false},
		{"Env var overrides flag", "docker", "podman", "linux", func(cmd string) bool { return cmd == "podman" }, "podman", false},

		// OS-specific behavior
		{"macOS with sandbox-exec", true, "", "darwin", func(cmd string) bool { return cmd == "sandbox-exec" }, "sandbox-exec", false},
		{"macOS falls back to docker", true, "", "darwin", func(cmd string) bool { return cmd == "docker" }, "docker", false},
		{"Linux with docker", true, "", "linux", func(cmd string) bool { return cmd == "docker" }, "docker", false},
		{"Linux with podman", true, "", "linux", func(cmd string) bool { return cmd == "podman" }, "podman", false},

		// Error cases
		{"Sandbox true, no command", true, "", "windows", func(cmd string) bool { return false }, "", true},
		{"Invalid command from env", true, "invalid", "linux", func(cmd string) bool { return true }, "", true},
		{"Specified command not found", "docker", "", "linux", func(cmd string) bool { return false }, "", true},
	}

	originalGOOS := runtimeGOOS
	originalCommandExists := commandExists
	defer func() {
		runtimeGOOS = originalGOOS
		commandExists = originalCommandExists
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envVar != "" {
				os.Setenv("GEMINI_SANDBOX", tc.envVar)
				defer os.Unsetenv("GEMINI_SANDBOX")
			} else {
				os.Unsetenv("GEMINI_SANDBOX")
			}

			runtimeGOOS = tc.goos
			commandExists = tc.mockCmdExists

			cmd, err := getSandboxCommand(tc.sandboxOption)

			if (err != nil) != tc.expectError {
				t.Errorf("expected error: %v, got: %v", tc.expectError, err)
			}
			if cmd != tc.expectedCmd {
				t.Errorf("expected command: %s, got: %s", tc.expectedCmd, cmd)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	originalCommandExists := commandExists
	defer func() {
		commandExists = originalCommandExists
	}()
	commandExists = func(cmd string) bool { return cmd == "docker" }

	testCases := []struct {
		name               string
		sandboxOption      any
		sandboxImageOption string
		envImage           string
		expectedCommand    string
		expectedImage      string
		expectError        bool
	}{
		{"Sandbox true, no image", true, "", "", "docker", "us-docker.pkg.dev/gemini-code-dev/gemini-cli/sandbox:latest", false},
		{"Sandbox true, with image flag", true, "my-image", "", "docker", "my-image", false},
		{"Sandbox true, with image env", true, "", "env-image", "docker", "env-image", false},
		{"Image flag overrides env", true, "flag-image", "env-image", "docker", "flag-image", false},
		{"Sandbox false", false, "", "", "", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envImage != "" {
				os.Setenv("GEMINI_SANDBOX_IMAGE", tc.envImage)
				defer os.Unsetenv("GEMINI_SANDBOX_IMAGE")
			} else {
				os.Unsetenv("GEMINI_SANDBOX_IMAGE")
			}

			cfg, err := LoadConfig(tc.sandboxOption, tc.sandboxImageOption)

			if (err != nil) != tc.expectError {
				t.Errorf("expected error: %v, got: %v", tc.expectError, err)
			}

			if tc.expectedCommand == "" {
				if cfg != nil {
					t.Errorf("expected nil config, got: %+v", cfg)
				}
			} else {
				if cfg == nil {
					t.Errorf("expected config, got nil")
				} else {
					if cfg.Command != tc.expectedCommand {
						t.Errorf("expected command: %s, got: %s", tc.expectedCommand, cfg.Command)
					}
					if cfg.Image != tc.expectedImage {
						t.Errorf("expected image: %s, got: %s", tc.expectedImage, cfg.Image)
					}
				}
			}
		})
	}
}

func TestStartSandboxExec(t *testing.T) {
	var capturedArgs []string
	var capturedEnv []string

	originalRunCommand := runCommandWithEnv
	runCommandWithEnv = func(name string, env []string, arg ...string) error {
		capturedArgs = append([]string{name}, arg...)
		capturedEnv = env
		return nil
	}
	defer func() {
		runCommandWithEnv = originalRunCommand
	}()

	testCases := []struct {
		name              string
		profileEnv        string
		expectedProfile   string
		expectError       bool
		expectedArgsContains []string
	}{
		{
			"Default profile",
			"",
			"permissive-open",
			false,
			[]string{"-f", "-D", "TARGET_DIR", "-D", "TMP_DIR", "-D", "HOME_DIR", "-D", "CACHE_DIR"},
		},
		{
			"Custom profile",
			"restrictive-closed",
			"restrictive-closed",
			false,
			[]string{"-f", "-D", "TARGET_DIR"},
		},
		{
			"Invalid profile",
			"non-existent-profile",
			"",
			true,
			nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			capturedArgs = nil
			capturedEnv = nil

			if tc.profileEnv != "" {
				os.Setenv("SEATBELT_PROFILE", tc.profileEnv)
				defer os.Unsetenv("SEATBELT_PROFILE")
			} else {
				os.Unsetenv("SEATBELT_PROFILE")
			}

			err := startSandboxExec(&Config{}, []string{"--some-arg"})

			if (err != nil) != tc.expectError {
				t.Errorf("expected error: %v, got: %v", tc.expectError, err)
			}

			if !tc.expectError {
				if capturedArgs[0] != "sandbox-exec" {
					t.Errorf("expected command to be 'sandbox-exec', got '%s'", capturedArgs[0])
				}
				for _, expected := range tc.expectedArgsContains {
					found := false
					for _, actual := range capturedArgs {
						if strings.HasPrefix(actual, expected) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected arg '%s' not found in %v", expected, capturedArgs)
					}
				}

				foundEnv := false
				for _, env := range capturedEnv {
					if env == "SANDBOX=sandbox-exec" {
						foundEnv = true
						break
					}
				}
				if !foundEnv {
					t.Errorf("expected 'SANDBOX=sandbox-exec' in env, but not found")
				}
			}
		})
	}
}

// Reset runtime.GOOS after tests
func TestMain(m *testing.M) {
	originalGOOS := runtimeGOOS
	code := m.Run()
	runtimeGOOS = originalGOOS
	os.Exit(code)
}