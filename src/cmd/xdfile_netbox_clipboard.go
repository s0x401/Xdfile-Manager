package cmd

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type xdfileNetBoxDownloadSource struct {
	Parent string
	Name   string
}

type xdfileNetBoxFileInfo struct {
	Exists bool
	IsDir  bool
}

var (
	xdfileNetBoxDownloadPathsFunc     = xdfileNetBoxDownloadPaths
	xdfileNetBoxStatPathFunc          = xdfileNetBoxStatPath
	xdfileNetBoxUploadPathFunc        = xdfileNetBoxUploadPath
	xdfileNetBoxRemovePathFunc        = xdfileNetBoxRemovePath
	xdfileNetBoxUniquePasteTargetFunc = xdfileNetBoxUniquePasteCopyTarget
	xdfileRemoveAllFunc               = os.RemoveAll
)

func xdfileNetBoxDownloadPaths(paths []string) ([]string, string, error) {
	sources, connection, err := xdfileNetBoxDownloadSources(paths)
	if err != nil {
		return nil, "", err
	}
	if len(sources) == 0 {
		return nil, "", nil
	}

	cacheDir, err := os.MkdirTemp("", "xdfile-remote-clipboard-*")
	if err != nil {
		return nil, "", fmt.Errorf("create remote clipboard cache: %w", err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = xdfileRemoveAllFunc(cacheDir)
		}
	}()

	names := make([]string, 0, len(sources))
	for _, source := range sources {
		names = append(names, source.Name)
	}

	script := xdfileNetBoxTarScript(sources[0].Parent, names)
	if err := connection.streamSSHScriptStdout(script, func(reader io.Reader) error {
		return xdfileExtractNetBoxTarArchive(reader, cacheDir)
	}); err != nil {
		return nil, "", err
	}

	localPaths := make([]string, 0, len(sources))
	for _, source := range sources {
		localPaths = append(localPaths, filepath.Join(cacheDir, filepath.FromSlash(source.Name)))
	}
	cleanup = false
	return localPaths, cacheDir, nil
}

func xdfileNetBoxDownloadSources(paths []string) ([]xdfileNetBoxDownloadSource, xdfileNetBoxConnection, error) {
	sources := make([]xdfileNetBoxDownloadSource, 0, len(paths))
	profile := ""
	parent := ""
	for _, value := range paths {
		remote, ok := xdfileParseNetBoxPath(value)
		if !ok {
			return nil, xdfileNetBoxConnection{}, fmt.Errorf("remote clipboard copy requires SSH paths")
		}
		name := path.Base(remote.Path)
		if name == "." || name == "/" || name == "" {
			return nil, xdfileNetBoxConnection{}, fmt.Errorf("remote clipboard copy cannot copy %s", xdfileNetBoxPathLabel(value))
		}
		currentParent := path.Dir(remote.Path)
		if currentParent == "." {
			currentParent = "/"
		}
		if profile == "" {
			profile = remote.Profile
			parent = currentParent
		}
		if !strings.EqualFold(profile, remote.Profile) || parent != currentParent {
			return nil, xdfileNetBoxConnection{}, fmt.Errorf("remote clipboard copy requires one SSH directory selection")
		}
		sources = append(sources, xdfileNetBoxDownloadSource{
			Parent: currentParent,
			Name:   name,
		})
	}

	if len(sources) == 0 {
		return nil, xdfileNetBoxConnection{}, nil
	}
	connection, ok := xdfileFindNetBoxConnection(profile)
	if !ok {
		return nil, xdfileNetBoxConnection{}, fmt.Errorf("SSH connection %q is not configured", profile)
	}
	return sources, connection, nil
}

func xdfileNetBoxTarScript(parent string, names []string) string {
	var builder strings.Builder
	builder.WriteString("set -eu\n")
	builder.WriteString("cd -- ")
	builder.WriteString(xdfilePOSIXShellQuote(parent))
	builder.WriteString("\nset --")
	for _, name := range names {
		builder.WriteByte(' ')
		builder.WriteString(xdfilePOSIXShellQuote(name))
	}
	builder.WriteString("\ntar -cf - -- \"$@\"")
	return builder.String()
}

func (c xdfileNetBoxConnection) streamSSHScriptStdout(script string, consume func(io.Reader) error) error {
	if c.passwordForAuth() != "" {
		return c.streamPasswordSSHScriptStdout(script, consume)
	}
	return c.streamSystemSSHScriptStdout(script, consume)
}

func (c xdfileNetBoxConnection) runSSHScriptWithStdin(script string, stdin io.Reader) ([]byte, error) {
	if c.passwordForAuth() != "" {
		return c.runPasswordSSHScriptWithStdin(script, stdin)
	}
	return c.runSystemSSHScriptWithStdin(script, stdin)
}

