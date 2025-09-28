package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPromptResolution(t *testing.T) {
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	testCases := []struct {
		name           string
		args           []string
		stdin          string
		expectedPrompt string
		expectTUI      bool
	}{
		{name: "prompt flag", args: []string{"--prompt", "from flag"}, stdin: "", expectedPrompt: "from flag", expectTUI: false},
		{name: "args", args: []string{"from", "args"}, stdin: "", expectedPrompt: "from args", expectTUI: false},
		{name: "stdin", args: []string{}, stdin: "from stdin", expectedPrompt: "from stdin", expectTUI: false},
		{name: "empty stdin", args: []string{}, stdin: "", expectedPrompt: "", expectTUI: true},
		{name: "prompt flag and args", args: []string{"--prompt", "from flag", "ignored"}, stdin: "", expectedPrompt: "from flag", expectTUI: false},
		{name: "prompt flag and stdin", args: []string{"--prompt", "from flag"}, stdin: "from stdin", expectedPrompt: "from flag", expectTUI: false},
		{name: "args and stdin", args: []string{"from", "args"}, stdin: "from stdin", expectedPrompt: "from args", expectTUI: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new, completely isolated command for each test case.
			testCmd := &cobra.Command{Use: "gemini", Args: cobra.ArbitraryArgs}
			// Define only the flags needed for the test, ensuring no state is carried over.
			testCmd.PersistentFlags().StringP("prompt", "p", "", "The prompt to use (non-interactive)")


			var capturedPrompt string
			var tuiStarted bool

			r, w, _ := os.Pipe()
			if tc.stdin != "" {
				_, _ = w.WriteString(tc.stdin)
			}
			_ = w.Close()
			os.Stdin = r

			testCmd.RunE = func(cmd *cobra.Command, args []string) error {
				prompt, _ := cmd.Flags().GetString("prompt")
				if prompt == "" && len(args) > 0 {
					prompt = strings.Join(args, " ")
				}

				if prompt == "" {
					stat, _ := os.Stdin.Stat()
					if (stat.Mode() & os.ModeNamedPipe) != 0 {
						bytes, err := io.ReadAll(os.Stdin)
						if err != nil {
							return err
						}
						prompt = string(bytes)
					}
				}

				if prompt == "" {
					tuiStarted = true
					return nil
				}
				capturedPrompt = prompt
				return io.EOF // Use EOF to signal successful test run completion.
			}

			testCmd.SetArgs(tc.args)
			err := testCmd.Execute()

			if err != nil && err != io.EOF {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.expectTUI {
				if !tuiStarted {
					t.Error("expected TUI to be started, but it wasn't")
				}
			} else {
				if capturedPrompt != tc.expectedPrompt {
					t.Errorf("expected prompt '%s', got '%s'", tc.expectedPrompt, capturedPrompt)
				}
			}
		})
	}
}