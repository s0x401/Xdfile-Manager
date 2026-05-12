package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	variable "github.com/s0x401/xdfile-manager/src/config"
	"github.com/s0x401/xdfile-manager/src/internal/utils"
	"golang.org/x/crypto/ssh"
)

const (
	xdfileNetBoxFileName            = "xdfile-netbox.json"
	xdfileNetBoxScheme              = "xdssh"
	xdfileNetBoxConnectActionPrefix = "netbox_connect:"
	xdfileNetBoxEditActionPrefix    = "netbox_edit:"
	xdfileNetBoxDeleteActionPrefix  = "netbox_delete:"
	xdfileNetBoxEntryCacheTTL       = 3 * time.Second
)

type xdfileNetBoxConnection struct {
	Name         string `json:"name"`
	Host         string `json:"host"`
	User         string `json:"user,omitempty"`
	Port         int    `json:"port,omitempty"`
	RemotePath   string `json:"remote_path,omitempty"`
	IdentityFile string `json:"identity_file,omitempty"`
	ExtraArgs    string `json:"extra_args,omitempty"`
	Password     string `json:"password,omitempty"`
}

type xdfileNetBoxPrefs struct {
	Connections []xdfileNetBoxConnection `json:"connections"`
}

type xdfileNetBoxPath struct {
	Profile string
	Path    string
}

type xdfileNetBoxEntryCacheKey struct {
	Dir        string
	ShowHidden bool
	SortMode   xdfileSortMode
}

type xdfileNetBoxEntryCacheValue struct {
	Entries []xdfileEntry
	At      time.Time
}

var (
	xdfileNetBoxConnectionsCache []xdfileNetBoxConnection
	xdfileNetBoxPasswordCache    = map[string]string{}
	xdfileNetBoxEntryCache       = map[xdfileNetBoxEntryCacheKey]xdfileNetBoxEntryCacheValue{}
	xdfileNetBoxReadEntriesFunc  = xdfileNetBoxReadRemoteEntries
	xdfileNetBoxRunScriptFunc    = func(connection xdfileNetBoxConnection, script string) ([]byte, error) {
		return connection.runSSHScript(script)
	}
	xdfileNetBoxStartScriptFunc = func(connection xdfileNetBoxConnection, script string, cwd string, events chan tea.Msg) (func(), error) {
		return connection.startSSHScriptStream(script, cwd, events)
	}
)

func xdfileNetBoxPrefsPath() string {
	return filepath.Join(variable.XdfileMainDir, xdfileNetBoxFileName)
}

func xdfileLoadNetBoxPrefs(path string) ([]xdfileNetBoxConnection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read SSH connection settings: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var prefs xdfileNetBoxPrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, fmt.Errorf("parse SSH connection settings: %w", err)
	}
	return xdfileNormalizeNetBoxConnections(prefs.Connections), nil
}

func xdfileSaveNetBoxPrefs(path string, connections []xdfileNetBoxConnection) error {
	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return fmt.Errorf("create SSH connection config directory: %w", err)
	}
	data, err := json.MarshalIndent(xdfileNetBoxPrefs{
		Connections: xdfileNormalizeNetBoxConnections(connections),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode SSH connection settings: %w", err)
	}
	if err := os.WriteFile(path, data, utils.ConfigFilePerm); err != nil {
		return fmt.Errorf("write SSH connection settings: %w", err)
	}
	return nil
}

func xdfileNormalizeNetBoxConnections(connections []xdfileNetBoxConnection) []xdfileNetBoxConnection {
	normalized := make([]xdfileNetBoxConnection, 0, len(connections))
	seen := map[string]struct{}{}
	for _, connection := range connections {
		connection = connection.normalized()
		if connection.Name == "" || connection.Host == "" {
			continue
		}
		key := strings.ToLower(connection.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, connection)
	}
	sort.SliceStable(normalized, func(i int, j int) bool {
		return strings.ToLower(normalized[i].Name) < strings.ToLower(normalized[j].Name)
	})
	return normalized
}

func (c xdfileNetBoxConnection) normalized() xdfileNetBoxConnection {
	c.Name = strings.TrimSpace(c.Name)
	c.Host = strings.TrimSpace(c.Host)
	c.User = strings.TrimSpace(c.User)
	c.IdentityFile = strings.TrimSpace(c.IdentityFile)
	c.ExtraArgs = strings.TrimSpace(c.ExtraArgs)
	if c.Port <= 0 {
		c.Port = 22
	}
	if c.RemotePath == "" {
		c.RemotePath = "/"
	}
	c.RemotePath = xdfileNetBoxCleanRemotePath(c.RemotePath)
	return c
}