func (c xdfileNetBoxConnection) streamSystemSSHScriptStdout(script string, consume func(io.Reader) error) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh client not found in PATH")
	}
	args, err := c.sshArgs("sh -lc " + xdfilePOSIXShellQuote(script))
	if err != nil {
		return err
	}

	cmd := exec.Command(sshPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start SSH copy for %s: %w", c.Name, err)
	}

	var stderrBuf bytes.Buffer
	stderrDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&stderrBuf, stderr)
		stderrDone <- copyErr
	}()

	consumeErr := consume(stdout)
	if consumeErr != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	stderrErr := <-stderrDone
	if consumeErr != nil {
		return fmt.Errorf("read remote clipboard archive for %s: %w", c.Name, consumeErr)
	}
	if stderrErr != nil {
		return fmt.Errorf("read SSH copy errors for %s: %w", c.Name, stderrErr)
	}
	if waitErr != nil {
		message := strings.TrimSpace(stderrBuf.String())
		if message == "" {
			message = waitErr.Error()
		}
		return fmt.Errorf("SSH copy failed for %s: %s", c.Name, message)
	}
	return nil
}

func (c xdfileNetBoxConnection) runSystemSSHScriptWithStdin(script string, stdin io.Reader) ([]byte, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("ssh client not found in PATH")
	}
	args, err := c.sshArgs("sh -lc " + xdfilePOSIXShellQuote(script))
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(sshPath, args...)
	cmd.Stdin = stdin
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

func (c xdfileNetBoxConnection) streamPasswordSSHScriptStdout(script string, consume func(io.Reader) error) error {
	client, err := c.dialPasswordSSH()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session failed for %s: %w", c.Name, err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}
	if err := session.Start("sh -lc " + xdfilePOSIXShellQuote(script)); err != nil {
		return fmt.Errorf("start SSH copy for %s: %w", c.Name, err)
	}

	var stderrBuf bytes.Buffer
	stderrDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&stderrBuf, stderr)
		stderrDone <- copyErr
	}()

	consumeErr := consume(stdout)
	if consumeErr != nil {
		_ = session.Close()
	}
	waitErr := session.Wait()
	stderrErr := <-stderrDone
	if consumeErr != nil {
		return fmt.Errorf("read remote clipboard archive for %s: %w", c.Name, consumeErr)
	}
	if stderrErr != nil {
		return fmt.Errorf("read SSH copy errors for %s: %w", c.Name, stderrErr)
	}
	if waitErr != nil {
		message := strings.TrimSpace(stderrBuf.String())
		if message == "" {
			message = waitErr.Error()
		}
		return fmt.Errorf("SSH copy failed for %s: %s", c.Name, message)
	}
	return nil
}

func (c xdfileNetBoxConnection) runPasswordSSHScriptWithStdin(script string, stdin io.Reader) ([]byte, error) {
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

	session.Stdin = stdin
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

func xdfileNetBoxStatPath(target string) (xdfileNetBoxFileInfo, error) {
	remote, connection, err := xdfileNetBoxPathConnection(target)
	if err != nil {
		return xdfileNetBoxFileInfo{}, err
	}
	output, err := connection.runSSHScript(
		"target=" + xdfilePOSIXShellQuote(remote.Path) + "\n" + strings.TrimSpace(`
if [ -e "$target" ]; then
  if [ -d "$target" ]; then
    printf 'd\n'
  else
    printf 'f\n'
  fi
else
  printf 'm\n'
fi
`),
	)
	if err != nil {
		return xdfileNetBoxFileInfo{}, err
	}
	switch strings.TrimSpace(string(output)) {
	case "d":
		return xdfileNetBoxFileInfo{Exists: true, IsDir: true}, nil
	case "f":
		return xdfileNetBoxFileInfo{Exists: true}, nil
	case "m", "":
		return xdfileNetBoxFileInfo{}, nil
	default:
		return xdfileNetBoxFileInfo{}, fmt.Errorf("unexpected SSH stat output for %s: %s", xdfileNetBoxPathLabel(target), strings.TrimSpace(string(output)))
	}
}

func xdfileNetBoxRemovePath(target string) error {
	remote, connection, err := xdfileNetBoxPathConnection(target)
	if err != nil {
		return err
	}
	_, err = connection.runSSHScript(
		"target=" + xdfilePOSIXShellQuote(remote.Path) + "\n" +
			`rm -rf -- "$target"`,
	)
	if err == nil {
		xdfileInvalidateNetBoxParentEntryCache(target)
		xdfileInvalidateNetBoxEntryCache(target)
	}
	return err
}

func xdfileNetBoxUploadPath(sourcePath string, target string) error {
	remote, connection, err := xdfileNetBoxPathConnection(target)
	if err != nil {
		return err
	}

	sourcePath = filepath.Clean(sourcePath)
	entryName := path.Base(remote.Path)
	if entryName == "." || entryName == "/" || entryName == "" {
		return fmt.Errorf("invalid remote paste target: %s", xdfileNetBoxPathLabel(target))
	}

	reader, writer := io.Pipe()
	writeDone := make(chan error, 1)
	go func() {
		tarWriter := tar.NewWriter(writer)
		err := xdfileWriteLocalPathTar(tarWriter, sourcePath, entryName)
		if closeErr := tarWriter.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
		writeDone <- err
	}()

	script := "dest=" + xdfilePOSIXShellQuote(path.Dir(remote.Path)) + "\n" + strings.TrimSpace(`
mkdir -p -- "$dest"
tar -xf - -C "$dest"
`)
	_, runErr := connection.runSSHScriptWithStdin(script, reader)
	_ = reader.Close()
	writeErr := <-writeDone
	if runErr != nil {
		return runErr
	}
	if writeErr != nil {
		return writeErr
	}
	xdfileInvalidateNetBoxParentEntryCache(target)
	xdfileInvalidateNetBoxEntryCache(target)
	return nil
}

func xdfileWriteLocalPathTar(writer *tar.Writer, sourcePath string, entryName string) error {
	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlinks are not supported for remote paste yet: %s", sourcePath)
	}
	if !sourceInfo.IsDir() {
		return xdfileWriteLocalTarEntry(writer, sourcePath, entryName, sourceInfo)
	}

	return filepath.WalkDir(sourcePath, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(currentPath)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported for remote paste yet: %s", currentPath)
		}

		tarName := entryName
		if currentPath != sourcePath {
			rel, err := filepath.Rel(sourcePath, currentPath)
			if err != nil {
				return err
			}
			tarName = path.Join(entryName, filepath.ToSlash(rel))
		}
		return xdfileWriteLocalTarEntry(writer, currentPath, tarName, info)
	})
}

