package dashboard

import (
	"errors"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func validBrowserURL(rawURL string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	return parsed.String(), true
}

func browserCommand(goos, targetURL string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{targetURL}, nil
	case "linux":
		return "xdg-open", []string{targetURL}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", targetURL}, nil
	default:
		return "", nil, errors.New("opening a browser is not supported on this operating system")
	}
}

func systemOpenURL(targetURL string) error {
	name, args, err := browserCommand(runtime.GOOS, targetURL)
	if err != nil {
		return err
	}
	return exec.Command(name, args...).Run()
}

func (m Model) selectedBrowserURL() (string, bool) {
	if !m.HasSelection() {
		return "", false
	}
	return validBrowserURL(m.spots[m.selectedIndex].URL)
}

func (m Model) CanOpenSelectionURL() bool {
	_, ok := m.selectedBrowserURL()
	return ok
}

func (m Model) openSelectedURLCmd() tea.Cmd {
	targetURL, ok := m.selectedBrowserURL()
	if !ok || m.openURL == nil {
		return nil
	}
	return func() tea.Msg {
		return URLOpenedMsg{URL: targetURL, Err: m.openURL(targetURL)}
	}
}
