package statuscmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestRunStatus_Summary_PrintsDaemonWarnings(t *testing.T) {
	warnings := []string{"config dir permissions too open", "failed to load forward rule \"web\""}
	stubMockResponses(t, map[string]json.RawMessage{
		"daemon.status": mustJSON(t, protocol.DaemonStatusResult{PID: 12345, Uptime: "1h", Warnings: warnings}),
		"host.list":     mustJSON(t, protocol.HostListResult{}),
		"session.list":  mustJSON(t, protocol.SessionListResult{}),
	})

	output := captureStdout(t, func() { RunStatus(setupMockDaemonDir(t), []string{}) })

	for _, w := range warnings {
		want := i18n.T("cli.daemon.status_warning", map[string]any{"Message": w})
		if !strings.Contains(output, want) {
			t.Errorf("output should contain warning line %q, got %q", want, output)
		}
	}
}

func TestRunStatus_Summary_NoWarningsNoWarningLines(t *testing.T) {
	stubMockResponses(t, map[string]json.RawMessage{
		"daemon.status": mustJSON(t, protocol.DaemonStatusResult{PID: 12345, Uptime: "1h"}),
		"host.list":     mustJSON(t, protocol.HostListResult{}),
		"session.list":  mustJSON(t, protocol.SessionListResult{}),
	})

	output := captureStdout(t, func() { RunStatus(setupMockDaemonDir(t), []string{}) })

	marker := i18n.T("cli.daemon.status_warning", map[string]any{"Message": ""})
	if strings.Contains(output, strings.TrimSpace(marker)) {
		t.Errorf("output should not contain a warning line, got %q", output)
	}
}
