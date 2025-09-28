package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestInitialView verifies the TUI starts with the correct initial state.
func TestInitialView(t *testing.T) {
	m := InitialModel()
	view := m.View()

	if !strings.Contains(view, "Tips for getting started:") {
		t.Errorf("Initial view does not contain the welcome tips")
	}

	if m.textarea.Value() != "" {
		t.Errorf("Initial textarea should be empty")
	}
}

// TestUserInputAndDisplay tests that the view transitions correctly
// from the initial screen to the conversation view.
func TestUserInputAndDisplay(t *testing.T) {
	m := InitialModel()
	testInput := "Hello, Gemini!"

	// 1. Check that the initial view is showing.
	initialView := m.View()
	if !strings.Contains(initialView, "Tips for getting started:") {
		t.Fatal("Test setup failed: Initial view does not contain the welcome tips")
	}

	// 2. Simulate user input and update the model.
	m.textarea.SetValue(testInput)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(model) // Assert type to access model fields

	// 3. Get the new view after the update.
	conversationView := m.View()

	// 4. Check that the conversation view contains the user's input.
	if !strings.Contains(conversationView, "You: "+testInput) {
		t.Errorf("Conversation view does not contain user input")
	}

	// 5. Check that the conversation view no longer contains the initial tips.
	if strings.Contains(conversationView, "Tips for getting started:") {
		t.Errorf("Conversation view should not contain the welcome tips")
	}

	// 6. Check that the textarea was cleared after sending.
	if m.textarea.Value() != "" {
		t.Errorf("Textarea was not cleared after sending message")
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
}