package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Create and seal a secret interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInteractiveWizard()
	},
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

func runInteractiveWizard() error {
	numVarsInput, err := promptTextInput("How many variables for the secret?", "1")
	if err != nil {
		return err
	}

	numVars, err := strconv.Atoi(strings.TrimSpace(numVarsInput))
	if err != nil {
		return fmt.Errorf("invalid number of variables %q: %w", numVarsInput, err)
	}
	if numVars < 0 {
		return fmt.Errorf("number of variables cannot be negative")
	}

	secretName, err := promptTextInput("Secret name", "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(secretName) == "" {
		return fmt.Errorf("secret name cannot be empty")
	}

	namespaces, err := listNamespaces()
	if err != nil {
		return err
	}

	namespace, err := promptSelectOption("Choose Namespace", namespaces, "default")
	if err != nil {
		return err
	}

	secretArgs := make([]string, 0, numVars)
	for i := 1; i <= numVars; i++ {
		keyName, err := promptTextInput(fmt.Sprintf("Key name for variable #%d", i), "")
		if err != nil {
			return err
		}
		if strings.TrimSpace(keyName) == "" {
			return fmt.Errorf("key name for variable #%d cannot be empty", i)
		}

		useFile, err := promptSelectOption(
			"Choose input method",
			[]string{"Manual input (from-literal)", "File input (from-file)"},
			"",
		)
		if err != nil {
			return err
		}

		if useFile == "File input (from-file)" {
			filePath, err := promptTextInput(fmt.Sprintf("Path to file for %s", keyName), "")
			if err != nil {
				return err
			}
			if _, err := os.Stat(filePath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("❌ File %q does not exist. Exiting.", filePath)
				}
				return fmt.Errorf("failed to read file %q: %w", filePath, err)
			}
			secretArgs = append(secretArgs, fmt.Sprintf("--from-file=%s=%s", keyName, filePath))
			continue
		}

		fmt.Println(`If you're inputting JSON manually, make sure all double quotes (") are escaped:

Example:
  {\"key\":\"value\"}

For complex values, use file input instead.`)

		value, err := promptTextInput(fmt.Sprintf("Value for %s", keyName), "")
		if err != nil {
			return err
		}
		secretArgs = append(secretArgs, fmt.Sprintf("--from-literal=%s=%s", keyName, value))
	}

	secretType, err := promptSelectOption("Choose secret type", []string{"Opaque", "kubernetes.io/basic-auth"}, "Opaque")
	if err != nil {
		return err
	}

	outputFile, err := createAndSealSecret(secretName, namespace, secretType, secretArgs)
	if err != nil {
		return err
	}

	fmt.Printf("Sealed secret written to %s\n", outputFile)
	return nil
}

type textPromptModel struct {
	title        string
	defaultValue string
	input        textinput.Model
	value        string
	cancelled    bool
}

func newTextPromptModel(title, defaultValue string) textPromptModel {
	input := textinput.New()
	input.Focus()
	input.Prompt = "> "
	input.SetValue(defaultValue)

	return textPromptModel{
		title:        title,
		defaultValue: defaultValue,
		input:        input,
	}
}

func (m textPromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch key := msg.(type) {
	case tea.KeyMsg:
		switch key.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				value = m.defaultValue
			}
			m.value = value
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m textPromptModel) View() string {
	header := m.title
	if m.defaultValue != "" {
		header = fmt.Sprintf("%s (default: %s)", m.title, m.defaultValue)
	}
	return fmt.Sprintf("%s\n%s\n", header, m.input.View())
}

func promptTextInput(title, defaultValue string) (string, error) {
	model := newTextPromptModel(title, defaultValue)
	resultModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", err
	}

	result, ok := resultModel.(textPromptModel)
	if !ok {
		return "", fmt.Errorf("unexpected prompt model type")
	}
	if result.cancelled {
		return "", fmt.Errorf("prompt cancelled")
	}

	return result.value, nil
}

type selectPromptModel struct {
	title     string
	options   []string
	cursor    int
	selected  string
	cancelled bool
}

func newSelectPromptModel(title string, options []string, defaultOption string) selectPromptModel {
	cursor := 0
	for i, option := range options {
		if option == defaultOption {
			cursor = i
			break
		}
	}

	return selectPromptModel{
		title:   title,
		options: options,
		cursor:  cursor,
	}
}

func (m selectPromptModel) Init() tea.Cmd {
	return nil
}

func (m selectPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch key := msg.(type) {
	case tea.KeyMsg:
		keyText := key.String()
		switch keyText {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.options[m.cursor]
			return m, tea.Quit
		default:
			if len(keyText) == 1 && keyText[0] >= '1' && keyText[0] <= '9' {
				idx := int(keyText[0] - '1')
				if idx < len(m.options) {
					m.cursor = idx
					m.selected = m.options[m.cursor]
					return m, tea.Quit
				}
			}
		}
	}

	return m, nil
}

func (m selectPromptModel) View() string {
	var view strings.Builder
	view.WriteString(m.title)
	view.WriteString("\n")
	view.WriteString("Use ↑/↓ and Enter (or number key).\n\n")

	for i, option := range m.options {
		cursor := " "
		if i == m.cursor {
			cursor = "›"
		}
		view.WriteString(fmt.Sprintf("%s %d) %s\n", cursor, i+1, option))
	}

	return view.String()
}

func promptSelectOption(title string, options []string, defaultOption string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options available for %s", title)
	}

	model := newSelectPromptModel(title, options, defaultOption)
	resultModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", err
	}

	result, ok := resultModel.(selectPromptModel)
	if !ok {
		return "", fmt.Errorf("unexpected selection model type")
	}
	if result.cancelled {
		return "", fmt.Errorf("selection cancelled")
	}
	if strings.TrimSpace(result.selected) == "" {
		return "", fmt.Errorf("no option selected")
	}

	return result.selected, nil
}
