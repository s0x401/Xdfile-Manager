package cmd

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNetBoxPathRoundTrip(t *testing.T) {
	got := xdfileNetBoxURL("prod", "/var/log")
	if got != "xdssh://prod/var/log" {
		t.Fatalf("expected netbox url, got %q", got)
	}

	parsed, ok := xdfileParseNetBoxPath(got)
	if !ok {
		t.Fatalf("expected %q to parse", got)
	}
	if parsed.Profile != "prod" || parsed.Path != "/var/log" {
		t.Fatalf("unexpected parsed path: %+v", parsed)
	}

	parent, ok := xdfileNetBoxParent(got)
	if !ok || parent != "xdssh://prod/var" {
		t.Fatalf("expected parent xdssh://prod/var, got %q ok=%v", parent, ok)
	}

	joined, ok := xdfileNetBoxJoin(parent, "nginx")
	if !ok || joined != "xdssh://prod/var/nginx" {
		t.Fatalf("expected joined path, got %q ok=%v", joined, ok)
	}
}

func TestNetBoxConnectionPrefsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "netbox.json")
	connections := []xdfileNetBoxConnection{
		{Name: "prod", Host: "example.com", User: "root", Port: 2200, RemotePath: "var/www", IdentityFile: "~/.ssh/id_ed25519", Password: "secret"},
	}
	if err := xdfileSaveNetBoxPrefs(path, connections); err != nil {
		t.Fatalf("save netbox prefs: %v", err)
	}

	loaded, err := xdfileLoadNetBoxPrefs(path)
	if err != nil {
		t.Fatalf("load netbox prefs: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected one connection, got %+v", loaded)
	}
	got := loaded[0]
	if got.Name != "prod" || got.Host != "example.com" || got.User != "root" || got.Port != 2200 || got.RemotePath != "/var/www" {
		t.Fatalf("unexpected loaded connection: %+v", got)
	}
	if got.Password != "secret" {
		t.Fatalf("expected saved password to round trip")
	}
}

func TestNetBoxPasswordCacheOverridesSavedPassword(t *testing.T) {
	xdfileSetNetBoxPassword("prod", "temporary")
	defer xdfileClearNetBoxPassword("prod")

	connection := xdfileNetBoxConnection{Name: "prod", Password: "saved"}
	if got := connection.passwordForAuth(); got != "temporary" {
		t.Fatalf("expected temporary password, got %q", got)
	}

	xdfileClearNetBoxPassword("prod")
	if got := connection.passwordForAuth(); got != "saved" {
		t.Fatalf("expected saved password, got %q", got)
	}
}

func TestNetBoxParseSavePassword(t *testing.T) {
	for _, value := range []string{"yes", "Y", "1", "保存"} {
		got, err := xdfileParseNetBoxSavePassword(value)
		if err != nil || !got {
			t.Fatalf("expected %q to parse as true, got %v err=%v", value, got, err)
		}
	}
	for _, value := range []string{"", "no", "N", "0", "不保存"} {
		got, err := xdfileParseNetBoxSavePassword(value)
		if err != nil || got {
			t.Fatalf("expected %q to parse as false, got %v err=%v", value, got, err)
		}
	}
	if _, err := xdfileParseNetBoxSavePassword("maybe"); err == nil {
		t.Fatalf("expected invalid save password value to fail")
	}
}

func TestNetBoxParseListLine(t *testing.T) {
	entry, ok := xdfileNetBoxParseListLine("prod", "f\t12\t1700000000\tapp.log\t/var/log/app.log")
	if !ok {
		t.Fatal("expected list line to parse")
	}
	if entry.Name != "app.log" || entry.Path != "xdssh://prod/var/log/app.log" || entry.IsDir || entry.Size != 12 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !entry.Modified.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("unexpected modified time: %v", entry.Modified)
	}
}

