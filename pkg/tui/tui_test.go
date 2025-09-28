package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestInitialModel ensures the TUI starts with the correct initial state.
func TestInitialModel(t *testing.T) {
	m := InitialModel()
	if !strings.Contains(m.viewport.View(), "Welcome to the Gemini CLI!") {
		t.Errorf("Initial view does not contain welcome message")
	}

	if m.textarea.Value() != "" {
		t.Errorf("Initial textarea should be empty")
	}
}

// TestQuitMessage ensures the TUI quits on "ctrl+c".
func TestQuitMessage(t *testing.T) {
	m := InitialModel()
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Errorf("Expected a quit command, but got nil")
	}

	// tea.Quit is a function, so we can't directly compare it.
	// A common way to test this is to check if the returned command is non-nil
	// when a quit event is triggered. For a more robust test, one might
	// need to use a custom test runner, but this is sufficient for now.
}

// TestUserInputAndDisplay tests that user input is added to the conversation.
func TestUserInputAndDisplay(t *testing.T) {
	m := InitialModel()
	testInput := "Hello, Gemini!"
	m.textarea.SetValue(testInput)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := m.Update(msg)

	// Check if the input was added to the conversation history
	if !strings.Contains(newModel.(model).viewport.View(), "You: "+testInput) {
		t.Errorf("Viewport does not contain user input after sending")
	}

	// Check if the textarea was cleared
	if newModel.(model).textarea.Value() != "" {
		t.Errorf("Textarea was not cleared after sending")
	}
}