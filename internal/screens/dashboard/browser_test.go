package dashboard

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"surfista/internal/surf"
)

func TestValidBrowserURLAllowsOnlyHTTPURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  string
		ok    bool
	}{
		{value: " https://www.surfline.com/surf-report/honolua-bay/id ", want: "https://www.surfline.com/surf-report/honolua-bay/id", ok: true},
		{value: "http://www.surfline.com/surf-report/honolua-bay/id", want: "http://www.surfline.com/surf-report/honolua-bay/id", ok: true},
		{value: "file:///tmp/spot", ok: false},
		{value: "javascript:alert(1)", ok: false},
		{value: "not a URL", ok: false},
		{value: "", ok: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()
			got, ok := validBrowserURL(test.value)
			if got != test.want || ok != test.ok {
				t.Fatalf("validBrowserURL(%q) = %q, %v; want %q, %v", test.value, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestBrowserCommandUsesPlatformURLLauncher(t *testing.T) {
	t.Parallel()

	targetURL := "https://www.surfline.com/surf-report/honolua-bay/id"
	tests := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{goos: "darwin", wantName: "open", wantArgs: []string{targetURL}},
		{goos: "linux", wantName: "xdg-open", wantArgs: []string{targetURL}},
		{goos: "windows", wantName: "rundll32", wantArgs: []string{"url.dll,FileProtocolHandler", targetURL}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.goos, func(t *testing.T) {
			t.Parallel()
			name, args, err := browserCommand(test.goos, targetURL)
			if err != nil || name != test.wantName || !reflect.DeepEqual(args, test.wantArgs) {
				t.Fatalf("browserCommand(%q) = %q, %v, %v; want %q, %v, nil", test.goos, name, args, err, test.wantName, test.wantArgs)
			}
		})
	}

	if _, _, err := browserCommand("plan9", targetURL); err == nil {
		t.Fatal("unsupported operating system returned no error")
	}
}

func TestUOpensSelectedLocationURL(t *testing.T) {
	t.Parallel()

	const targetURL = "https://www.surfline.com/surf-report/honolua-bay/id"
	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay", URL: targetURL}}, nil)
	var openedURL string
	model.openURL = func(value string) error {
		openedURL = value
		return nil
	}

	if _, cmd := model.Update(dashboardKey('u')); cmd != nil {
		t.Fatal("u returned an open command without a selected location")
	}
	model, _ = model.Update(dashboardKey('j'))
	model, cmd := model.Update(dashboardKey('u'))
	if cmd == nil {
		t.Fatal("u returned no open command for a selected location with a URL")
	}
	message, ok := cmd().(URLOpenedMsg)
	if !ok {
		t.Fatalf("open command returned an unexpected message type")
	}
	if message.URL != targetURL || message.Err != nil || openedURL != targetURL {
		t.Fatalf("open result = %+v, opened URL = %q", message, openedURL)
	}
}

func TestOpenURLControlAppearsOnlyForOpenableSelection(t *testing.T) {
	t.Parallel()

	const targetURL = "https://www.surfline.com/surf-report/honolua-bay/id"
	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay", URL: targetURL}}, nil)
	if strings.Contains(model.View(), "u open") {
		t.Fatal("URL control appeared without a selected location")
	}
	model, _ = model.Update(dashboardKey('j'))
	if !strings.Contains(model.View(), "u open") {
		t.Fatal("URL control did not appear for an openable selected location")
	}

	model = New(nil, nil, []surf.Spot{{ID: "legacy", Name: "Legacy Spot"}}, nil)
	model, _ = model.Update(dashboardKey('j'))
	if strings.Contains(model.View(), "u open") {
		t.Fatal("URL control appeared for a selected location without a URL")
	}
}

func TestUDoesNothingForInvalidSelectedURL(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay", URL: "file:///tmp/spot"}}, nil)
	model.openURL = func(string) error {
		return errors.New("opener should not be called")
	}
	model, _ = model.Update(dashboardKey('j'))
	if model.CanOpenSelectionURL() {
		t.Fatal("invalid selected URL was marked openable")
	}
	if _, cmd := model.Update(dashboardKey('u')); cmd != nil {
		t.Fatal("u returned an open command for an invalid selected URL")
	}
}