func TestNetBoxExtractTarArchiveCopiesFilesAndDirectories(t *testing.T) {
	var archive bytes.Buffer
	tarWriter := tar.NewWriter(&archive)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "aa/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}); err != nil {
		t.Fatalf("write dir header: %v", err)
	}
	content := []byte("remote image")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "aa/1.png",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(content)),
	}); err != nil {
		t.Fatalf("write file header: %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatalf("write file body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	targetDir := t.TempDir()
	if err := xdfileExtractNetBoxTarArchive(&archive, targetDir); err != nil {
		t.Fatalf("extract remote archive: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(targetDir, "aa", "1.png"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("expected extracted content %q, got %q", string(content), string(data))
	}
}

func TestOpenNetBoxConnectionSyncsTerminalCwd(t *testing.T) {
	oldReadEntries := xdfileNetBoxReadEntriesFunc
	defer func() { xdfileNetBoxReadEntriesFunc = oldReadEntries }()
	xdfileNetBoxReadEntriesFunc = func(dir string, _ bool, _ xdfileSortMode) ([]xdfileEntry, error) {
		if dir != "xdssh://prod/var/www" {
			t.Fatalf("unexpected remote dir read: %q", dir)
		}
		return nil, nil
	}

	localDir := t.TempDir()
	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: localDir},
			{Label: "RIGHT", Cwd: localDir},
		},
		terminal: xdfileTerminal{Cwd: localDir},
		netboxConnections: []xdfileNetBoxConnection{
			{Name: "prod", Host: "example.com", RemotePath: "/var/www"},
		},
	}

	if cmd := m.openNetBoxConnection(0); cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected immediate netbox connection, got %T", msg)
		}
	}
	if m.panels[0].Cwd != "xdssh://prod/var/www" {
		t.Fatalf("expected panel to connect to remote path, got %q", m.panels[0].Cwd)
	}
	if m.terminal.Cwd != "xdssh://prod/var/www" {
		t.Fatalf("expected terminal cwd to follow remote panel, got %q", m.terminal.Cwd)
	}
}

func TestNetBoxShellPathAndCDUseRemotePaths(t *testing.T) {
	resolved, err := xdfileResolveShellPath("xdssh://prod/var/log", "../tmp")
	if err != nil {
		t.Fatalf("resolve remote shell path: %v", err)
	}
	if resolved != "xdssh://prod/var/tmp" {
		t.Fatalf("expected resolved remote path, got %q", resolved)
	}

	oldReadEntries := xdfileNetBoxReadEntriesFunc
	defer func() { xdfileNetBoxReadEntriesFunc = oldReadEntries }()
	xdfileNetBoxReadEntriesFunc = func(dir string, _ bool, _ xdfileSortMode) ([]xdfileEntry, error) {
		if dir != "xdssh://prod/var/tmp" {
			t.Fatalf("unexpected remote dir read: %q", dir)
		}
		return nil, nil
	}

	result, handled := xdfileRunManagedShellCommand("xdssh://prod/var/log", "cd ../tmp")
	if !handled {
		t.Fatalf("expected remote cd to be handled")
	}
	if result.Err != nil {
		t.Fatalf("remote cd failed: %v", result.Err)
	}
	if result.Dir != "xdssh://prod/var/tmp" || !result.SyncActivePanel {
		t.Fatalf("unexpected remote cd result: %+v", result)
	}
}

func TestNetBoxTerminalRunsRemoteShellCommands(t *testing.T) {
	oldConnections := xdfileNetBoxConnectionsSnapshot()
	oldStartScript := xdfileNetBoxStartScriptFunc
	defer func() {
		xdfileSetNetBoxConnectionsCache(oldConnections)
		xdfileNetBoxStartScriptFunc = oldStartScript
	}()

	xdfileSetNetBoxConnectionsCache([]xdfileNetBoxConnection{
		{Name: "prod", Host: "example.com", RemotePath: "/"},
	})
	xdfileNetBoxStartScriptFunc = func(connection xdfileNetBoxConnection, script string, cwd string, events chan tea.Msg) (func(), error) {
		if connection.Name != "prod" {
			t.Fatalf("unexpected connection: %+v", connection)
		}
		if script != "cd -- '/var'\nuname -a" {
			t.Fatalf("unexpected remote script: %q", script)
		}
		if cwd != "xdssh://prod/var" {
			t.Fatalf("unexpected remote cwd: %q", cwd)
		}
		go func() {
			events <- xdfileTerminalLineMsg{Line: "Linux prod 6.0", Finalize: true}
			events <- xdfileTerminalCommandDoneMsg{Cwd: cwd}
			close(events)
		}()
		return func() {}, nil
	}

	msg := xdfileExecuteCommandCmd("xdssh://prod/var", "uname -a", 80, 20)()
	start, ok := msg.(xdfileTerminalCommandStartMsg)
	if !ok {
		t.Fatalf("expected terminal start message, got %T", msg)
	}
	line, ok := (<-start.Events).(xdfileTerminalLineMsg)
	if !ok {
		t.Fatalf("expected streamed line")
	}
	if line.Line != "Linux prod 6.0" {
		t.Fatalf("unexpected remote output: %q", line.Line)
	}
	done, ok := (<-start.Events).(xdfileTerminalCommandDoneMsg)
	if !ok || done.Cwd != "xdssh://prod/var" || done.Err != nil {
		t.Fatalf("unexpected done message: %+v ok=%v", done, ok)
	}
}

