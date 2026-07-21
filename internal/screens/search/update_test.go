package search

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
	model.Query = "  \t "

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
	model.Query = " Huntington Beach "

	loading, cmd := model.Update(key(tea.KeyEnter, ""))
	if cmd == nil || !loading.Loading {
		t.Fatal("valid query did not start asynchronous search")
	}
	if !loading.HasSearched {
		t.Fatal("submitted query was not marked as searched")
	}
	commandResult := cmd()
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
	model.Query = "Pipeline"
	loading, cmd := model.Update(key(tea.KeyEnter, ""))
	failed, _ := loading.Update(cmd())
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

func key(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: text}
}
