package tui

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google-gemini/gemini-cli-go/pkg/auth"
	"github.com/google-gemini/gemini-cli-go/pkg/config"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type (
	errMsg         error
	responseMsg    string
	conversation []string
)

type model struct {
	viewport      viewport.Model
	textarea      textarea.Model
	senderStyle   lipgloss.Style
	responseStyle lipgloss.Style
	errorStyle    lipgloss.Style
	client        *genai.GenerativeModel
	convo         conversation
	err           error
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 0 // No limit

	ta.SetWidth(50) // Initial width
	ta.SetHeight(1) // Single line input

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(50, 10) // Initial width and height
	vp.SetContent("Welcome to the Gemini CLI! Type a message and press Enter to start.")

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		textarea:      ta,
		viewport:      vp,
		senderStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("5")),   // Purple
		responseStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),   // Green
		errorStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("1")),   // Red
		convo:         make(conversation, 0),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.initClient)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			fmt.Println("Exiting...")
			return m, tea.Quit
		case tea.KeyEnter:
			if m.textarea.Value() == "" {
				return m, nil
			}
			userInput := m.textarea.Value()
			m.convo = append(m.convo, m.senderStyle.Render("You: ")+userInput)
			m.viewport.SetContent(strings.Join(m.convo, "\n"))
			m.textarea.Reset()
			m.viewport.GotoBottom()
			return m, m.send(userInput)
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - m.textarea.Height() - 2 // Account for textarea and gap
		m.textarea.SetWidth(msg.Width)
		return m, nil

	case responseMsg:
		m.convo = append(m.convo, m.responseStyle.Render("Gemini: ")+string(msg))
		m.viewport.SetContent(strings.Join(m.convo, "\n"))
		m.viewport.GotoBottom()
		return m, nil

	case errMsg:
		m.err = msg
		m.convo = append(m.convo, m.errorStyle.Render("Error: "+msg.Error()))
		m.viewport.SetContent(strings.Join(m.convo, "\n"))
		m.viewport.GotoBottom()
		return m, nil

	default:
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	if m.err != nil {
		// Don't render the text area on error
		return m.viewport.View()
	}
	return fmt.Sprintf(
		"%s\n\n%s",
		m.viewport.View(),
		m.textarea.View(),
	)
}

func (c conversation) String() string {
	return strings.Join(c, "\n")
}

func (m *model) initClient() tea.Msg {
	cfg, err := config.Load()
	if err != nil {
		return errMsg(fmt.Errorf("failed to load config: %w", err))
	}

	authType := "oauth2"
	if cfg.Security != nil && cfg.Security.Auth != nil && cfg.Security.Auth.SelectedType != "" {
		authType = cfg.Security.Auth.SelectedType
	}

	authenticator, err := auth.NewAuthenticator(authType)
	if err != nil {
		return errMsg(err)
	}
	if err := authenticator.Authenticate(); err != nil {
		return errMsg(fmt.Errorf("authentication failed: %w", err))
	}
	token, err := authenticator.GetToken()
	if err != nil {
		return errMsg(err)
	}

	modelName := "gemini-pro"
	if cfg.Model != nil && cfg.Model.Name != "" {
		modelName = cfg.Model.Name
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(token))
	if err != nil {
		return errMsg(fmt.Errorf("failed to create client: %w", err))
	}

	m.client = client.GenerativeModel(modelName)
	return nil // No message on success
}

func (m *model) send(prompt string) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return errMsg(fmt.Errorf("client not initialized"))
		}

		ctx := context.Background()
		resp, err := m.client.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			return errMsg(fmt.Errorf("failed to generate content: %w", err))
		}

		// Extract the text from the response
		var responseText strings.Builder
		for _, cand := range resp.Candidates {
			for _, part := range cand.Content.Parts {
				if txt, ok := part.(genai.Text); ok {
					responseText.WriteString(string(txt))
				}
			}
		}

		return responseMsg(responseText.String())
	}
}

func Start() {
	// We are disabling the alt screen buffer for now to allow users to scroll
	// back and see the conversation history after the application exits.
	// This can be re-enabled if we add a "copy to clipboard" feature.
	p := tea.NewProgram(initialModel())

	if err := p.Start(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}