package sandbox

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

//go:embed profiles/*.sb
var profiles embed.FS

// Config holds the configuration for the sandbox.
type Config struct {
	Command string
	Image   string
}

// Start starts the sandbox if it's configured.
func Start(cfg *Config, args []string) error {
	if cfg == nil {
		return nil
	}

	switch cfg.Command {
	case "docker", "podman":
		return startContainer(cfg, args)
	case "sandbox-exec":
		return startSandboxExec(cfg, args)
	default:
		return fmt.Errorf("unknown sandbox command: %s", cfg.Command)
	}
}

func startContainer(cfg *Config, args []string) error {
	if err := ensureSandboxImageIsPresent(cfg.Command, cfg.Image); err != nil {
		return err
	}

	fmt.Printf("hopping into sandbox (command: %s, image: %s) ...\n", cfg.Command, cfg.Image)

	cmdArgs := []string{"run", "-i", "--rm", "--init"}

	// Add TTY if stdin is a TTY
	if fileInfo, _ := os.Stdin.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		cmdArgs = append(cmdArgs, "-t")
	}

	// Mount current working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	cmdArgs = append(cmdArgs, "--volume", fmt.Sprintf("%s:%s", workDir, workDir))
	cmdArgs = append(cmdArgs, "--workdir", workDir)

	// Set SANDBOX env var
	cmdArgs = append(cmdArgs, "--env", fmt.Sprintf("SANDBOX=%s", cfg.Command))

	// Image and command
	cmdArgs = append(cmdArgs, cfg.Image)
	cmdArgs = append(cmdArgs, os.Args[0]) // The path to the gemini executable
	cmdArgs = append(cmdArgs, args...)

	return runCommand(cfg.Command, cmdArgs...)
}

func startSandboxExec(cfg *Config, args []string) error {
	profileName := os.Getenv("SEATBELT_PROFILE")
	if profileName == "" {
		profileName = "permissive-open"
	}

	profileData, err := fs.ReadFile(profiles, filepath.Join("profiles", profileName+".sb"))
	if err != nil {
		return fmt.Errorf("missing macos seatbelt profile '%s'", profileName)
	}

	tmpfile, err := os.CreateTemp("", "sandbox-profile-*.sb")
	if err != nil {
		return fmt.Errorf("failed to create temp profile file: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(profileData); err != nil {
		return fmt.Errorf("failed to write to temp profile file: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return fmt.Errorf("failed to close temp profile file: %w", err)
	}

	fmt.Printf("hopping into sandbox (command: sandbox-exec, profile: %s) ...\n", profileName)

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	tmpDir := os.TempDir()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	cmdArgs := []string{
		"-f", tmpfile.Name(),
		"-D", fmt.Sprintf("TARGET_DIR=%s", workDir),
		"-D", fmt.Sprintf("TMP_DIR=%s", tmpDir),
		"-D", fmt.Sprintf("HOME_DIR=%s", homeDir),
		"-D", fmt.Sprintf("CACHE_DIR=%s", cacheDir),
	}

	// Add dummy INCLUDE_DIR params for now.
	for i := 0; i < 5; i++ {
		cmdArgs = append(cmdArgs, "-D", fmt.Sprintf("INCLUDE_DIR_%d=/dev/null", i))
	}

	// The command to run inside the sandbox
	sandboxedCmd := append([]string{os.Args[0]}, args...)
	cmdArgs = append(cmdArgs, sandboxedCmd...)

	// We need to set the SANDBOX env var for the child process
	env := os.Environ()
	env = append(env, "SANDBOX=sandbox-exec")

	return runCommandWithEnv("sandbox-exec", env, cmdArgs...)
}

func ensureSandboxImageIsPresent(sandboxCmd, image string) error {
	exists, err := imageExists(sandboxCmd, image)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	}
	if exists {
		return nil
	}

	fmt.Printf("Image %s not found locally, attempting to pull...\n", image)
	if err := pullImage(sandboxCmd, image); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}

	exists, err = imageExists(sandboxCmd, image)
	if err != nil {
		return fmt.Errorf("failed to check if image exists after pull: %w", err)
	}
	if !exists {
		return fmt.Errorf("failed to obtain sandbox image %s after pull attempt", image)
	}

	return nil
}

