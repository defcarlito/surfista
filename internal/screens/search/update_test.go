package search

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"surfista/internal/surf"
)

type fakeSearcher struct {
	spots []surf.Spot
	err   error
	query string
}

func (f *fakeSearcher) SearchSpots(_ context.Context, query string) ([]surf.Spot, error) {
	f.query = query
	return f.spots, f.err
}

type fakeTracker struct {
	spots []surf.Spot
	added bool
	err   error
}

func (f *fakeTracker) Add(spot surf.Spot) (bool, error) {
	f.spots = append(f.spots, spot)
	return f.added, f.err
}

func TestEmptySearchQueryIsIgnored(t *testing.T) {
	t.Parallel()

	searcher := &fakeSearcher{}
	model := New(searcher, &fakeTracker{})
	model.Input.SetValue("  \t ")

	updated, cmd := model.Update(key(tea.KeyEnter, ""))
	if cmd != nil {
		t.Fatal("empty query returned a command")
	}
	if updated.Loading {
		t.Fatal("empty query entered loading state")
	}
	if searcher.query != "" {
		t.Fatalf("searcher received query %q", searcher.query)
	}
}

func TestTypingDoesNotShowAnEmptyResultState(t *testing.T) {
	t.Parallel()

	model := New(&fakeSearcher{}, &fakeTracker{})
	updated, _ := model.Update(key(0, "Honolua"))

	if updated.Input.Value() != "Honolua" {
		t.Fatalf("input value = %q, want Honolua", updated.Input.Value())
	}
	if updated.HasSearched {
		t.Fatal("typing marked the query as searched")
	}
	if strings.Contains(updated.View(), "No matching Surfline spots found") {
		t.Fatal("typing displayed an empty search result before submission")
	}
}

func TestSuccessfulSearchUsesCommandAndTypedResult(t *testing.T) {
	t.Parallel()

	want := []surf.Spot{{ID: "spot-1", Name: "Huntington State Beach"}}
	searcher := &fakeSearcher{spots: want}
	model := New(searcher, &fakeTracker{})
	model.Input.SetValue(" Huntington Beach ")

	loading, cmd := model.Update(key(tea.KeyEnter, ""))
	if cmd == nil || !loading.Loading {
		t.Fatal("valid query did not start asynchronous search")
	}
	if !loading.HasSearched {
		t.Fatal("submitted query was not marked as searched")
	}
	commandResult := searchCommandResult(t, cmd)
	msg, ok := commandResult.(SearchResultsMsg)
	if !ok {
		t.Fatalf("command returned %T, want SearchResultsMsg", commandResult)
	}
	updated, _ := loading.Update(msg)
	if updated.Loading || len(updated.Results) != 1 || updated.Results[0] != want[0] {
		t.Fatalf("unexpected search state: %+v", updated)
	}
	if searcher.query != "Huntington Beach" {
		t.Fatalf("search query = %q, want trimmed query", searcher.query)
	}
}

func TestSearchErrorAndStaleResponse(t *testing.T) {
	t.Parallel()

	searcher := &fakeSearcher{err: errors.New("API unavailable")}
	model := New(searcher, &fakeTracker{})
	model.Input.SetValue("Pipeline")
	loading, cmd := model.Update(key(tea.KeyEnter, ""))
	failed, _ := loading.Update(searchCommandResult(t, cmd))
	if failed.Err == nil || failed.Loading {
		t.Fatalf("search error state = %+v", failed)
	}

	failed.activeRequestID = 2
	stale := SearchResultsMsg{RequestID: 1, Spots: []surf.Spot{{ID: "stale"}}}
	unchanged, _ := failed.Update(stale)
	if len(unchanged.Results) != 0 {
		t.Fatalf("stale results were applied: %+v", unchanged.Results)
	}
}

