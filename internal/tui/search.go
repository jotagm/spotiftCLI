package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"cli_spotify/internal/webapi"
)

// searchState holds the search view's input, results, and selection.
type searchState struct {
	input   textinput.Model
	results []webapi.Track
	cursor  int
	typing  bool   // true while the query field is focused
	status  string // transient status/error line
}

func newSearchState() searchState {
	ti := textinput.New()
	ti.Placeholder = "search songs..."
	ti.Prompt = "  🔎 "
	ti.CharLimit = 100
	return searchState{input: ti}
}

// enterSearch focuses the query field and switches to the search view.
func (m Model) enterSearch() (Model, tea.Cmd) {
	m.view = viewSearch
	m.search.typing = true
	cmd := m.search.input.Focus()
	return m, cmd
}

// handleSearchKey routes keys while the search view is active.
func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.search.input.Blur()
		m.view = viewNowPlaying
		return m, nil
	}

	if m.search.typing {
		switch msg.String() {
		case "enter":
			query := strings.TrimSpace(m.search.input.Value())
			if query == "" {
				return m, nil
			}
			m.search.typing = false
			m.search.input.Blur()
			m.search.status = "Searching..."
			return m, doSearch(m.web, query)
		}
		var cmd tea.Cmd
		m.search.input, cmd = m.search.input.Update(msg)
		return m, cmd
	}

	// Results list is focused.
	switch msg.String() {
	case "up", "k":
		if m.search.cursor > 0 {
			m.search.cursor--
		}
	case "down", "j":
		if m.search.cursor < len(m.search.results)-1 {
			m.search.cursor++
		}
	case "/", "i":
		m.search.typing = true
		return m, m.search.input.Focus()
	case "enter":
		if t := m.selectedTrack(); t != nil {
			m.search.status = "Playing: " + t.Name
			return m, playTrack(m.pc, "", t.URI, t.Name)
		}
	}
	return m, nil
}

// selectedTrack returns the highlighted result, or nil.
func (m Model) selectedTrack() *webapi.Track {
	if m.search.cursor < 0 || m.search.cursor >= len(m.search.results) {
		return nil
	}
	return &m.search.results[m.search.cursor]
}

// searchView renders the search screen.
func (m Model) searchView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  🔎 SEARCH") + "\n\n")
	b.WriteString(m.search.input.View() + "\n\n")

	if m.search.status != "" {
		b.WriteString(dimStyle.Render("  "+m.search.status) + "\n\n")
	}

	if len(m.search.results) == 0 {
		b.WriteString(helpStyle.Render("  Type a query and press Enter. [esc] back") + "\n")
		return b.String()
	}

	// Scroll a window of results around the cursor.
	visible := m.height - 9
	if visible < 3 {
		visible = 3
	}
	start := 0
	if m.search.cursor >= visible {
		start = m.search.cursor - visible + 1
	}
	end := start + visible
	if end > len(m.search.results) {
		end = len(m.search.results)
	}

	for i := start; i < end; i++ {
		t := m.search.results[i]
		line := truncate(t.Name, 45) + dimSep + truncate(t.ArtistNames(), 30)
		if i == m.search.cursor {
			b.WriteString(greenStyle.Render("  ▸ "+line) + "\n")
		} else {
			b.WriteString("    " + dimStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [↑↓] move  [enter] play  [/] new search  [esc] back") + "\n")
	return b.String()
}

const dimSep = "  —  "