func (c xdfileNetBoxConnection) validate() error {
	c = c.normalized()
	if c.Name == "" {
		return fmt.Errorf("connection name cannot be empty")
	}
	if strings.ContainsAny(c.Name, `/\:@?#%`) {
		return fmt.Errorf("connection name cannot contain / \\ : @ ? # or %%")
	}
	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func xdfileSetNetBoxConnectionsCache(connections []xdfileNetBoxConnection) {
	xdfileNetBoxConnectionsCache = xdfileNormalizeNetBoxConnections(connections)
}

func xdfileNetBoxConnectionsSnapshot() []xdfileNetBoxConnection {
	return append([]xdfileNetBoxConnection(nil), xdfileNetBoxConnectionsCache...)
}

func xdfileFindNetBoxConnectionIn(connections []xdfileNetBoxConnection, name string) (xdfileNetBoxConnection, bool) {
	name = strings.TrimSpace(name)
	for _, connection := range connections {
		if strings.EqualFold(connection.Name, name) {
			return connection.normalized(), true
		}
	}
	return xdfileNetBoxConnection{}, false
}

func xdfileNetBoxPasswordKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func xdfileSetNetBoxPassword(name string, password string) {
	key := xdfileNetBoxPasswordKey(name)
	if key == "" {
		return
	}
	if password == "" {
		delete(xdfileNetBoxPasswordCache, key)
		return
	}
	xdfileNetBoxPasswordCache[key] = password
}

func xdfileClearNetBoxPassword(name string) {
	delete(xdfileNetBoxPasswordCache, xdfileNetBoxPasswordKey(name))
}

func (c xdfileNetBoxConnection) passwordForAuth() string {
	if password := xdfileNetBoxPasswordCache[xdfileNetBoxPasswordKey(c.Name)]; password != "" {
		return password
	}
	return c.Password
}

func xdfileFindNetBoxConnection(name string) (xdfileNetBoxConnection, bool) {
	if connection, ok := xdfileFindNetBoxConnectionIn(xdfileNetBoxConnectionsCache, name); ok {
		return connection, true
	}
	connections, err := xdfileLoadNetBoxPrefs(xdfileNetBoxPrefsPath())
	if err != nil {
		return xdfileNetBoxConnection{}, false
	}
	xdfileSetNetBoxConnectionsCache(connections)
	return xdfileFindNetBoxConnectionIn(xdfileNetBoxConnectionsCache, name)
}

func xdfileNetBoxConnectAction(index int) xdfileAction {
	return xdfileAction(fmt.Sprintf("%s%d", xdfileNetBoxConnectActionPrefix, index))
}

func xdfileNetBoxEditAction(index int) xdfileAction {
	return xdfileAction(fmt.Sprintf("%s%d", xdfileNetBoxEditActionPrefix, index))
}

func xdfileNetBoxDeleteIndexedAction(index int) xdfileAction {
	return xdfileAction(fmt.Sprintf("%s%d", xdfileNetBoxDeleteActionPrefix, index))
}

func xdfileParseNetBoxConnectAction(action xdfileAction) (int, bool) {
	return xdfileParseCommandIndexedAction(action, xdfileNetBoxConnectActionPrefix)
}

func xdfileParseNetBoxEditAction(action xdfileAction) (int, bool) {
	return xdfileParseCommandIndexedAction(action, xdfileNetBoxEditActionPrefix)
}

func xdfileParseNetBoxDeleteIndexedAction(action xdfileAction) (int, bool) {
	return xdfileParseCommandIndexedAction(action, xdfileNetBoxDeleteActionPrefix)
}

func (m *xdfileModel) netBoxMenuDefinition() xdfileMenu {
	items := []xdfileButton{
		{Action: xdfileActionNetBoxNew, Key: "Ins", Label: "New SSH connection"},
	}
	if xdfileIsNetBoxPath(m.panels[m.activePanel].Cwd) {
		items = append(items, xdfileButton{Action: xdfileActionNetBoxDisconnect, Label: "Disconnect active panel"})
	}
	connections := m.netboxConnections
	if len(connections) > 0 {
		items = append(items, xdfileButton{Label: "Connections", Disabled: true})
	}
	for i, connection := range connections {
		label := connection.Name
		if connection.User != "" {
			label += "  " + connection.User + "@" + connection.Host
		} else {
			label += "  " + connection.Host
		}
		items = append(items,
			xdfileButton{Action: xdfileNetBoxConnectAction(i), Label: label},
			xdfileButton{Action: xdfileNetBoxEditAction(i), Label: "Edit " + connection.Name},
			xdfileButton{Action: xdfileNetBoxDeleteIndexedAction(i), Label: "Delete " + connection.Name},
		)
	}
	return xdfileMenu{Action: xdfileActionNetBoxMenu, Label: "NetBox", Items: items}
}

func (m *xdfileModel) netBoxConnectionAt(index int) (xdfileNetBoxConnection, bool) {
	if index < 0 || index >= len(m.netboxConnections) {
		return xdfileNetBoxConnection{}, false
	}
	return m.netboxConnections[index].normalized(), true
}

func (m *xdfileModel) netBoxConnectionByName(name string) (xdfileNetBoxConnection, bool) {
	return xdfileFindNetBoxConnectionIn(m.netboxConnections, name)
}

func (m *xdfileModel) openNetBoxConnection(index int) tea.Cmd {
	connection, ok := m.netBoxConnectionAt(index)
	if !ok {
		m.setStatus("SSH connection not found")
		return nil
	}
	target := xdfileNetBoxURL(connection.Name, connection.RemotePath)
	if err := m.changePanelDir(m.activePanel, target, ""); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.setStatus("Connected %s to %s", m.panels[m.activePanel].Label, connection.Name)
	return m.syncTerminalToPanel(m.activePanel)
}

func (m *xdfileModel) disconnectNetBoxPanel() tea.Cmd {
	if !xdfileIsNetBoxPath(m.panels[m.activePanel].Cwd) {
		m.setStatus("Active panel is not connected")
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	if err := m.changePanelDir(m.activePanel, cwd, ""); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.setStatus("Disconnected active panel")
	return m.syncTerminalToPanel(m.activePanel)
}

func (m *xdfileModel) openNetBoxConnectionForm(existing *xdfileNetBoxConnection) tea.Cmd {
	connection := xdfileNetBoxConnection{Port: 22, RemotePath: "/"}
	action := xdfileActionNetBoxSave
	oldName := ""
	title := "New SSH Connection"
	if existing != nil {
		connection = existing.normalized()
		oldName = connection.Name
		title = "Edit SSH Connection"
	}
	savePassword := "no"
	if connection.Password != "" {
		savePassword = "yes"
	}
	fields := []xdfileModalField{
		m.newModalFormField("Name", "work-server", connection.Name),
		m.newModalFormField("Host", "example.com", connection.Host),
		m.newModalFormField("Port", "22", strconv.Itoa(connection.Port)),
		m.newModalFormField("User", "optional", connection.User),
		m.newModalPasswordField("Password", "optional; blank keeps saved password", ""),
		m.newModalFormField("Save password", "yes/no", savePassword),
		m.newModalFormField("Remote path", "/", connection.RemotePath),
		m.newModalFormField("Identity file", "optional: ~/.ssh/id_ed25519", connection.IdentityFile),
		m.newModalFormField("Extra ssh args", "optional: -J jump-host", connection.ExtraArgs),
	}
	m.openFormModal(
		action,
		title,
		"Leave password blank to use key files, ssh-agent, or an existing saved password. Saved passwords are plain text.",
		fields,
	)
	m.modal.SourcePath = oldName
	return nil
}

func (m *xdfileModel) saveNetBoxConnectionFromModal() tea.Cmd {
	if len(m.modal.FormFields) < 9 {
		m.setStatus("SSH connection form is incomplete")
		return nil
	}
	port, err := strconv.Atoi(strings.TrimSpace(m.modal.FormFields[2].Value()))
	if err != nil {
		m.setStatusErr(fmt.Errorf("invalid SSH port: %w", err))
		return nil
	}
	connection := xdfileNetBoxConnection{
		Name:         m.modal.FormFields[0].Value(),
		Host:         m.modal.FormFields[1].Value(),
		Port:         port,
		User:         m.modal.FormFields[3].Value(),
		RemotePath:   m.modal.FormFields[6].Value(),
		IdentityFile: m.modal.FormFields[7].Value(),
		ExtraArgs:    m.modal.FormFields[8].Value(),
	}.normalized()
	if err := connection.validate(); err != nil {
		m.setStatusErr(err)
		return nil
	}

	oldName := strings.TrimSpace(m.modal.SourcePath)
	oldConnection, oldConnectionOK := m.netBoxConnectionByName(oldName)
	password := m.modal.FormFields[4].Value()
	savePassword, err := xdfileParseNetBoxSavePassword(m.modal.FormFields[5].Value())
	if err != nil {
		m.setStatusErr(err)
		return nil
	}
	switch {
	case savePassword && password != "":
		connection.Password = password
		xdfileSetNetBoxPassword(connection.Name, password)
	case savePassword && oldConnectionOK:
		connection.Password = oldConnection.Password
	case !savePassword && password != "":
		xdfileSetNetBoxPassword(connection.Name, password)
	default:
		xdfileClearNetBoxPassword(connection.Name)
	}
	if oldName != "" && !strings.EqualFold(oldName, connection.Name) {
		xdfileClearNetBoxPassword(oldName)
	}

	updated := make([]xdfileNetBoxConnection, 0, len(m.netboxConnections)+1)
	replaced := false
	for _, current := range m.netboxConnections {
		if strings.EqualFold(current.Name, oldName) || strings.EqualFold(current.Name, connection.Name) {
			if !replaced {
				updated = append(updated, connection)
				replaced = true
			}
			continue
		}
		updated = append(updated, current)
	}
	if !replaced {
		updated = append(updated, connection)
	}
	if err := m.saveNetBoxConnections(updated); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.closeModal()
	m.setStatus("Saved SSH connection %s", connection.Name)
	return nil
}

func xdfileParseNetBoxSavePassword(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "no", "n", "false", "0", "off", "否", "不", "不保存":
		return false, nil
	case "yes", "y", "true", "1", "on", "save", "是", "保存":
		return true, nil
	default:
		return false, fmt.Errorf("Save password must be yes or no")
	}
}

func (m *xdfileModel) confirmDeleteNetBoxConnection(index int) tea.Cmd {
	connection, ok := m.netBoxConnectionAt(index)
	if !ok {
		m.setStatus("SSH connection not found")
		return nil
	}
	m.modal = xdfileModal{
		Kind:        xdfileModalConfirm,
		Title:       "Delete SSH Connection",
		Description: fmt.Sprintf("Delete saved SSH connection %s?", connection.Name),
		Action:      xdfileActionNetBoxDelete,
		Input:       m.modalInputModel(),
		SourcePath:  connection.Name,
	}
	m.setStatus("Press Enter to delete or Esc to cancel")
	return nil
}

func (m *xdfileModel) deleteNetBoxConnectionFromModal() tea.Cmd {
	name := strings.TrimSpace(m.modal.SourcePath)
	if name == "" {
		m.closeModal()
		return nil
	}
	updated := make([]xdfileNetBoxConnection, 0, len(m.netboxConnections))
	for _, connection := range m.netboxConnections {
		if strings.EqualFold(connection.Name, name) {
			continue
		}
		updated = append(updated, connection)
	}
	xdfileClearNetBoxPassword(name)
	if err := m.saveNetBoxConnections(updated); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.closeModal()
	m.setStatus("Deleted SSH connection %s", name)
	return nil
}

func (m *xdfileModel) saveNetBoxPrefs() error {
	if m.netboxFile == "" {
		return nil
	}
	return xdfileSaveNetBoxPrefs(m.netboxFile, m.netboxConnections)
}

func (m *xdfileModel) setNetBoxConnections(connections []xdfileNetBoxConnection) {
	m.netboxConnections = xdfileNormalizeNetBoxConnections(connections)
	xdfileSetNetBoxConnectionsCache(m.netboxConnections)
}

func (m *xdfileModel) saveNetBoxConnections(connections []xdfileNetBoxConnection) error {
	m.setNetBoxConnections(connections)
	return m.saveNetBoxPrefs()
}

func xdfileIsNetBoxPath(value string) bool {
	_, ok := xdfileParseNetBoxPath(value)
	return ok
}

func xdfileParseNetBoxPath(value string) (xdfileNetBoxPath, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != xdfileNetBoxScheme || parsed.Host == "" {
		return xdfileNetBoxPath{}, false
	}
	return xdfileNetBoxPath{
		Profile: parsed.Host,
		Path:    xdfileNetBoxCleanRemotePath(parsed.Path),
	}, true
}

func xdfileNetBoxURL(profile string, remotePath string) string {
	profile = strings.TrimSpace(profile)
	return (&url.URL{
		Scheme: xdfileNetBoxScheme,
		Host:   profile,
		Path:   xdfileNetBoxCleanRemotePath(remotePath),
	}).String()
}

func xdfileNetBoxCleanRemotePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	cleaned := path.Clean(value)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func xdfileNetBoxParent(value string) (string, bool) {
	remote, ok := xdfileParseNetBoxPath(value)
	if !ok {
		return "", false
	}
	parent := path.Dir(remote.Path)
	if parent == "." {
		parent = "/"
	}
	return xdfileNetBoxURL(remote.Profile, parent), parent != remote.Path
}

func xdfileNetBoxJoin(parent string, name string) (string, bool) {
	remote, ok := xdfileParseNetBoxPath(parent)
	if !ok {
		return "", false
	}
	return xdfileNetBoxURL(remote.Profile, path.Join(remote.Path, name)), true
}

func xdfileNetBoxPathLabel(value string) string {
	remote, ok := xdfileParseNetBoxPath(value)
	if !ok {
		return value
	}
	return remote.Profile + ":" + remote.Path
}

func xdfileNetBoxPathsEqual(a string, b string) bool {
	left, leftOK := xdfileParseNetBoxPath(a)
	right, rightOK := xdfileParseNetBoxPath(b)
	if !leftOK || !rightOK {
		return false
	}
	return strings.EqualFold(left.Profile, right.Profile) && left.Path == right.Path
}

func xdfileNetBoxReadRemoteEntries(dir string, showHidden bool, sortMode xdfileSortMode) ([]xdfileEntry, error) {
	remote, ok := xdfileParseNetBoxPath(dir)
	if !ok {
		return nil, fmt.Errorf("invalid SSH panel path: %s", dir)
	}
	sortMode = xdfileNormalizeSortMode(sortMode)
	cacheKey := xdfileNetBoxEntryCacheKey{
		Dir:        xdfileNetBoxURL(remote.Profile, remote.Path),
		ShowHidden: showHidden,
		SortMode:   sortMode,
	}
	if cached, ok := xdfileNetBoxEntryCache[cacheKey]; ok && time.Since(cached.At) < xdfileNetBoxEntryCacheTTL {
		return xdfileCloneNetBoxEntries(cached.Entries), nil
	}

	connection, ok := xdfileFindNetBoxConnection(remote.Profile)
	if !ok {
		return nil, fmt.Errorf("SSH connection %q is not configured", remote.Profile)
	}

	lines, err := connection.listRemoteDirectory(remote.Path)
	if err != nil {
		return nil, err
	}

	entries := make([]xdfileEntry, 0, len(lines)+1)
	if parent, ok := xdfileNetBoxParent(dir); ok {
		entries = append(entries, xdfileEntry{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			IsParent: true,
			sortName: "..",
		})
	}

	buffer := make([]xdfileEntry, 0, len(lines))
	for _, line := range lines {
		entry, ok := xdfileNetBoxParseListLine(remote.Profile, line)
		if !ok {
			continue
		}
		if !showHidden && strings.HasPrefix(entry.Name, ".") {
			continue
		}
		buffer = append(buffer, entry)
	}
	sort.Slice(buffer, func(i int, j int) bool {
		if buffer[i].IsDir != buffer[j].IsDir {
			return buffer[i].IsDir
		}
		return xdfileEntryLess(buffer[i], buffer[j], sortMode)
	})
	entries = append(entries, buffer...)
	xdfileNetBoxEntryCache[cacheKey] = xdfileNetBoxEntryCacheValue{
		Entries: xdfileCloneNetBoxEntries(entries),
		At:      time.Now(),
	}
	return entries, nil
}

func xdfileCloneNetBoxEntries(entries []xdfileEntry) []xdfileEntry {
	return append([]xdfileEntry(nil), entries...)
}

func xdfileInvalidateNetBoxEntryCache(value string) {
	if value == "" {
		xdfileNetBoxEntryCache = map[xdfileNetBoxEntryCacheKey]xdfileNetBoxEntryCacheValue{}
		return
	}
	remote, ok := xdfileParseNetBoxPath(value)
	if !ok {
		return
	}
	target := xdfileNetBoxURL(remote.Profile, remote.Path)
	for key := range xdfileNetBoxEntryCache {
		if xdfileNetBoxPathsEqual(key.Dir, target) {
			delete(xdfileNetBoxEntryCache, key)
		}
	}
}

func xdfileInvalidateNetBoxParentEntryCache(value string) {
	remote, ok := xdfileParseNetBoxPath(value)
	if !ok {
		return
	}
	xdfileInvalidateNetBoxEntryCache(xdfileNetBoxURL(remote.Profile, path.Dir(remote.Path)))
}

func xdfileNetBoxParseListLine(profile string, line string) (xdfileEntry, bool) {
	parts := strings.SplitN(line, "\t", 5)
	if len(parts) < 5 {
		return xdfileEntry{}, false
	}
	remotePath := xdfileNetBoxCleanRemotePath(parts[4])
	name := path.Base(remotePath)
	if name == "." || name == "/" || name == "" {
		return xdfileEntry{}, false
	}
	size, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	mtime, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	modified := time.Unix(mtime, 0)
	if mtime <= 0 {
		modified = time.Time{}
	}
	isDir := strings.TrimSpace(parts[0]) == "d"
	return xdfileEntry{
		Name:     name,
		Path:     xdfileNetBoxURL(profile, remotePath),
		IsDir:    isDir,
		Size:     size,
		Modified: modified,
		sortName: strings.ToLower(name),
		sortExt:  xdfileSortExtension(name),
	}, true
}

func (c xdfileNetBoxConnection) listRemoteDirectory(remotePath string) ([]string, error) {
	script := xdfileNetBoxListScript(remotePath)
	output, err := xdfileNetBoxRunScriptFunc(c, script)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(output), "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func xdfileNetBoxMakeDir(target string) error {
	remote, connection, err := xdfileNetBoxPathConnection(target)
	if err != nil {
		return err
	}
	_, err = connection.runSSHScript(
		"target=" + xdfilePOSIXShellQuote(remote.Path) + "\n" +
			`mkdir -p -- "$target"`,
	)
	if err == nil {
		xdfileInvalidateNetBoxParentEntryCache(target)
		xdfileInvalidateNetBoxEntryCache(target)
	}
	return err
}

func xdfileNetBoxRenamePath(source string, target string) error {
	sourceRemote, sourceConnection, err := xdfileNetBoxPathConnection(source)
	if err != nil {
		return err
	}
	targetRemote, ok := xdfileParseNetBoxPath(target)
	if !ok {
		return fmt.Errorf("remote rename requires a remote target")
	}
	if !strings.EqualFold(sourceRemote.Profile, targetRemote.Profile) {
		return fmt.Errorf("remote rename cannot cross SSH connections")
	}
	_, err = sourceConnection.runSSHScript(
		"src=" + xdfilePOSIXShellQuote(sourceRemote.Path) + "\n" +
			"dst=" + xdfilePOSIXShellQuote(targetRemote.Path) + "\n" + strings.TrimSpace(`
if [ -e "$dst" ]; then
  echo "target already exists: $dst" >&2
  exit 3
fi
mv -- "$src" "$dst"
`),
	)
	if err == nil {
		xdfileInvalidateNetBoxParentEntryCache(source)
		xdfileInvalidateNetBoxParentEntryCache(target)
		xdfileInvalidateNetBoxEntryCache(source)
		xdfileInvalidateNetBoxEntryCache(target)
	}
	return err
}

func xdfileNetBoxPathConnection(value string) (xdfileNetBoxPath, xdfileNetBoxConnection, error) {
	remote, ok := xdfileParseNetBoxPath(value)
	if !ok {
		return xdfileNetBoxPath{}, xdfileNetBoxConnection{}, fmt.Errorf("invalid SSH panel path: %s", value)
	}
	connection, ok := xdfileFindNetBoxConnection(remote.Profile)
	if !ok {
		return xdfileNetBoxPath{}, xdfileNetBoxConnection{}, fmt.Errorf("SSH connection %q is not configured", remote.Profile)
	}
	return remote, connection, nil
}

func xdfileRunNetBoxTerminalCommand(dir string, command string) xdfileTerminalResultMsg {
	command = strings.TrimSpace(command)
	result := xdfileTerminalResultMsg{
		Command: command,
		Dir:     dir,
	}
	if command == "" {
		return result
	}

	if nextDir, handled, err := xdfileBuiltinRemoteCD(dir, command); handled {
		result.Dir = nextDir
		result.Err = err
		result.SyncActivePanel = err == nil
		if err == nil {
			result.Output = xdfileTerminalPromptPathStyle.Render(xdfileNetBoxPathLabel(nextDir))
		}
		return result
	}

	remote, connection, err := xdfileNetBoxPathConnection(dir)
	if err != nil {
		result.Err = err
		return result
	}

	remoteCommand := xdfileNormalizeNetBoxTerminalCommand(command, remote.Path)
	output, err := xdfileNetBoxRunScriptFunc(
		connection,
		"cd -- "+xdfilePOSIXShellQuote(remote.Path)+"\n"+remoteCommand,
	)
	text := strings.TrimRight(
		xdfileSanitizeManagedTerminalText(
			strings.ReplaceAll(xdfileDecodeCommandOutput(output), "\r\n", "\n"),
		),
		"\n",
	)
	result.Output = text
	result.Err = err
	return result
}

func xdfileRunNetBoxManagedTerminalCommand(dir string, command string) (xdfileTerminalResultMsg, bool) {
	command = strings.TrimSpace(command)
	result := xdfileTerminalResultMsg{
		Command: command,
		Dir:     dir,
	}
	if command == "" {
		return result, true
	}
	switch strings.ToLower(command) {
	case "exit", "logout":
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		result.Dir = cwd
		result.Err = err
		result.SyncActivePanel = err == nil
		if err == nil {
			result.Output = "Disconnected SSH panel"
		}
		return result, true
	}
	if nextDir, handled, err := xdfileBuiltinRemoteCD(dir, command); handled {
		result.Dir = nextDir
		result.Err = err
		result.SyncActivePanel = err == nil
		if err == nil {
			result.Output = xdfileTerminalPromptPathStyle.Render(xdfileNetBoxPathLabel(nextDir))
		}
		return result, true
	}
	return result, false
}

func xdfileStartNetBoxStreamingTerminalCommand(dir string, command string, events chan tea.Msg) (func(), error) {
	remote, connection, err := xdfileNetBoxPathConnection(dir)
	if err != nil {
		return nil, err
	}
	remoteCommand := xdfileNormalizeNetBoxTerminalCommand(command, remote.Path)
	script := "cd -- " + xdfilePOSIXShellQuote(remote.Path) + "\n" + remoteCommand
	return xdfileNetBoxStartScriptFunc(connection, script, dir, events)
}

func xdfileNormalizeNetBoxTerminalCommand(command string, remotePath string) string {
	if xdfileContainsShellOperators(command) {
		return command
	}
	parsed, err := xdfileParseShellCommand(command)
	if err != nil || parsed.Name == "" || len(parsed.Args) > 0 {
		return command
	}
	switch strings.ToLower(parsed.Name) {
	case "pwd":
		return "printf '%s\\n' " + xdfilePOSIXShellQuote(remotePath)
	case "ll":
		return "ls -la"
	case "la":
		return "ls -a"
	default:
		return command
	}
}

func (c xdfileNetBoxConnection) runSSHScript(script string) ([]byte, error) {
	if c.passwordForAuth() != "" {
		return c.runPasswordSSHScript(script)
	}
	return c.runSystemSSHScript(script)
}

func (c xdfileNetBoxConnection) startSSHScriptStream(script string, cwd string, events chan tea.Msg) (func(), error) {
	if c.passwordForAuth() != "" {
		return c.startPasswordSSHScriptStream(script, cwd, events)
	}
	return c.startSystemSSHScriptStream(script, cwd, events)
}

func (c xdfileNetBoxConnection) runSystemSSHScript(script string) ([]byte, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("ssh client not found in PATH")
	}
	args, err := c.sshArgs("sh -lc " + xdfilePOSIXShellQuote(script))
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(sshPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return output, fmt.Errorf("SSH command failed for %s: %s", c.Name, message)
	}
	return output, nil
}