func imageExists(sandboxCmd, image string) (bool, error) {
	cmd := exec.Command(sandboxCmd, "images", "-q", image)
	output, err := cmd.Output()
	if err != nil {
		// This could be because the docker/podman daemon is not running.
		return false, fmt.Errorf("'%s images' command failed: %w", sandboxCmd, err)
	}
	return strings.TrimSpace(string(output)) != "", nil
}

func pullImage(sandboxCmd, image string) error {
	fmt.Printf("Pulling image %s using %s...\n", image, sandboxCmd)
	cmd := exec.Command(sandboxCmd, "pull", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsInsideSandbox checks if the CLI is running inside a sandbox.
func IsInsideSandbox() bool {
	return os.Getenv("SANDBOX") != ""
}

// LoadConfig loads the sandbox configuration based on settings, and CLI arguments.
func LoadConfig(sandboxOption any, sandboxImageOption string) (*Config, error) {
	command, err := getSandboxCommand(sandboxOption)
	if err != nil {
		return nil, err
	}

	if command == "" {
		return nil, nil
	}

	image := sandboxImageOption
	if image == "" {
		image = os.Getenv("GEMINI_SANDBOX_IMAGE")
	}
	if image == "" {
		// Fallback to a default image name, similar to the nodejs version
		image = "us-docker.pkg.dev/gemini-code-dev/gemini-cli/sandbox:latest"
	}

	if image == "" && command != "sandbox-exec" {
		return nil, fmt.Errorf("sandbox image is not specified")
	}

	return &Config{
		Command: command,
		Image:   image,
	}, nil
}

var (
	// For testing purposes
	runtimeGOOS = runtime.GOOS
)

func getSandboxCommand(sandboxOption any) (string, error) {
	if IsInsideSandbox() {
		return "", nil
	}

	envSandbox := os.Getenv("GEMINI_SANDBOX")
	if envSandbox != "" {
		sandboxOption = envSandbox
	}

	var sandbox bool
	var sandboxCmd string

	switch v := sandboxOption.(type) {
	case bool:
		sandbox = v
	case string:
		val := strings.ToLower(strings.TrimSpace(v))
		if val == "1" || val == "true" {
			sandbox = true
		} else if val == "0" || val == "false" || val == "" {
			sandbox = false
		} else {
			sandboxCmd = val
		}
	default:
		// if the flag is not a bool or string, assume false
		sandbox = false
	}

	if !sandbox && sandboxCmd == "" {
		return "", nil
	}

	validCommands := []string{"docker", "podman", "sandbox-exec"}
	isValidCmd := func(cmd string) bool {
		for _, c := range validCommands {
			if c == cmd {
				return true
			}
		}
		return false
	}

	if sandboxCmd != "" {
		if !isValidCmd(sandboxCmd) {
			return "", fmt.Errorf("invalid sandbox command '%s'. Must be one of %v", sandboxCmd, validCommands)
		}
		if !commandExists(sandboxCmd) {
			return "", fmt.Errorf("missing sandbox command '%s' (from GEMINI_SANDBOX)", sandboxCmd)
		}
		return sandboxCmd, nil
	}

	if runtimeGOOS == "darwin" && commandExists("sandbox-exec") {
		return "sandbox-exec", nil
	}
	if commandExists("docker") && sandbox {
		return "docker", nil
	}
	if commandExists("podman") && sandbox {
		return "podman", nil
	}

	if sandbox {
		return "", fmt.Errorf("GEMINI_SANDBOX is true but failed to determine command for sandbox; install docker or podman or specify command in GEMINI_SANDBOX")
	}

	return "", nil
}

// commandExists checks if a command exists in the system's PATH.
var commandExists = func(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// SetCommandExists is for testing purposes only
func SetCommandExists(f func(string) bool) error {
	commandExists = f
	return nil
}

// runCommand executes a command and replaces the current process with it.
func runCommand(name string, arg ...string) error {
	return runCommandWithEnv(name, os.Environ(), arg...)
}

var runCommandWithEnv = func(name string, env []string, arg ...string) error {
	// Look for the executable in the PATH
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("executable not found: %s", name)
	}

	// The first argument to syscall.Exec must be the path to the executable.
	// The second argument is the list of arguments, including the executable name as arg[0].
	argv := append([]string{name}, arg...)

	// Execute the command, replacing the current process
	if err := syscall.Exec(path, argv, env); err != nil {
		return fmt.Errorf("failed to exec command: %w", err)
	}

	// syscall.Exec does not return on success, so this line should not be reached.
	return nil
}