func TestAddSelectedResult(t *testing.T) {
	t.Parallel()

	tracker := &fakeTracker{added: true}
	spot := surf.Spot{ID: "spot-1", Name: "Huntington State Beach"}
	model := New(&fakeSearcher{}, tracker)
	model.Results = []surf.Spot{{ID: "other", Name: "Other"}, spot}
	model.Cursor = 1

	adding, cmd := model.Update(key(tea.KeyEnter, ""))
	if cmd == nil {
		t.Fatal("Enter on selected result did not return an add command")
	}
	added, clearCmd := adding.Update(cmd())
	if len(tracker.spots) != 1 || tracker.spots[0] != spot {
		t.Fatalf("tracked spots = %+v, want selected spot", tracker.spots)
	}
	if added.Status == "" || clearCmd == nil {
		t.Fatalf("success feedback missing: %+v", added)
	}
}

func TestWindowResizeUpdatesContentAndInputWidths(t *testing.T) {
	t.Parallel()

	model := New(&fakeSearcher{}, &fakeTracker{})
	wide, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if wide.ContentWidth() != 60 {
		t.Fatalf("wide content width = %d, want 60", wide.ContentWidth())
	}
	if wide.Input.Width() != 56 {
		t.Fatalf("wide input width = %d, want 56", wide.Input.Width())
	}
	narrow, _ := wide.Update(tea.WindowSizeMsg{Width: 30, Height: 20})
	if narrow.ContentWidth() != 26 {
		t.Fatalf("narrow content width = %d, want 26", narrow.ContentWidth())
	}
	if narrow.Input.Width() != 22 {
		t.Fatalf("narrow input width = %d, want 22", narrow.Input.Width())
	}
	if renderedWidth := lipgloss.Width(narrow.View()); renderedWidth != 30 {
		t.Fatalf("rendered width = %d, want terminal width 30", renderedWidth)
	}
	for _, line := range strings.Split(narrow.View(), "\n") {
		if strings.Contains(line, "┌") && !strings.HasPrefix(line, "  ") {
			t.Fatalf("input border does not retain a two-cell margin: %q", line)
		}
	}
}

func TestResultKeyboardNavigationIsPreserved(t *testing.T) {
	t.Parallel()

	model := New(&fakeSearcher{}, &fakeTracker{})
	model.Results = []surf.Spot{{ID: "one"}, {ID: "two"}, {ID: "three"}}

	model, _ = model.Update(key(tea.KeyDown, ""))
	if model.Cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", model.Cursor)
	}
	model, _ = model.Update(key(0, "j"))
	if model.Cursor != 2 {
		t.Fatalf("cursor after j = %d, want 2", model.Cursor)
	}
	model, _ = model.Update(key(tea.KeyDown, ""))
	if model.Cursor != 2 {
		t.Fatalf("cursor moved past final result: %d", model.Cursor)
	}
	model, _ = model.Update(key(tea.KeyUp, ""))
	model, _ = model.Update(key(0, "k"))
	if model.Cursor != 0 {
		t.Fatalf("cursor after up and k = %d, want 0", model.Cursor)
	}
}

func TestEscapeClearsSearchBeforeReturning(t *testing.T) {
	t.Parallel()

	model := New(&fakeSearcher{}, &fakeTracker{})
	model.Input.SetValue("Honolua")
	model.Results = []surf.Spot{{ID: "spot-1", Name: "Honolua Bay"}}
	model.HasSearched = true

	if model.Escape() {
		t.Fatal("first Escape requested a return instead of clearing the search")
	}
	if model.Input.Value() != "" || len(model.Results) != 0 || model.HasSearched {
		t.Fatalf("first Escape did not clear search state: %+v", model)
	}
	if !model.Escape() {
		t.Fatal("second Escape did not request a return")
	}
}

func searchCommandResult(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return msg
	}
	for _, nested := range batch {
		result := nested()
		switch result.(type) {
		case SearchResultsMsg, SearchErrorMsg:
			return result
		}
	}
	t.Fatal("search batch did not contain a search result")
	return nil
}

func key(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: text}
}