func (c xdfileNetBoxConnection) startSystemSSHScriptStream(script string, cwd string, events chan tea.Msg) (func(), error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("ssh client not found in PATH")
	}
	args, err := c.sshArgs("sh -lc " + xdfilePOSIXShellQuote(script))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, sshPath, args...)
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		cancel()
		_ = reader.Close()
		_ = writer.Close()
		return nil, fmt.Errorf("start SSH command for %s: %w", c.Name, err)
	}

	stop := sync.OnceFunc(func() {
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = reader.Close()
		_ = writer.Close()
	})

	go func() {
		defer cancel()
		defer close(events)

		readDone := make(chan error, 1)
		go func() {
			readDone <- xdfileStreamCommandOutput(reader, func(line string, rewrite bool, finalize bool) {
				events <- xdfileTerminalLineMsg{Line: line, Rewrite: rewrite, Finalize: finalize}
			})
		}()

		err := cmd.Wait()
		_ = writer.Close()
		readErr := <-readDone
		if err == nil && readErr != nil {
			err = readErr
		}

		canceled := errors.Is(ctx.Err(), context.Canceled)
		if canceled {
			err = nil
		} else if err != nil {
			err = fmt.Errorf("SSH command failed for %s: %w", c.Name, err)
		}
		events <- xdfileTerminalCommandDoneMsg{
			Cwd:      cwd,
			Err:      err,
			Canceled: canceled,
		}
	}()

	return stop, nil
}

