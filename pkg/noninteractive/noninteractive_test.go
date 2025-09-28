package noninteractive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google-gemini/gemini-cli-go/pkg/config"
	"github.com/google/generative-ai-go/genai"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/option"
)

func TestRun_SimpleTextResponse(t *testing.T) {
	// 1. Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[
			{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]},
			{"candidates":[{"content":{"parts":[{"text":" World"}]}}]}
		]`)
	}))
	defer server.Close()

	// 2. Setup client and model
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey("fake-api-key"), option.WithEndpoint(server.URL))
	assert.NoError(t, err)
	model := client.GenerativeModel("gemini-pro")

	// 3. Setup config and run
	cfg := &config.Settings{}

	// Redirect stdout
	r, w, _ := os.Pipe()
	tmp := os.Stdout
	defer func() {
		os.Stdout = tmp
	}()
	os.Stdout = w

	// 4. Run the function with default text format
	runErr := Run(ctx, cfg, model, "Test prompt", "text")
	w.Close()

	// 5. Assertions
	assert.NoError(t, runErr)

	var buf strings.Builder
	io.Copy(&buf, r)

	assert.Equal(t, "Hello World\n", buf.String())
}

func TestRun_SingleFunctionCall(t *testing.T) {
	// 1. Setup mock server
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if callCount == 0 {
			fmt.Fprintln(w, `[
				{"candidates":[{"content":{"parts":[{"functionCall":{"name":"testTool","args":{"arg1":"value1"}}}]}}]}
			]`)
		} else {
			fmt.Fprintln(w, `[
				{"candidates":[{"content":{"parts":[{"text":"Final answer"}]}}]}
			]`)
		}
		callCount++
	}))
	defer server.Close()

	// 2. Setup client and model
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey("fake-api-key"), option.WithEndpoint(server.URL))
	assert.NoError(t, err)
	model := client.GenerativeModel("gemini-pro")

	// 3. Setup config and run
	cfg := &config.Settings{}

	// Redirect stdout
	r, w, _ := os.Pipe()
	tmp := os.Stdout
	defer func() {
		os.Stdout = tmp
	}()
	os.Stdout = w

	// 4. Run
	runErr := Run(ctx, cfg, model, "Use a tool", "text")
	w.Close()

	// 5. Assertions
	assert.NoError(t, runErr)

	var buf strings.Builder
	io.Copy(&buf, r)

	assert.Equal(t, "Final answer\n", buf.String())
	assert.Equal(t, 2, callCount, "Expected two calls to the model")
}

func TestRun_JsonOutput(t *testing.T) {
	// 1. Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[
			{"candidates":[{"content":{"parts":[{"text":"JSON response"}]}}]}
		]`)
	}))
	defer server.Close()

	// 2. Setup client and model
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey("fake-api-key"), option.WithEndpoint(server.URL))
	assert.NoError(t, err)
	model := client.GenerativeModel("gemini-pro")

	// 3. Setup config and run
	cfg := &config.Settings{}

	// Redirect stdout
	r, w, _ := os.Pipe()
	tmp := os.Stdout
	defer func() {
		os.Stdout = tmp
	}()
	os.Stdout = w

	// 4. Run with "json" format
	runErr := Run(ctx, cfg, model, "Test prompt", "json")
	w.Close()

	// 5. Assertions
	assert.NoError(t, runErr)

	var buf strings.Builder
	io.Copy(&buf, r)

	var output JSONOutput
	jsonErr := json.Unmarshal([]byte(buf.String()), &output)
	assert.NoError(t, jsonErr, "Failed to unmarshal JSON output")

	assert.Equal(t, "JSON response", output.Response)
	assert.NotNil(t, output.Stats)
}