func xdfileWriteLocalTarEntry(writer *tar.Writer, sourcePath string, tarName string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = strings.TrimPrefix(filepath.ToSlash(tarName), "/")
	if header.Name == "" || header.Name == "." {
		return fmt.Errorf("invalid tar entry name for %s", sourcePath)
	}
	if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
		header.Name += "/"
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func xdfileNetBoxUniquePasteCopyTarget(target string) (string, error) {
	remote, _, err := xdfileNetBoxPathConnection(target)
	if err != nil {
		return "", err
	}
	info, err := xdfileNetBoxStatPath(target)
	if err != nil {
		return "", err
	}
	if !info.Exists {
		return target, nil
	}

	dir := path.Dir(remote.Path)
	name := path.Base(remote.Path)
	base := name
	ext := ""
	if !info.IsDir {
		ext = path.Ext(name)
		base = strings.TrimSuffix(name, ext)
		if base == "" {
			base = name
			ext = ""
		}
	}

	for i := 2; i <= 1000; i++ {
		candidate := xdfileNetBoxURL(remote.Profile, path.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext)))
		candidateInfo, err := xdfileNetBoxStatPath(candidate)
		if err != nil {
			return "", err
		}
		if !candidateInfo.Exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to find a free remote paste target for %s", xdfileNetBoxPathLabel(target))
}

func xdfileExtractNetBoxTarArchive(reader io.Reader, targetDir string) error {
	targetRoot, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header == nil {
			continue
		}

		targetPath, err := xdfileNetBoxTarTargetPath(targetRoot, header.Name)
		if err != nil {
			return err
		}

		info := header.FileInfo()
		mode := info.Mode().Perm()
		switch header.Typeflag {
		case tar.TypeDir:
			if mode == 0 {
				mode = 0o755
			}
			if err := os.MkdirAll(targetPath, mode); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if mode == 0 {
				mode = 0o644
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			if err := xdfileExtractNetBoxTarFile(tarReader, targetPath, mode); err != nil {
				return err
			}
			if !header.ModTime.IsZero() {
				_ = os.Chtimes(targetPath, header.ModTime, header.ModTime)
			}
		case tar.TypeXHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			continue
		default:
			return fmt.Errorf("unsupported remote archive entry %s", header.Name)
		}
	}
	return nil
}

func xdfileExtractNetBoxTarFile(reader io.Reader, targetPath string, mode os.FileMode) error {
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func xdfileNetBoxTarTargetPath(root string, name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || name == "." {
		return "", fmt.Errorf("invalid remote archive entry")
	}
	cleanName := path.Clean(name)
	if cleanName == "." || strings.HasPrefix(cleanName, "../") || cleanName == ".." || path.IsAbs(cleanName) {
		return "", fmt.Errorf("unsafe remote archive entry: %s", name)
	}

	targetPath := filepath.Join(root, filepath.FromSlash(cleanName))
	if !xdfilePathWithinRoot(root, targetPath) {
		return "", fmt.Errorf("unsafe remote archive target: %s", name)
	}
	return targetPath, nil
}

func (m *xdfileModel) registerRemoteClipboardDir(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	m.remoteClipboardDirs = append(m.remoteClipboardDirs, dir)
}

func (m *xdfileModel) cleanupRemoteClipboardDirs() {
	for _, dir := range m.remoteClipboardDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		_ = xdfileRemoveAllFunc(dir)
	}
	m.remoteClipboardDirs = nil
}
