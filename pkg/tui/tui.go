package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google-gemini/gemini-cli-go/pkg/auth"
	"github.com/google-gemini/gemini-cli-go/pkg/config"
	"github.com/google-gemini/gemini-cli-go/pkg/updatechecker"
	"github.comcom/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type (
	errMsg             error
	responseMsg        string
	conversation       []string
	UpdateAvailableMsg *updatechecker.ReleaseInfo
)

type model struct {
	viewport             viewport.Model
	textarea             textarea.Model
	senderStyle          lipgloss.Style
	responseStyle        lipgloss.Style
	errorStyle           lipgloss.Style
	client               *genai.GenerativeModel
	convo                conversation
	err                  error
	updateInfo           *updatechecker.ReleaseInfo
	geminiMdFileCount    int
	projectName          string
	sandboxActive        bool
	modelName            string
	inConversation       bool
	credentialsLoadedMsg string
}

func InitialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Type your message or @path/to/file"
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.KeyMap.InsertNewline.SetEnabled(false)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63"))

	ta.FocusedStyle.Prompt = lipgloss.NewStyle().
		Foreground(lipgloss.Color("202"))
	ta.FocusedStyle.Base = borderStyle

	vp := viewport.New(50, 10)

	wd, err := os.Getwd()
	if err != nil {
		log.Printf("could not get working directory: %v", err)
	}

	return model{
		textarea:       ta,
		viewport:       vp,
		senderStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		responseStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		errorStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		convo:          make(conversation, 0),
		projectName:    filepath.Base(wd),
		sandboxActive:  false,
		modelName:      "gemini-pro",
		inConversation: false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.initClient, m.loadGeminiMdFiles)
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
			return m, tea.Quit
		case tea.KeyEnter:
			userInput := m.textarea.Value()
			if userInput == "" {
				return m, nil
			}
			if strings.HasPrefix(userInput, "/") {
				return m.handleCommand(userInput), nil
			}
			if strings.HasPrefix(userInput, "@") {
				if !m.inConversation {
					m.inConversation = true
				}
				m.convo = append(m.convo, "Adding files via @ is not yet implemented.")
				m.textarea.Reset()
				return m, nil
			}

			if !m.inConversation {
				m.inConversation = true
			}

			m.convo = append(m.convo, m.senderStyle.Render("You: ")+userInput)
			m.textarea.Reset()
			return m, m.send(userInput)
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - m.textarea.Height() - 2
		m.textarea.SetWidth(msg.Width)
		if m.updateInfo != nil {
			m.viewport.Height--
		}
		return m, nil
	case responseMsg:
		m.convo = append(m.convo, m.responseStyle.Render("Gemini: ")+string(msg))
		return m, nil
	case UpdateAvailableMsg:
		m.updateInfo = msg
		m.viewport.Height--
		return m, nil
	case errMsg:
		m.err = msg
		m.convo = append(m.convo, m.errorStyle.Render("Error: "+msg.Error()))
		return m, nil
	default:
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	if m.err != nil {
		m.viewport.SetContent(strings.Join(m.convo, "\n"))
		return m.viewport.View()
	}

	var viewContent string
	if !m.inConversation {
		viewContent = m.renderInitialContent(m.viewport.Width)
	} else {
		viewContent = strings.Join(m.convo, "\n")
	}
	m.viewport.SetContent(viewContent)
	m.viewport.GotoBottom()

	footer := m.renderFooter()
	mainView := fmt.Sprintf(
		"%s\n%s\n%s",
		m.viewport.View(),
		m.textarea.View(),
		footer,
	)

	if m.updateInfo != nil {
		updateMessage := fmt.Sprintf(
			"Update available! %s -> %s. To update, run: go install github.com/google-gemini/gemini-cli-go@latest",
			updatechecker.CurrentVersion,
			m.updateInfo.Version,
		)
		mainView += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(updateMessage)
	}

	return mainView
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

	authenticator, hasCachedToken, err := auth.NewAuthenticator(authType)
	if err != nil {
		return errMsg(err)
	}

	if hasCachedToken {
		m.credentialsLoadedMsg = "Loaded cached credentials."
	}

	token, err := authenticator.GetToken()
	if err != nil {
		return errMsg(fmt.Errorf("authentication failed: %w", err))
	}

	if cfg.Model != nil && cfg.Model.Name != "" {
		m.modelName = cfg.Model.Name
	}
	if cfg.Tools != nil && cfg.Tools.Sandbox != nil {
		if val, ok := cfg.Tools.Sandbox.(bool); ok {
			m.sandboxActive = val
		} else if val, ok := cfg.Tools.Sandbox.(string); ok {
			m.sandboxActive = val != ""
		}
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(token))
	if err != nil {
		return errMsg(fmt.Errorf("failed to create client: %w", err))
	}

	m.client = client.GenerativeModel(m.modelName)
	return nil
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

func (m model) handleCommand(input string) model {
	switch input {
	case "/quit":
	case "/help":
		if !m.inConversation {
			m.inConversation = true
		}
		m.convo = append(m.convo, getHelpText())
		m.textarea.Reset()
	}
	return m
}

func (m *model) renderInitialContent(width int) string {
	var logo string
	if width >= 100 {
		logo = longAsciiLogo
	} else if width >= 60 {
		logo = shortAsciiLogo
	} else {
		logo = tinyAsciiLogo
	}

	tips := "Tips for getting started:\n" +
		"1. Ask questions, edit files, or run commands.\n" +
		"2. Be specific for the best results.\n" +
		"3. /help for more information."

	geminiFiles := fmt.Sprintf("Using: %d GEMINI.md files", m.geminiMdFileCount)

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s",
		m.credentialsLoadedMsg,
		logo,
		tips,
		geminiFiles,
	)
}

func (m *model) renderFooter() string {
	project := fmt.Sprintf("Project: %s", m.projectName)
	sandbox := fmt.Sprintf("Sandbox: %s", tern(m.sandboxActive, "Active", "Inactive"))
	modelInfo := fmt.Sprintf("Model: %s", m.modelName)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		project,
		"  |  ",
		sandbox,
		"  |  ",
		modelInfo,
	)
}

func (m *model) loadGeminiMdFiles() tea.Msg {
	var count int
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "GEMINI.md" {
			count++
		}
		return nil
	})
	if err != nil {
		return errMsg(fmt.Errorf("error counting GEMINI.md files: %w", err))
	}
	m.geminiMdFileCount = count
	return nil
}

func getHelpText() string {
	return "Available Commands:\n" +
		"  /help      Show this help message\n" +
		"  /quit      Exit the application\n" +
		"  @<file>   Add a file to the context"
}

func tern(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func Start(p *tea.Program) {
	if _, err := p.Run(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}