package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google-gemini/gemini-cli-go/pkg/auth"
	"github.com/google-gemini/gemini-cli-go/pkg/config"
	"github.com/google-gemini/gemini-cli-go/pkg/noninteractive"
	"github.com/google-gemini/gemini-cli-go/pkg/tui"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gemini",
	Short: "A CLI for interacting with the Gemini API",
	Long:  `A command-line interface for Google's Gemini API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Non-interactive mode is triggered by providing args, or the --prompt flag
		prompt, _ := cmd.Flags().GetString("prompt")
		if prompt == "" && len(args) > 0 {
			prompt = strings.Join(args, " ")
		}

		if prompt == "" {
			// No prompt, start the interactive TUI
			tui.Start()
			return nil
		}

		// Proceed with non-interactive mode
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gemini-cli",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("gemini-cli-go v0.0.1")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.PersistentFlags().StringP("model", "m", "", "The model to use")
	rootCmd.PersistentFlags().StringP("prompt", "p", "", "The prompt to use (non-interactive)")
	rootCmd.PersistentFlags().StringP("prompt-interactive", "i", "", "Execute a prompt and then enter interactive mode")
	rootCmd.PersistentFlags().BoolP("sandbox", "s", false, "Run in a sandbox")
	rootCmd.PersistentFlags().String("sandbox-image", "", "The sandbox image to use")
	rootCmd.PersistentFlags().BoolP("all-files", "a", false, "Include all files in the context")
	rootCmd.PersistentFlags().Bool("show-memory-usage", false, "Show memory usage in the status bar")
	rootCmd.PersistentFlags().BoolP("yolo", "y", false, "Automatically accept all actions")
	rootCmd.PersistentFlags().String("approval-mode", "default", "Set the approval mode (`default`, `auto_edit`, `yolo`)")
	rootCmd.PersistentFlags().BoolP("checkpointing", "c", false, "Enable checkpointing of file edits")
	rootCmd.PersistentFlags().Bool("experimental-acp", false, "Start the agent in ACP mode")
	rootCmd.PersistentFlags().StringArray("allowed-mcp-server-names", []string{}, "Allowed MCP server names")
	rootCmd.PersistentFlags().StringArray("allowed-tools", []string{}, "Tools that are allowed to run without confirmation")
	rootCmd.PersistentFlags().StringArrayP("extensions", "e", []string{}, "A list of extensions to use")
	rootCmd.PersistentFlags().BoolP("list-extensions", "l", false, "List all available extensions and exit")
	rootCmd.PersistentFlags().StringArray("include-directories", []string{}, "Additional directories to include in the workspace")
	rootCmd.PersistentFlags().Bool("screen-reader", false, "Enable screen reader mode")
	rootCmd.PersistentFlags().StringP("output-format", "o", "text", "The format of the CLI output (`text`, `json`)")
}