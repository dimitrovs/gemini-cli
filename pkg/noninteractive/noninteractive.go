package noninteractive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google-gemini/gemini-cli-go/pkg/config"
	"github.com/google-gemini/gemini-cli-go/pkg/tools"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
)

// JSONOutput represents the structure of the JSON output.
type JSONOutput struct {
	Response string      `json:"response"`
	Stats    interface{} `json:"stats"` // Placeholder for stats
}

// Run executes a non-interactive prompt.
func Run(ctx context.Context, cfg *config.Settings, model *genai.GenerativeModel, prompt string, outputFormat string) error {
	chat := model.StartChat()
	var responseText string

	currentUserParts := []genai.Part{genai.Text(prompt)}

	maxTurns := 10
	if cfg.Model != nil && cfg.Model.MaxSessionTurns > 0 {
		maxTurns = cfg.Model.MaxSessionTurns
	}

	turnCount := 0
	for {
		turnCount++
		if maxTurns >= 0 && turnCount > maxTurns {
			return fmt.Errorf("max turns exceeded: %d", maxTurns)
		}

		iter := chat.SendMessageStream(ctx, currentUserParts...)

		var collectedFunctionCalls []genai.FunctionCall

		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}

			if len(resp.Candidates) > 0 {
				candidate := resp.Candidates[0]
				if candidate.Content != nil {
					for _, part := range candidate.Content.Parts {
						switch v := part.(type) {
						case genai.Text:
							if outputFormat == "json" {
								responseText += string(v)
							} else {
								fmt.Fprint(os.Stdout, string(v))
							}
						case genai.FunctionCall:
							collectedFunctionCalls = append(collectedFunctionCalls, v)
						}
					}
				}
			}
		}

		if len(collectedFunctionCalls) > 0 {
			var toolResponseParts []genai.Part
			for _, fc := range collectedFunctionCalls {
				toolResponse := tools.ExecuteToolCall(&fc)
				toolResponseParts = append(toolResponseParts, toolResponse)
			}
			currentUserParts = toolResponseParts
		} else {
			// End of conversation
			if outputFormat == "json" {
				// For now, stats are empty. This can be implemented later.
				stats := map[string]interface{}{}
				output := JSONOutput{
					Response: responseText,
					Stats:    stats,
				}
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(output); err != nil {
					return fmt.Errorf("failed to encode JSON: %w", err)
				}
			} else {
				fmt.Fprintln(os.Stdout) // Final newline for text output
			}
			return nil
		}
	}
}