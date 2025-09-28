package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google-gemini/gemini-cli-go/pkg/auth"
	"github.com/google-gemini/gemini-cli-go/pkg/config"
	"github.com/google-gemini/gemini-cli-go/pkg/noninteractive"
	"github.com/google-gemini/gemini-cli-go/pkg/sandbox"
	"github.com/google-gemini/gemini-cli-go/pkg/tui"
	"github.com/google-gemini/gemini-cli-go/pkg/updatechecker"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/spf13/cobra"
)

var exit = os.Exit
var rootCmd *cobra.Command

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gemini-cli",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("gemini-cli-go v0.0.1")
	},
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gemini",
		Short: "A CLI for interacting with the Gemini API",
		Long:  `A command-line interface for Google's Gemini API.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if sandbox.IsInsideSandbox() {
				return nil
			}

			cfg, err := config.Load()
			if err != nil {
				// We can't use the logger here because it's not initialized yet.
				fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
				// Continue without config...
			}

			var sandboxOption any
			if cmd.Flags().Changed("sandbox") {
				sandboxOption, _ = cmd.Flags().GetBool("sandbox")
			} else if cfg != nil && cfg.Tools != nil && cfg.Tools.Sandbox != nil {
				sandboxOption = cfg.Tools.Sandbox
			}

			var sandboxImageOption string
			if cmd.Flags().Changed("sandbox-image") {
				sandboxImageOption, _ = cmd.Flags().GetString("sandbox-image")
			} else if cfg != nil && cfg.Tools != nil && cfg.Tools.SandboxImage != "" {
				sandboxImageOption = cfg.Tools.SandboxImage
			}

			sandboxCfg, err := sandbox.LoadConfig(sandboxOption, sandboxImageOption)
			if err != nil {
				return fmt.Errorf("failed to load sandbox config: %w", err)
			}

			if sandboxCfg != nil {
				if err := sandbox.Start(sandboxCfg, os.Args[1:]); err != nil {
					return fmt.Errorf("failed to start sandbox: %w", err)
				}
				exit(0)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration at the beginning
			cfg, err := config.Load()
			if err != nil {
				// We can't use the logger here because it's not initialized yet.
				fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
				// Continue without config...
			}

			// Non-interactive mode is triggered by providing args, or the --prompt flag
			prompt, _ := cmd.Flags().GetString("prompt")
			if prompt == "" && len(args) > 0 {
				prompt = strings.Join(args, " ")
			}

			if prompt == "" {
				// If no prompt is provided, check for stdin
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeNamedPipe) != 0 {
					bytes, err := io.ReadAll(os.Stdin)
					if err != nil {
						return fmt.Errorf("failed to read from stdin: %w", err)
					}
					prompt = string(bytes)
				}
			}

			if prompt == "" {
				// No prompt, start the interactive TUI
				disableUpdateNag, _ := cmd.Flags().GetBool("disable-update-nag")
				if cfg != nil && cfg.General != nil && cfg.General.DisableUpdateNag {
					disableUpdateNag = true
				}

				if !disableUpdateNag {
					p := tea.NewProgram(tui.InitialModel())
					go func() {
						release, err := updatechecker.CheckForUpdates()
						if err == nil && release != nil {
							p.Send(tui.UpdateAvailableMsg(release))
						}
					}()
					tui.Start(p)
				} else {
					p := tea.NewProgram(tui.InitialModel())
					tui.Start(p)
				}

				return nil
			}

			// Proceed with non-interactive mode
			ctx := context.Background()

			// Get auth type from config, default to oauth2
			authType := "oauth2"
			if cfg.Security != nil && cfg.Security.Auth != nil && cfg.Security.Auth.SelectedType != "" {
				authType = cfg.Security.Auth.SelectedType
			}

			// Authenticate
			authenticator, err := auth.NewAuthenticator(authType)
			if err != nil {
				return err
			}
			if err := authenticator.Authenticate(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			token, err := authenticator.GetToken()
			if err != nil {
				return err
			}

			// Get model from config or flag
			modelName, _ := cmd.Flags().GetString("model")
			if modelName == "" && cfg.Model != nil && cfg.Model.Name != "" {
				modelName = cfg.Model.Name
			}
			if modelName == "" {
				modelName = "gemini-pro" // A sensible default
			}

			// Create the client
			client, err := genai.NewClient(ctx, option.WithAPIKey(token))
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			defer client.Close()

			model := client.GenerativeModel(modelName)

			// Get output format
			outputFormat, _ := cmd.Flags().GetString("output-format")

			// Call the new non-interactive runner
			return noninteractive.Run(ctx, cfg, model, prompt, outputFormat)
		},
	}

	cmd.AddCommand(versionCmd)
	cmd.PersistentFlags().StringP("model", "m", "", "The model to use")
	cmd.PersistentFlags().StringP("prompt", "p", "", "The prompt to use (non-interactive)")
	cmd.PersistentFlags().StringP("prompt-interactive", "i", "", "Execute a prompt and then enter interactive mode")
	cmd.PersistentFlags().BoolP("sandbox", "s", false, "Run in a sandbox")
	cmd.PersistentFlags().String("sandbox-image", "", "The sandbox image to use")
	cmd.PersistentFlags().BoolP("all-files", "a", false, "Include all files in the context")
	cmd.PersistentFlags().Bool("show-memory-usage", false, "Show memory usage in the status bar")
	cmd.PersistentFlags().BoolP("yolo", "y", false, "Automatically accept all actions")
	cmd.PersistentFlags().String("approval-mode", "default", "Set the approval mode (`default`, `auto_edit`, `yolo`)")
	cmd.PersistentFlags().BoolP("checkpointing", "c", false, "Enable checkpointing of file edits")
	cmd.PersistentFlags().Bool("experimental-acp", false, "Start the agent in ACP mode")
	cmd.PersistentFlags().StringArray("allowed-mcp-server-names", []string{}, "Allowed MCP server names")
	cmd.PersistentFlags().StringArray("allowed-tools", []string{}, "Tools that are allowed to run without confirmation")
	cmd.PersistentFlags().StringArrayP("extensions", "e", []string{}, "A list of extensions to use")
	cmd.PersistentFlags().BoolP("list-extensions", "l", false, "List all available extensions and exit")
	cmd.PersistentFlags().StringArray("include-directories", []string{}, "Additional directories to include in the workspace")
	cmd.PersistentFlags().Bool("screen-reader", false, "Enable screen reader mode")
	cmd.PersistentFlags().StringP("output-format", "o", "text", "The format of the CLI output (`text`, `json`)")
	cmd.PersistentFlags().Bool("disable-update-nag", false, "Disable the update notification")

	return cmd
}

func init() {
	rootCmd = newRootCmd()
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}