func TestNetBoxTerminalPromptUsesConnectionUser(t *testing.T) {
	m := &xdfileModel{
		terminal: xdfileTerminal{
			Cwd:   "xdssh://prod/var",
			Input: xdfileNewManagedTerminalInput(),
		},
		netboxConnections: []xdfileNetBoxConnection{
			{Name: "prod", Host: "example.com", User: "root"},
		},
	}
	m.syncManagedTerminalPrompt()
	if got := m.terminal.Input.Prompt; got != "root@prod> " {
		t.Fatalf("expected remote user prompt, got %q", got)
	}

	m.terminal.Cwd = t.TempDir()
	m.syncManagedTerminalPrompt()
	if got := m.terminal.Input.Prompt; got != "XD> " {
		t.Fatalf("expected local prompt, got %q", got)
	}
}

func TestNetBoxMenuEditsPasswordsInConnectionForm(t *testing.T) {
	m := &xdfileModel{
		netboxConnections: []xdfileNetBoxConnection{
			{Name: "prod", Host: "example.com", User: "root", Password: "saved"},
		},
	}

	menu := m.netBoxMenuDefinition()
	for _, item := range menu.Items {
		if strings.Contains(strings.ToLower(item.Label), "password") {
			t.Fatalf("password editing should stay in connection form, got menu item %+v", item)
		}
	}

	connection, ok := m.netBoxConnectionAt(0)
	if !ok {
		t.Fatalf("expected connection")
	}
	m.openNetBoxConnectionForm(&connection)
	if len(m.modal.FormFields) < 9 {
		t.Fatalf("expected password fields in connection form")
	}
	if m.modal.FormFields[4].Label != "Password" || m.modal.FormFields[5].Label != "Save password" {
		t.Fatalf("expected password editing near the top of connection form, got labels %q and %q", m.modal.FormFields[4].Label, m.modal.FormFields[5].Label)
	}
	if got := m.modal.FormFields[5].Value(); got != "yes" {
		t.Fatalf("expected saved password connection to default Save password yes, got %q", got)
	}
}

func TestNetBoxExitDisconnectsActivePanel(t *testing.T) {
	result, handled := xdfileRunNetBoxManagedTerminalCommand("xdssh://prod/var", "exit")
	if !handled {
		t.Fatalf("expected exit to be handled")
	}
	if result.Err != nil {
		t.Fatalf("expected exit disconnect to succeed: %v", result.Err)
	}
	if !result.SyncActivePanel {
		t.Fatalf("expected exit to sync active panel")
	}
	if xdfileIsNetBoxPath(result.Dir) {
		t.Fatalf("expected exit to return to local path, got %q", result.Dir)
	}
	if !strings.Contains(result.Output, "Disconnected") {
		t.Fatalf("expected disconnect output, got %q", result.Output)
	}
}

func TestNetBoxReadRemoteEntriesUsesShortCache(t *testing.T) {
	oldConnections := xdfileNetBoxConnectionsSnapshot()
	oldRunScript := xdfileNetBoxRunScriptFunc
	oldCache := xdfileNetBoxEntryCache
	defer func() {
		xdfileSetNetBoxConnectionsCache(oldConnections)
		xdfileNetBoxRunScriptFunc = oldRunScript
		xdfileNetBoxEntryCache = oldCache
	}()

	xdfileNetBoxEntryCache = map[xdfileNetBoxEntryCacheKey]xdfileNetBoxEntryCacheValue{}
	xdfileSetNetBoxConnectionsCache([]xdfileNetBoxConnection{
		{Name: "prod", Host: "example.com", RemotePath: "/"},
	})
	calls := 0
	xdfileNetBoxRunScriptFunc = func(connection xdfileNetBoxConnection, script string) ([]byte, error) {
		calls++
		return []byte("f\t1\t1700000000\tfile.txt\t/var/file.txt\n"), nil
	}

	if _, err := xdfileNetBoxReadRemoteEntries("xdssh://prod/var", false, xdfileSortModeName); err != nil {
		t.Fatalf("read remote entries: %v", err)
	}
	if _, err := xdfileNetBoxReadRemoteEntries("xdssh://prod/var", false, xdfileSortModeName); err != nil {
		t.Fatalf("read cached remote entries: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one SSH listing due to cache, got %d", calls)
	}

	xdfileInvalidateNetBoxEntryCache("xdssh://prod/var")
	if _, err := xdfileNetBoxReadRemoteEntries("xdssh://prod/var", false, xdfileSortModeName); err != nil {
		t.Fatalf("read after invalidation: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected invalidation to force new SSH listing, got %d", calls)
	}
}