func (c xdfileNetBoxConnection) passwordSSHClientConfig() (*ssh.ClientConfig, error) {
	c = c.normalized()
	if c.ExtraArgs != "" {
		return nil, fmt.Errorf("password SSH does not support Extra ssh args yet")
	}
	user := c.sshUser()
	if user == "" {
		return nil, fmt.Errorf("SSH user is required for password login")
	}
	password := c.passwordForAuth()
	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}, nil
}

func (c xdfileNetBoxConnection) dialPasswordSSH() (*ssh.Client, error) {
	c = c.normalized()
	config, err := c.passwordSSHClientConfig()
	if err != nil {
		return nil, err
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)), config)
	if err != nil {
		return nil, fmt.Errorf("SSH password login failed for %s: %w", c.Name, err)
	}
	return client, nil
}

func (c xdfileNetBoxConnection) runPasswordSSHScript(script string) ([]byte, error) {
	client, err := c.dialPasswordSSH()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("SSH session failed for %s: %w", c.Name, err)
	}
	defer session.Close()

	output, err := session.CombinedOutput("sh -lc " + xdfilePOSIXShellQuote(script))
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return output, fmt.Errorf("SSH command failed for %s: %s", c.Name, message)
	}
	return output, nil
}

func (c xdfileNetBoxConnection) startPasswordSSHScriptStream(script string, cwd string, events chan tea.Msg) (func(), error) {
	client, err := c.dialPasswordSSH()
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("SSH session failed for %s: %w", c.Name, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	session.Stdout = writer
	session.Stderr = writer

	if err := session.Start("sh -lc " + xdfilePOSIXShellQuote(script)); err != nil {
		cancel()
		_ = session.Close()
		_ = client.Close()
		_ = reader.Close()
		_ = writer.Close()
		return nil, fmt.Errorf("start SSH command for %s: %w", c.Name, err)
	}

	stop := sync.OnceFunc(func() {
		cancel()
		_ = session.Close()
		_ = client.Close()
		_ = reader.Close()
		_ = writer.Close()
	})

	go func() {
		defer cancel()
		defer close(events)
		defer client.Close()
		defer session.Close()

		readDone := make(chan error, 1)
		go func() {
			readDone <- xdfileStreamCommandOutput(reader, func(line string, rewrite bool, finalize bool) {
				events <- xdfileTerminalLineMsg{Line: line, Rewrite: rewrite, Finalize: finalize}
			})
		}()

		err := session.Wait()
		_ = writer.Close()
		readErr := <-readDone
		if err == nil && readErr != nil {
			err = readErr
		}

		canceled := errors.Is(ctx.Err(), context.Canceled)
		if canceled {
			err = nil
		} else if err != nil {
			err = fmt.Errorf("SSH command failed for %s: %w", c.Name, err)
		}
		events <- xdfileTerminalCommandDoneMsg{
			Cwd:      cwd,
			Err:      err,
			Canceled: canceled,
		}
	}()

	return stop, nil
}

func (c xdfileNetBoxConnection) sshUser() string {
	if c.User != "" {
		return c.User
	}
	if user := strings.TrimSpace(os.Getenv("USER")); user != "" {
		return user
	}
	return strings.TrimSpace(os.Getenv("USERNAME"))
}

func (c xdfileNetBoxConnection) sshArgs(remoteCommand string) ([]string, error) {
	c = c.normalized()
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(c.Port),
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.ExtraArgs != "" {
		extra, err := xdfileSplitShellWords(c.ExtraArgs)
		if err != nil {
			return nil, fmt.Errorf("parse SSH extra args: %w", err)
		}
		args = append(args, extra...)
	}
	target := c.Host
	if c.User != "" {
		target = c.User + "@" + c.Host
	}
	args = append(args, target, remoteCommand)
	return args, nil
}

func xdfileNetBoxListScript(remotePath string) string {
	quoted := xdfilePOSIXShellQuote(xdfileNetBoxCleanRemotePath(remotePath))
	return "dir=" + quoted + "\n" + strings.TrimSpace(`
if [ ! -d "$dir" ]; then
  echo "not a directory: $dir" >&2
  exit 2
fi
for p in "$dir"/* "$dir"/.[!.]* "$dir"/..?*; do
  [ -e "$p" ] || continue
  base=${p##*/}
  [ "$base" = "." ] && continue
  [ "$base" = ".." ] && continue
  if [ -d "$p" ]; then
    kind=d
    size=0
  else
    kind=f
    size=$(wc -c < "$p" 2>/dev/null || printf 0)
  fi
  mtime=$(stat -c %Y "$p" 2>/dev/null || date -r "$p" +%s 2>/dev/null || printf 0)
  printf '%s\t%s\t%s\t%s\t%s\n' "$kind" "$size" "$mtime" "$base" "$p"
done
`)
}

func xdfilePOSIXShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func xdfilePathIsRemote(value string) bool {
	return xdfileIsNetBoxPath(value)
}

func xdfileDisplayPath(value string) string {
	if xdfileIsNetBoxPath(value) {
		return xdfileNetBoxPathLabel(value)
	}
	return value
}

func xdfileParentPath(value string) string {
	if parent, ok := xdfileNetBoxParent(value); ok {
		return parent
	}
	return filepath.Dir(value)
}

func xdfileJoinPath(parent string, name string) string {
	if joined, ok := xdfileNetBoxJoin(parent, name); ok {
		return joined
	}
	return filepath.Join(parent, name)
}
