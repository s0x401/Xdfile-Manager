package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/quick"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	vt "github.com/charmbracelet/x/vt"
	"github.com/s0x401/xdfile-manager/src/internal/common"
	platformsystem "github.com/s0x401/xdfile-manager/src/internal/platform/system"
	"github.com/s0x401/xdfile-manager/src/internal/utils"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

const (
	xdfilePreviewLimit      = 128 * 1024
	xdfilePreviewLineLimit  = 180
	xdfilePreviewLineWidth  = 240
	xdfileArchiveEntryLimit = 160
	xdfileHexBytesPerLine   = 16
	xdfileHexColumnWidth    = 48
	xdfileHexLineLimit      = 80
	xdfilePDFMetadataLimit  = 160
)

var xdfileOpenPathFunc = xdfileOpenPath
var xdfileReadClipboardPathsFunc = xdfileReadClipboardPaths
var xdfileReadClipboardCutFunc = xdfileReadClipboardCut
var xdfileWriteClipboardPathsFunc = xdfileWriteClipboardPaths
var xdfileIsDetachedGUIExecutableFunc = xdfileIsWindowsGUIExecutable

var xdfileArchivePreviewLabels = map[string]string{
	".zip":  "ZIP archive",
	".jar":  "JAR archive",
	".apk":  "APK package",
	".epub": "EPUB book",
	".cbz":  "Comic archive",
	".docx": "Word document (OpenXML)",
	".xlsx": "Excel workbook (OpenXML)",
	".pptx": "PowerPoint presentation (OpenXML)",
	".odt":  "OpenDocument text",
	".ods":  "OpenDocument spreadsheet",
	".odp":  "OpenDocument presentation",
	".vsix": "VSIX extension",
}

var xdfilePDFMetadataKeys = [...]string{
	"Title",
	"Author",
	"Creator",
	"Producer",
	"CreationDate",
	"ModDate",
}

var xdfileAudioExtensions = xdfileStringSet{
	".mp3":  {},
	".wav":  {},
	".flac": {},
	".ogg":  {},
	".m4a":  {},
	".aac":  {},
	".wma":  {},
	".opus": {},
}

func xdfileReadEntries(dir string, showHidden bool, sortMode xdfileSortMode) ([]xdfileEntry, error) {
	if xdfileIsNetBoxPath(dir) {
		return xdfileNetBoxReadEntriesFunc(dir, showHidden, sortMode)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", abs)
	}

	items, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	sortMode = xdfileNormalizeSortMode(sortMode)
	entries := make([]xdfileEntry, 0, len(items)+1)
	parent := filepath.Dir(abs)
	if parent != abs {
		entries = append(entries, xdfileEntry{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			IsParent: true,
			sortName: "..",
		})
	}

	buffer := make([]xdfileEntry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if strings.HasPrefix(name, xdfileDeleteUndoDirPrefix) || strings.HasPrefix(name, xdfileLegacyDeleteUndoDirPrefix) {
			continue
		}
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		itemInfo, infoErr := item.Info()
		if infoErr != nil {
			continue
		}

		isDir := item.IsDir()
		sortName := strings.ToLower(name)
		sortExt := ""
		if !isDir && sortMode == xdfileSortModeExt {
			sortExt = xdfileSortExtension(name)
		}

		buffer = append(buffer, xdfileEntry{
			Name:     name,
			Path:     filepath.Join(abs, name),
			IsDir:    isDir,
			Size:     itemInfo.Size(),
			Modified: itemInfo.ModTime(),
			sortName: sortName,
			sortExt:  sortExt,
		})
	}

	sort.Slice(buffer, func(i int, j int) bool {
		if buffer[i].IsDir != buffer[j].IsDir {
			return buffer[i].IsDir
		}
		return xdfileEntryLess(buffer[i], buffer[j], sortMode)
	})

	entries = append(entries, buffer...)
	return entries, nil
}

func xdfileEntryLess(left xdfileEntry, right xdfileEntry, sortMode xdfileSortMode) bool {
	leftName := xdfileEntrySortName(left)
	rightName := xdfileEntrySortName(right)

	if left.IsDir || right.IsDir || sortMode != xdfileSortModeExt {
		return leftName < rightName
	}

	leftExt := xdfileEntrySortExt(left)
	rightExt := xdfileEntrySortExt(right)
	if leftExt != rightExt {
		return leftExt < rightExt
	}
	return leftName < rightName
}

func xdfileEntrySortName(entry xdfileEntry) string {
	if entry.sortName != "" {
		return entry.sortName
	}
	return strings.ToLower(entry.Name)
}

func xdfileEntrySortExt(entry xdfileEntry) string {
	if entry.sortExt != "" {
		return entry.sortExt
	}
	return xdfileSortExtension(entry.Name)
}

func xdfileSortExtension(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}

func xdfileReadClipboardPaths() ([]string, error) {
	return platformsystem.ReadClipboardPaths()
}

func xdfileReadClipboardCut() (bool, error) {
	return platformsystem.ReadClipboardCut()
}

func xdfileWriteClipboardPaths(paths []string, cut bool) error {
	return platformsystem.WriteClipboardPaths(paths, cut)
}

func xdfileReadPreview(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return xdfileReadDirectoryPreview(path, info)
	}

	data, err := xdfileReadPreviewSample(path, xdfilePreviewLimit)
	if err != nil {
		return "", err
	}

	contentType := xdfileDetectContentType(data)
	switch {
	case xdfileArchivePreviewKind(path) != "":
		return xdfileReadArchivePreview(path, info, contentType)
	case xdfileIsRasterImagePreview(path):
		if preview, imageErr := xdfileReadImagePreview(path, info, contentType); imageErr == nil {
			return preview, nil
		}
	case xdfileIsPDFPreview(path, data, contentType):
		return xdfileReadPDFPreview(path, info, data, contentType), nil
	case xdfileIsMediaPreview(path, contentType):
		return xdfileReadMediaPreview(path, info, data, contentType), nil
	case xdfileIsTextPreview(path, data):
		return xdfileReadTextPreview(path, info, contentType)
	default:
		return xdfileReadBinaryPreview(path, info, data, contentType), nil
	}

	return xdfileReadBinaryPreview(path, info, data, contentType), nil
}

func xdfileReadBinaryPreviewPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	data, err := xdfileReadPreviewSample(path, xdfilePreviewLimit)
	if err != nil {
		return "", err
	}
	return xdfileReadBinaryPreview(path, info, data, xdfileDetectContentType(data)), nil
}

func xdfileReadDirectoryPreview(path string, info os.FileInfo) (string, error) {
	children, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	lines := []string{
		xdfilePreviewKeyValue("Path", path),
		xdfilePreviewTypeLine(path, "Directory"),
		xdfilePreviewKeyValue("Modified", info.ModTime().Format(time.RFC3339)),
		xdfilePreviewKeyValue("Children", strconv.Itoa(len(children))),
		"",
	}

	limit := min(40, len(children))
	for i := 0; i < limit; i++ {
		marker := xdfileEntryKindSpecForEntry(xdfileEntry{
			Name:  children[i].Name(),
			IsDir: children[i].IsDir(),
		}).render()
		lines = append(lines, marker+" "+xdfilePreviewListEntry(children[i].Name(), children[i].IsDir()))
	}
	if len(children) > limit {
		lines = append(lines, "", xdfilePreviewMuted(fmt.Sprintf("... %d more items", len(children)-limit)))
	}

	return strings.Join(lines, "\n"), nil
}

func xdfileReadPreviewSample(path string, limit int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := make([]byte, limit)
	n, readErr := file.Read(data)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	return data[:n], nil
}

func xdfileDetectContentType(data []byte) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(data[:min(512, len(data))])
}

func xdfilePreviewTypeStyle(path string) lipgloss.Style {
	if color, ok := xdfileResolveFileColor(filepath.Base(path)); ok {
		return lipgloss.NewStyle().Foreground(color).Bold(true)
	}
	return xdfileTitleStyle
}

func xdfilePreviewHeading(text string) string {
	return xdfileTitleStyle.Render(text)
}

func xdfilePreviewMuted(text string) string {
	return xdfileDimStyle.Render(text)
}

func xdfilePreviewKeyValue(label string, value string) string {
	return xdfileTagStyle.Render(xdfilePadRight(label+":", 11)) + xdfilePathStyle.Render(xdfileSanitizePreviewInlineText(value))
}

func xdfilePreviewTypeLine(path string, kind string) string {
	return xdfileTagStyle.Render(xdfilePadRight("Type:", 11)) + xdfilePreviewTypeStyle(path).Render(kind)
}

func xdfilePreviewListEntry(name string, isDir bool) string {
	if isDir {
		return xdfileDirStyle.Render(name)
	}
	if color, ok := xdfileResolveFileColor(name); ok {
		return lipgloss.NewStyle().Foreground(color).Render(name)
	}
	return xdfilePathStyle.Render(name)
}

func xdfilePreviewHeader(path string, info os.FileInfo, kind string, contentType string) []string {
	lines := []string{
		xdfilePreviewKeyValue("Path", path),
		xdfilePreviewTypeLine(path, kind),
		xdfilePreviewKeyValue("Size", xdfileHumanSize(info.Size())),
		xdfilePreviewKeyValue("Modified", info.ModTime().Format(time.RFC3339)),
	}
	if contentType != "" {
		lines = append(lines, xdfilePreviewKeyValue("Detected", contentType))
	}
	return append(lines, "")
}

func xdfileIsTextPreview(path string, data []byte) bool {
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		return true
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return false
	}
	if lexers.Match(filepath.Base(path)) != nil {
		return true
	}
	isText, err := common.IsTextFile(path)
	return err == nil && isText
}

func xdfileReadTextPreview(path string, info os.FileInfo, contentType string) (string, error) {
	kind := "Text file"
	lexer := lexers.Match(filepath.Base(path))
	if lexer != nil {
		kind = lexer.Config().Name + " source"
	}

	content, err := utils.ReadFileContent(path, xdfilePreviewLineWidth, xdfilePreviewLineLimit)
	if err != nil {
		return "", err
	}

	lines := xdfilePreviewHeader(path, info, kind, contentType)
	if strings.TrimSpace(content) == "" {
		lines = append(lines, xdfilePreviewMuted("(empty file)"))
		return strings.Join(lines, "\n"), nil
	}

	if lexer != nil && xdfileShouldHighlightLexerName(lexer.Config().Name) {
		if highlighted, highlightErr := xdfileHighlightANSI(content, lexer.Config().Name); highlightErr == nil && highlighted != "" {
			content = highlighted
		}
	}

	textLines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	lines = append(lines, textLines...)
	if len(textLines) >= xdfilePreviewLineLimit || info.Size() > xdfilePreviewLimit {
		lines = append(lines, "", xdfilePreviewMuted("... preview truncated ..."))
	}
	return strings.Join(lines, "\n"), nil
}

func xdfileIsRasterImagePreview(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".svg" {
		return false
	}
	return common.ImageExtensions[ext]
}

func xdfilePreviewCanUseImageThumbnail(path string) bool {
	return xdfileIsRasterImagePreview(path)
}

func xdfilePreviewCanUseThumbnail(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return xdfilePreviewCanUseImageThumbnail(path) || ext == ".pdf"
}

func xdfilePreviewCanToggleBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	data, err := xdfileReadPreviewSample(path, xdfilePreviewLimit)
	if err != nil {
		return false
	}

	contentType := xdfileDetectContentType(data)
	switch {
	case xdfileArchivePreviewKind(path) != "":
		return true
	case xdfileIsRasterImagePreview(path):
		return true
	case xdfileIsPDFPreview(path, data, contentType):
		return true
	case xdfileIsMediaPreview(path, contentType):
		return true
	case xdfileIsTextPreview(path, data):
		return true
	default:
		return false
	}
}

func xdfileReadImagePreview(path string, info os.FileInfo, contentType string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		return "", err
	}

	lines := xdfilePreviewHeader(path, info, strings.ToUpper(format)+" image", contentType)
	lines = append(lines,
		xdfilePreviewKeyValue("Format", strings.ToUpper(format)),
		xdfilePreviewKeyValue("Dimensions", fmt.Sprintf("%d x %d", cfg.Width, cfg.Height)),
		xdfilePreviewKeyValue("Color", fmt.Sprintf("%T", cfg.ColorModel)),
	)
	return strings.Join(lines, "\n"), nil
}

func xdfileArchivePreviewKind(path string) string {
	lowerName := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(lowerName, ".tar.gz"), strings.HasSuffix(lowerName, ".tgz"):
		return "TAR.GZ archive"
	case strings.HasSuffix(lowerName, ".tar.bz2"), strings.HasSuffix(lowerName, ".tbz2"):
		return "TAR.BZ2 archive"
	case strings.HasSuffix(lowerName, ".tar"):
		return "TAR archive"
	}
	if kind, ok := xdfileArchivePreviewLabels[strings.ToLower(filepath.Ext(lowerName))]; ok {
		return kind
	}
	return ""
}

func xdfileReadArchivePreview(path string, info os.FileInfo, contentType string) (string, error) {
	kind := xdfileArchivePreviewKind(path)
	entries, err := xdfileReadArchiveEntries(path)
	if err != nil {
		return "", err
	}

	lines := xdfilePreviewHeader(path, info, kind, contentType)
	lines = append(lines, xdfilePreviewKeyValue("Entries", strconv.Itoa(len(entries))), "")

	limit := min(xdfileArchiveEntryLimit, len(entries))
	for _, entry := range entries[:limit] {
		lines = append(lines, xdfilePreviewListEntry(entry, strings.HasSuffix(entry, "/")))
	}
	if len(entries) > limit {
		lines = append(lines, "", xdfilePreviewMuted(fmt.Sprintf("... preview truncated, %d more entries ...", len(entries)-limit)))
	}
	return strings.Join(lines, "\n"), nil
}

func xdfileReadArchiveEntries(path string) ([]string, error) {
	lowerName := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(lowerName, ".tar.gz"), strings.HasSuffix(lowerName, ".tgz"):
		return xdfileReadTarEntries(path, true, false)
	case strings.HasSuffix(lowerName, ".tar.bz2"), strings.HasSuffix(lowerName, ".tbz2"):
		return xdfileReadTarEntries(path, false, true)
	case strings.HasSuffix(lowerName, ".tar"):
		return xdfileReadTarEntries(path, false, false)
	default:
		return xdfileReadZIPEntries(path)
	}
}

func xdfileReadZIPEntries(path string) ([]string, error) {
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zipReader.Close()

	entries := make([]string, 0, len(zipReader.File))
	for _, file := range zipReader.File {
		name := file.Name
		if file.FileInfo().IsDir() && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		entries = append(entries, name)
	}
	return entries, nil
}

func xdfileReadTarEntries(path string, gzipCompressed bool, bzipCompressed bool) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var reader io.Reader = file
	if gzipCompressed {
		gzipReader, gzipErr := gzip.NewReader(file)
		if gzipErr != nil {
			return nil, gzipErr
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	if bzipCompressed {
		reader = bzip2.NewReader(file)
	}

	tarReader := tar.NewReader(reader)
	entries := []string{}
	for {
		header, tarErr := tarReader.Next()
		switch {
		case errors.Is(tarErr, io.EOF):
			return entries, nil
		case tarErr != nil:
			return nil, tarErr
		case header == nil:
			continue
		}

		name := header.Name
		if header.FileInfo().IsDir() && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		entries = append(entries, name)
	}
}

func xdfileIsPDFPreview(path string, data []byte, contentType string) bool {
	if strings.EqualFold(filepath.Ext(path), ".pdf") {
		return true
	}
	return bytes.HasPrefix(data, []byte("%PDF-")) || contentType == "application/pdf"
}

func xdfileReadPDFPreview(path string, info os.FileInfo, data []byte, contentType string) string {
	lines := xdfilePreviewHeader(path, info, "PDF document", contentType)

	if version := xdfilePDFVersion(data); version != "" {
		lines = append(lines, xdfilePreviewKeyValue("Version", "PDF-"+version))
	}

	metadata := xdfileExtractPDFMetadata(data)
	if len(metadata) > 0 {
		lines = append(lines, "", xdfilePreviewHeading("Metadata"))
		lines = append(lines, metadata...)
	}

	snippets := xdfileExtractPrintableStrings(data, 6, 8)
	if len(snippets) > 0 {
		lines = append(lines, "", xdfilePreviewHeading("Sample strings"))
		for _, snippet := range snippets {
			lines = append(lines, xdfileMetaStyle.Render("• ")+xdfilePathStyle.Render(xdfileSanitizePreviewInlineText(snippet)))
		}
	}

	if info.Size() > int64(len(data)) {
		lines = append(lines, "", xdfilePreviewMuted("... preview truncated ..."))
	}
	return strings.Join(lines, "\n")
}

func xdfilePDFVersion(data []byte) string {
	if !bytes.HasPrefix(data, []byte("%PDF-")) || len(data) < 8 {
		return ""
	}
	end := 8
	for end < len(data) && data[end] != '\n' && data[end] != '\r' && end < 16 {
		end++
	}
	return string(bytes.TrimSpace(data[5:end]))
}

func xdfileExtractPDFMetadata(data []byte) []string {
	lines := make([]string, 0, len(xdfilePDFMetadataKeys))
	for _, key := range xdfilePDFMetadataKeys {
		marker := []byte("/" + key + " (")
		index := bytes.Index(data, marker)
		if index < 0 {
			continue
		}
		start := index + len(marker)
		raw, ok := xdfileReadPDFLiteralStringBytes(data[start:], xdfilePDFMetadataLimit)
		if !ok {
			continue
		}
		value := xdfileDecodePDFLiteralString(raw)
		if value == "" {
			continue
		}
		lines = append(lines, xdfilePreviewKeyValue(key, value))
	}
	return lines
}

func xdfileReadPDFLiteralStringBytes(data []byte, maxBytes int) ([]byte, bool) {
	var out []byte
	depth := 1
	for i := 0; i < len(data) && len(out) < maxBytes; i++ {
		b := data[i]
		switch b {
		case '\\':
			if i+1 >= len(data) {
				return out, false
			}
			i++
			escaped := data[i]
			switch escaped {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '(', ')', '\\':
				out = append(out, escaped)
			case '\n':
			case '\r':
				if i+1 < len(data) && data[i+1] == '\n' {
					i++
				}
			default:
				if escaped >= '0' && escaped <= '7' {
					value := int(escaped - '0')
					for count := 0; count < 2 && i+1 < len(data) && data[i+1] >= '0' && data[i+1] <= '7'; count++ {
						i++
						value = value*8 + int(data[i]-'0')
					}
					out = append(out, byte(value))
				} else {
					out = append(out, escaped)
				}
			}
		case '(':
			depth++
			out = append(out, b)
		case ')':
			depth--
			if depth == 0 {
				return out, true
			}
			out = append(out, b)
		default:
			out = append(out, b)
		}
	}
	return out, false
}

func xdfileDecodePDFLiteralString(data []byte) string {
	var value string
	if decoded, ok := xdfileDecodeUTF16CommandOutput(data); ok {
		value = decoded
	} else {
		value = xdfileDecodeCommandOutput(data)
	}
	return xdfileSanitizePreviewInlineText(value)
}

func xdfileSanitizePreviewInlineText(value string) string {
	if value == "" {
		return ""
	}
	value = xdfileStripANSISequences(value)
	var builder strings.Builder
	builder.Grow(len(value))
	spacePending := false
	for _, r := range value {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			spacePending = true
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			spacePending = true
		case r == utf8.RuneError:
			spacePending = true
		default:
			if spacePending && builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			builder.WriteRune(r)
			spacePending = false
		}
	}
	return strings.TrimSpace(builder.String())
}

func xdfileStripANSISequences(value string) string {
	if value == "" {
		return ""
	}
	data := []byte(value)
	var builder strings.Builder
	builder.Grow(len(data))
	for i := 0; i < len(data); {
		if data[i] == 0x1b {
			if _, consumed, _ := xdfileConsumeManagedANSISequence(data[i:]); consumed > 0 {
				i += consumed
				continue
			}
			i++
			continue
		}
		builder.WriteByte(data[i])
		i++
	}
	return builder.String()
}

func xdfileIsMediaPreview(path string, contentType string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return common.VideoExtensions[ext] || xdfileAudioExtensions.has(ext) ||
		strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/")
}

func xdfileReadMediaPreview(path string, info os.FileInfo, data []byte, contentType string) string {
	ext := strings.ToLower(filepath.Ext(path))
	kind := "Media file"
	if common.VideoExtensions[ext] || strings.HasPrefix(contentType, "video/") {
		kind = "Video file"
	} else if xdfileAudioExtensions.has(ext) || strings.HasPrefix(contentType, "audio/") {
		kind = "Audio file"
	}

	lines := xdfilePreviewHeader(path, info, kind, contentType)
	lines = append(lines, xdfilePreviewKeyValue("Extension", ext))
	if signature := xdfileDescribeSignature(data); signature != "" {
		lines = append(lines, xdfilePreviewKeyValue("Signature", signature))
	}
	if strings.EqualFold(ext, ".wav") {
		lines = append(lines, xdfileReadWAVMetadata(data)...)
	}
	lines = append(lines, "", xdfilePreviewMuted("Binary payload is not rendered inline for recognized media files."))
	return strings.Join(lines, "\n")
}

func xdfileReadWAVMetadata(data []byte) []string {
	if len(data) < 44 || !bytes.Equal(data[:4], []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		return nil
	}

	return []string{
		xdfilePreviewKeyValue("Channels", strconv.Itoa(int(binary.LittleEndian.Uint16(data[22:24])))),
		xdfilePreviewKeyValue("Rate", fmt.Sprintf("%d Hz", binary.LittleEndian.Uint32(data[24:28]))),
		xdfilePreviewKeyValue("Bit depth", strconv.Itoa(int(binary.LittleEndian.Uint16(data[34:36])))),
	}
}

func xdfileReadBinaryPreview(path string, info os.FileInfo, data []byte, contentType string) string {
	lines := xdfilePreviewHeader(path, info, "Binary file", contentType)
	if signature := xdfileDescribeSignature(data); signature != "" {
		lines = append(lines, xdfilePreviewKeyValue("Signature", signature), "")
	}
	lines = append(lines, xdfilePreviewHeading("Hex dump:"))

	linesAdded := 0
	for offset := 0; offset < len(data) && linesAdded < xdfileHexLineLimit; offset += xdfileHexBytesPerLine {
		end := min(offset+xdfileHexBytesPerLine, len(data))
		chunk := data[offset:end]

		ascii := make([]byte, 0, len(chunk))
		for _, b := range chunk {
			if b >= 32 && b <= 126 {
				ascii = append(ascii, b)
			} else {
				ascii = append(ascii, '.')
			}
		}

		offsetText := xdfileDimStyle.Render(fmt.Sprintf("%08X", offset))
		hexText := lipgloss.NewStyle().Foreground(xdfileColorAccent2).Render(
			xdfilePadRight(xdfileFormatHexBytes(chunk), xdfileHexColumnWidth),
		)
		asciiText := xdfilePathStyle.Render("|" + string(ascii) + "|")
		lines = append(lines, offsetText+"  "+hexText+"  "+asciiText)
		linesAdded++
	}

	if len(data) > xdfileHexBytesPerLine*xdfileHexLineLimit || info.Size() > int64(len(data)) {
		lines = append(lines, "", xdfilePreviewMuted("... preview truncated ..."))
	}
	return strings.Join(lines, "\n")
}

func xdfileFormatHexBytes(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	parts := make([]string, 0, len(data)+1)
	for i, b := range data {
		if i == 8 {
			parts = append(parts, "")
		}
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	return strings.Join(parts, " ")
}

func xdfileDescribeSignature(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "PNG"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xFF, 0xD8, 0xFF}):
		return "JPEG"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "GIF"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("%PDF")):
		return "PDF"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("PK\x03\x04")):
		return "ZIP"
	case len(data) >= 2 && bytes.Equal(data[:2], []byte{0x1F, 0x8B}):
		return "GZIP"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("RIFF")):
		return "RIFF container"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("fLaC")):
		return "FLAC"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("OggS")):
		return "Ogg"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte("ID3")):
		return "MP3/ID3"
	case len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")):
		return "ISO Base Media container"
	}
	return ""
}

func xdfileExtractPrintableStrings(data []byte, minLength int, maxCount int) []string {
	lines := []string{}
	var current strings.Builder

	flush := func() {
		if current.Len() < minLength || len(lines) >= maxCount {
			current.Reset()
			return
		}
		lines = append(lines, current.String())
		current.Reset()
	}

	for _, b := range data {
		if b >= 32 && b <= 126 {
			current.WriteByte(b)
			continue
		}
		flush()
		if len(lines) >= maxCount {
			break
		}
	}
	flush()
	return lines
}

func xdfileOpenPath(path string) error {
	return platformsystem.OpenPath(filepath.Clean(path))
}

func xdfileDecodeCommandOutput(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if utf8.Valid(data) {
		return string(data)
	}
	if decoded, ok := xdfileDecodeUTF16CommandOutput(data); ok {
		return decoded
	}
	if runtime.GOOS == "windows" {
		if decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data); err == nil && utf8.Valid(decoded) {
			return string(decoded)
		}
	}
	return string(bytes.Runes(data))
}

func xdfileDecodeUTF16CommandOutput(data []byte) (string, bool) {
	if len(data) < 2 {
		return "", false
	}

	var decoder *encoding.Decoder
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		decoder = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		decoder = unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder()
	case xdfileLooksLikeUTF16LE(data):
		decoder = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
	case xdfileLooksLikeUTF16BE(data):
		decoder = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()
	default:
		return "", false
	}

	decoded, err := decoder.Bytes(data)
	if err != nil || !utf8.Valid(decoded) {
		return "", false
	}
	return string(decoded), true
}

func xdfileLooksLikeUTF16LE(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	sample := data[:min(len(data), 64)]
	zeroCount := 0
	for i := 1; i < len(sample); i += 2 {
		if sample[i] == 0 {
			zeroCount++
		}
	}
	return zeroCount >= max(2, len(sample)/8)
}

func xdfileLooksLikeUTF16BE(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	sample := data[:min(len(data), 64)]
	zeroCount := 0
	for i := 0; i < len(sample); i += 2 {
		if sample[i] == 0 {
			zeroCount++
		}
	}
	return zeroCount >= max(2, len(sample)/8)
}

func xdfileHumanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}

	value := float64(size)
	suffixes := []string{"B", "K", "M", "G", "T", "P"}
	exp := 0
	for value >= unit && exp < len(suffixes)-1 {
		value /= unit
		exp++
	}
	if value >= 100 {
		return fmt.Sprintf("%.0f%s", value, suffixes[exp])
	}
	return fmt.Sprintf("%.1f%s", value, suffixes[exp])
}

func xdfileRenamePath(oldPath string, newPath string) error {
	if xdfileIsNetBoxPath(oldPath) || xdfileIsNetBoxPath(newPath) {
		return xdfileNetBoxRenamePath(oldPath, newPath)
	}
	oldClean := filepath.Clean(oldPath)
	newClean := filepath.Clean(newPath)
	if oldClean == newClean {
		return nil
	}
	if _, err := os.Stat(newClean); err == nil {
		return fmt.Errorf("target already exists: %s", newClean)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(oldClean, newClean)
}

func xdfileMkdirPath(path string) error {
	if xdfileIsNetBoxPath(path) {
		return xdfileNetBoxMakeDir(path)
	}
	return os.MkdirAll(path, 0o755)
}

func xdfileUniqueCopyTarget(targetPath string) (string, error) {
	targetPath = filepath.Clean(targetPath)
	if _, err := os.Stat(targetPath); errors.Is(err, os.ErrNotExist) {
		return targetPath, nil
	} else if err != nil {
		return "", err
	}
	return xdfileUniqueNumberedCopyTarget(targetPath)
}

func xdfileUniquePasteCopyTarget(sourcePath string, targetPath string) (string, error) {
	if xdfilePathsEqual(sourcePath, targetPath) {
		return xdfileUniqueSameFolderCopyTarget(targetPath)
	}
	return xdfileUniqueCopyTarget(targetPath)
}

func xdfileUniqueSameFolderCopyTarget(targetPath string) (string, error) {
	targetPath = filepath.Clean(targetPath)
	if _, err := os.Stat(targetPath); errors.Is(err, os.ErrNotExist) {
		return targetPath, nil
	} else if err != nil {
		return "", err
	}

	dir, base, ext, err := xdfileCopyNameParts(targetPath)
	if err != nil {
		return "", err
	}
	for i := 1; i <= 999; i++ {
		suffix := " - Copy"
		if i > 1 {
			suffix = fmt.Sprintf(" - Copy (%d)", i)
		}
		candidate := filepath.Join(dir, base+suffix+ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("unable to find a free copy target for %s", targetPath)
}

func xdfileUniqueNumberedCopyTarget(targetPath string) (string, error) {
	targetPath = filepath.Clean(targetPath)
	dir := filepath.Dir(targetPath)
	_, base, ext, err := xdfileCopyNameParts(targetPath)
	if err != nil {
		return "", err
	}

	for i := 2; i <= 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("unable to find a free copy target for %s", targetPath)
}

func xdfileCopyNameParts(targetPath string) (string, string, string, error) {
	targetPath = filepath.Clean(targetPath)
	dir := filepath.Dir(targetPath)
	name := filepath.Base(targetPath)

	info, err := os.Stat(targetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", "", err
	}
	if err == nil && info.IsDir() {
		return dir, name, "", nil
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if base == "" {
		return dir, name, "", nil
	}
	return dir, base, ext, nil
}

func xdfileUniqueReplaceStageTarget(targetPath string) (string, error) {
	targetPath = filepath.Clean(targetPath)
	dir := filepath.Dir(targetPath)
	base := filepath.Base(targetPath)

	for i := 1; i <= 999; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s.xdfile-replace-%d", base, i))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("unable to find a replace staging target for %s", targetPath)
}

func xdfileReplacePath(sourcePath string, targetPath string, move bool) error {
	sourcePath = filepath.Clean(sourcePath)
	targetPath = filepath.Clean(targetPath)

	stagedTarget, err := xdfileUniqueReplaceStageTarget(targetPath)
	if err != nil {
		return err
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(stagedTarget)
		}
	}()

	if move {
		err = xdfileMovePath(sourcePath, stagedTarget)
	} else {
		err = xdfileCopyPath(sourcePath, stagedTarget)
	}
	if err != nil {
		return err
	}

	if err := os.RemoveAll(targetPath); err != nil {
		return err
	}
	if err := os.Rename(stagedTarget, targetPath); err != nil {
		return err
	}

	cleanup = false
	return nil
}

func xdfileValidateTransferTarget(sourcePath string, targetPath string, info os.FileInfo) error {
	sourceClean := filepath.Clean(sourcePath)
	targetClean := filepath.Clean(targetPath)
	if xdfilePathsEqual(sourceClean, targetClean) {
		return fmt.Errorf("source and target are the same")
	}
	if !info.IsDir() {
		return nil
	}
	if xdfilePathWithinRoot(sourceClean, targetClean) {
		return fmt.Errorf("target is inside source: %s", targetClean)
	}
	return nil
}

func xdfilePathWithinRoot(root string, path string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		rootAbs = filepath.Clean(root)
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		pathAbs = filepath.Clean(path)
	}

	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}

	parentPrefix := ".." + string(os.PathSeparator)
	if rel == ".." || strings.HasPrefix(rel, parentPrefix) {
		return false
	}
	if runtime.GOOS == "windows" {
		return !strings.HasPrefix(strings.ToLower(rel), strings.ToLower(parentPrefix))
	}
	return true
}

func xdfileCopyPath(sourcePath string, targetPath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlinks are not supported yet: %s", sourcePath)
	}
	if err := xdfileValidateTransferTarget(sourcePath, targetPath, info); err != nil {
		return err
	}
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("target already exists: %s", targetPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if info.IsDir() {
		return xdfileCopyDir(sourcePath, targetPath, info)
	}
	return xdfileCopyFile(sourcePath, targetPath, info)
}

func xdfileCopyDir(sourcePath string, targetPath string, info os.FileInfo) error {
	if err := os.MkdirAll(targetPath, info.Mode().Perm()); err != nil {
		return err
	}

	items, err := os.ReadDir(sourcePath)
	if err != nil {
		return err
	}

	for _, item := range items {
		childSource := filepath.Join(sourcePath, item.Name())
		childTarget := filepath.Join(targetPath, item.Name())
		if err := xdfileCopyPath(childSource, childTarget); err != nil {
			return err
		}
	}

	return os.Chtimes(targetPath, info.ModTime(), info.ModTime())
}

func xdfileCopyFile(sourcePath string, targetPath string, info os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}

	return os.Chtimes(targetPath, info.ModTime(), info.ModTime())
}

func xdfileMovePath(sourcePath string, targetPath string) error {
	sourceClean := filepath.Clean(sourcePath)
	targetClean := filepath.Clean(targetPath)

	info, err := os.Lstat(sourceClean)
	if err != nil {
		return err
	}
	if err := xdfileValidateTransferTarget(sourceClean, targetClean, info); err != nil {
		return err
	}
	if _, err := os.Stat(targetClean); err == nil {
		return fmt.Errorf("target already exists: %s", targetClean)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetClean), 0o755); err != nil {
		return err
	}
	if err := os.Rename(sourceClean, targetClean); err == nil {
		return nil
	}

	if err := xdfileCopyPath(sourceClean, targetClean); err != nil {
		return err
	}
	return os.RemoveAll(sourceClean)
}

func xdfileExecuteCommandCmd(dir string, command string, width int, height int) tea.Cmd {
	return func() tea.Msg {
		command = strings.TrimSpace(command)
		if xdfileIsNetBoxPath(dir) {
			if result, handled := xdfileRunNetBoxManagedTerminalCommand(dir, command); handled {
				return result
			}
			events := make(chan tea.Msg, xdfileTerminalEventBufferSize)
			cancel, err := xdfileStartNetBoxStreamingTerminalCommand(dir, command, events)
			if err != nil {
				close(events)
				return xdfileTerminalResultMsg{
					Command: command,
					Dir:     dir,
					Err:     err,
				}
			}
			return xdfileTerminalCommandStartMsg{
				Command: command,
				Dir:     dir,
				Events:  events,
				Cancel:  cancel,
			}
		}
		if result, handled := xdfileRunManagedShellCommand(dir, command); handled {
			return result
		}

		events := make(chan tea.Msg, xdfileTerminalEventBufferSize)
		cancel, emulator, err := xdfileStartStreamingCommand(dir, command, events, width, height)
		if err != nil {
			close(events)
			return xdfileTerminalResultMsg{
				Command: command,
				Dir:     dir,
				Err:     err,
			}
		}
		return xdfileTerminalCommandStartMsg{
			Command:  command,
			Dir:      dir,
			Events:   events,
			Cancel:   cancel,
			Emulator: emulator,
		}
	}
}

func xdfileWaitTerminalMsg(events <-chan tea.Msg) tea.Cmd {
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return msg
	}
}

func xdfileRunCommand(dir string, command string) xdfileTerminalResultMsg {
	command = strings.TrimSpace(command)
	result := xdfileTerminalResultMsg{
		Command: command,
		Dir:     dir,
	}
	if command == "" {
		return result
	}

	if xdfileIsNetBoxPath(dir) {
		return xdfileRunNetBoxTerminalCommand(dir, command)
	}

	if managed, handled := xdfileRunManagedShellCommand(dir, command); handled {
		return managed
	}

	switch strings.ToLower(command) {
	case "clear", "cls":
		result.Clear = true
		return result
	case "pwd":
		result.Output = dir
		return result
	}

	if nextDir, handled, err := xdfileBuiltinCD(dir, command); handled {
		result.Dir = nextDir
		result.Err = err
		result.SyncActivePanel = err == nil
		if err == nil {
			result.Output = nextDir
		}
		return result
	}

	if detached, handled := xdfileStartDetachedExternalCommand(dir, command); handled {
		return detached
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	command = xdfilePrepareExternalCommand(command)
	shell, args, cleanup, err := xdfileExternalCommandInvocation(dir, command)
	if err != nil {
		result.Err = fmt.Errorf("prepare command: %w", err)
		return result
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = dir
	cmd.Env = xdfileCommandExecutionEnvironment(os.Environ())
	xdfileConfigureManagedExternalCommand(cmd)
	outputBytes, err := cmd.CombinedOutput()
	result.Output = strings.TrimRight(
		xdfileSanitizeManagedTerminalText(
			strings.ReplaceAll(xdfileDecodeCommandOutput(outputBytes), "\r\n", "\n"),
		),
		"\n",
	)

	if err != nil {
		result.Err = xdfileNormalizeCommandError(err)
	}
	return result
}

func xdfileStartStreamingCommand(dir string, command string, events chan tea.Msg, width int, height int) (func(), *vt.SafeEmulator, error) {
	if cancel, handled := xdfileStartDetachedStreamingCommand(dir, command, events); handled {
		return cancel, nil, nil
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		cancel, emulator, err := xdfileStartStreamingCommandPTY(dir, command, events, width, height)
		if err == nil {
			return cancel, emulator, nil
		}
		return xdfileStartStreamingCommandPipe(dir, command, events)
	}
	return xdfileStartStreamingCommandPipe(dir, command, events)
}

func xdfileStartDetachedStreamingCommand(dir string, command string, events chan tea.Msg) (func(), bool) {
	detached, handled := xdfileStartDetachedExternalCommand(dir, command)
	if !handled {
		return nil, false
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		defer close(events)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if detached.Output != "" {
			events <- xdfileTerminalLineMsg{Line: detached.Output, Finalize: true}
		}
		events <- xdfileTerminalCommandDoneMsg{
			Cwd: dir,
			Err: detached.Err,
		}
	}()
	return cancel, true
}

func xdfileStartStreamingCommandPipe(dir string, command string, events chan tea.Msg) (func(), *vt.SafeEmulator, error) {
	ctx, cancel := context.WithCancel(context.Background())

	if detached, handled := xdfileStartDetachedExternalCommand(dir, command); handled {
		go func() {
			defer cancel()
			defer close(events)
			if detached.Output != "" {
				events <- xdfileTerminalLineMsg{Line: detached.Output, Finalize: true}
			}
			events <- xdfileTerminalCommandDoneMsg{
				Cwd: dir,
				Err: detached.Err,
			}
		}()
		return cancel, nil, nil
	}

	command = xdfilePrepareExternalCommand(command)
	shell, args, cleanup, err := xdfileExternalCommandInvocation(dir, command)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("prepare command: %w", err)
	}

	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = dir
	cmd.Env = xdfileCommandExecutionEnvironment(os.Environ())
	xdfileConfigureManagedExternalCommand(cmd)

	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		cancel()
		cleanup()
		_ = reader.Close()
		_ = writer.Close()
		return nil, nil, fmt.Errorf("start command: %w", err)
	}

	go func() {
		defer cancel()
		defer cleanup()
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
			err = xdfileNormalizeCommandError(err)
		}

		events <- xdfileTerminalCommandDoneMsg{
			Cwd:      dir,
			Err:      err,
			Canceled: canceled,
		}
	}()

	return cancel, nil, nil
}

func xdfileStartStreamingCommandPTY(dir string, command string, events chan tea.Msg, width int, height int) (func(), *vt.SafeEmulator, error) {
	ctx, cancel := context.WithCancel(context.Background())
	width = max(10, width)
	height = max(1, height)

	command = xdfilePrepareExternalCommand(command)
	path, args, cleanup, err := xdfileExternalCommandInvocation(dir, command)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("prepare command: %w", err)
	}

	backend, process, err := xdfileStartCommandPTYBackend(dir, path, args, width, height)
	if err != nil {
		cancel()
		cleanup()
		return nil, nil, fmt.Errorf("start PTY command: %w", err)
	}
	emulator := vt.NewSafeEmulator(width, height)
	emulator.SetScrollbackSize(xdfileTerminalScrollbackLimit)

	stop := sync.OnceFunc(func() {
		_ = process.Kill()
		_ = backend.Close()
		_ = emulator.Close()
		cancel()
	})

	go func() {
		defer cancel()
		defer cleanup()
		defer close(events)

		readDone := make(chan error, 1)
		go func() {
			buf := make([]byte, 32*1024)
			for {
				n, readErr := backend.Read(buf)
				if n > 0 {
					if _, writeErr := emulator.Write(buf[:n]); writeErr != nil {
						readDone <- fmt.Errorf("render PTY output: %w", writeErr)
						return
					}
					events <- xdfileTerminalStreamScreenMsg{}
				}
				if readErr != nil {
					if errors.Is(readErr, io.EOF) || xdfileIsBenignPTYReadError(readErr) {
						readDone <- nil
						return
					}
					readDone <- readErr
					return
				}
			}
		}()

		waitDone := make(chan error, 1)
		go func() {
			_, waitErr := process.Wait()
			waitDone <- waitErr
		}()

		err := <-waitDone
		_ = backend.Close()
		readErr := <-readDone
		_ = emulator.Close()
		if err == nil && readErr != nil {
			err = readErr
		}

		canceled := errors.Is(ctx.Err(), context.Canceled)
		if canceled {
			err = nil
		} else if err != nil {
			err = xdfileNormalizeCommandError(err)
		}

		events <- xdfileTerminalCommandDoneMsg{
			Cwd:      dir,
			Err:      err,
			Canceled: canceled,
		}
	}()

	go func() {
		<-ctx.Done()
		stop()
	}()

	return stop, emulator, nil
}

func xdfileStartDetachedExternalCommand(dir string, command string) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: strings.TrimSpace(command),
		Dir:     dir,
	}
	path, args, ok := xdfileDetachedExternalCommandCandidate(dir, command)
	if !ok {
		return result, false
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Env = xdfileCommandExecutionEnvironment(os.Environ())
	if err := cmd.Start(); err != nil {
		result.Err = fmt.Errorf("start detached command: %w", err)
		return result, true
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	result.Output = fmt.Sprintf("Started %s", path)
	return result, true
}

func xdfileDetachedExternalCommandCandidate(dir string, command string) (string, []string, bool) {
	if runtime.GOOS != "windows" {
		return "", nil, false
	}
	command = strings.TrimSpace(command)
	if command == "" || xdfileContainsShellOperators(command) {
		return "", nil, false
	}

	parsed, err := xdfileParseShellCommand(command)
	if err != nil || parsed.Name == "" {
		return "", nil, false
	}

	path, ok := xdfileResolveExternalExecutablePath(dir, parsed.Name)
	if !ok || !strings.EqualFold(filepath.Ext(path), ".exe") {
		return "", nil, false
	}
	if !xdfileIsDetachedGUIExecutableFunc(path) {
		return "", nil, false
	}
	return path, parsed.Args, true
}

func xdfileResolveExternalExecutablePath(dir string, name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}

	hasPathSeparator := strings.ContainsAny(name, `/\`)
	if hasPathSeparator || filepath.IsAbs(name) {
		path := filepath.FromSlash(name)
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		return xdfileExistingExecutablePath(path)
	}

	if dir != "" {
		if path, ok := xdfileExistingExecutablePath(filepath.Join(dir, name)); ok {
			return path, true
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return xdfileExistingExecutablePath(path)
}

func xdfileExistingExecutablePath(path string) (string, bool) {
	candidates := []string{path}
	if filepath.Ext(path) == "" {
		candidates = append(candidates, path+".exe")
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		abs, err := filepath.Abs(candidate)
		if err != nil {
			return filepath.Clean(candidate), true
		}
		return abs, true
	}
	return "", false
}

func xdfileIsWindowsGUIExecutable(path string) bool {
	file, err := pe.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		return header.Subsystem == 2
	case *pe.OptionalHeader64:
		return header.Subsystem == 2
	default:
		return false
	}
}

func xdfileExternalCommandInvocation(dir string, command string) (string, []string, func(), error) {
	cleanup := func() {}
	if runtime.GOOS != "windows" {
		return "/bin/sh", []string{"-lc", command}, cleanup, nil
	}
	cmdPath := xdfileWindowsCmdPath()

	if xdfileWindowsExternalCommandNeedsScript(command) {
		scriptPath, err := xdfileWriteWindowsCommandScript(dir, command)
		if err != nil {
			return "", nil, nil, err
		}
		cleanup = func() {
			_ = os.Remove(scriptPath)
		}
		return cmdPath, []string{"/d", "/c", scriptPath}, cleanup, nil
	}

	return cmdPath, []string{"/d", "/c", "chcp 65001>nul & " + command}, cleanup, nil
}

func xdfileWindowsExternalCommandNeedsScript(command string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return strings.Contains(command, `"`)
}

func xdfileWriteWindowsCommandScript(dir string, command string) (string, error) {
	file, err := os.CreateTemp("", "xdfile-external-*.cmd")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}

	content := "@echo off\r\nchcp 65001>nul\r\n" + xdfileWindowsCommandScriptBody(dir, command) + "\r\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func xdfileWindowsCommandScriptBody(dir string, command string) string {
	command = strings.ReplaceAll(command, "\r\n", "\n")
	command = strings.ReplaceAll(command, "\r", "\n")
	lines := strings.Split(command, "\n")
	for i, line := range lines {
		lines[i] = xdfileWindowsCommandScriptLine(dir, line)
	}
	return strings.Join(lines, "\r\n")
}

func xdfileWindowsCommandScriptLine(dir string, line string) string {
	leadingLen := len(line) - len(strings.TrimLeft(line, " \t"))
	leading := line[:leadingLen]
	body := line[leadingLen:]

	at := ""
	if strings.HasPrefix(body, "@") {
		at = "@"
		body = strings.TrimLeft(body[1:], " \t")
	}

	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return line
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{":", "::", "rem ", "call ", "start ", "if ", "for "} {
		if strings.HasPrefix(lower, prefix) {
			return line
		}
	}

	if xdfileWindowsCommandScriptLineStartsDetachedGUI(dir, body) {
		return leading + at + `start "" ` + body
	}

	first, _ := xdfileFirstWindowsCommandScriptToken(body)
	if first == "" {
		return line
	}
	ext := strings.ToLower(filepath.Ext(strings.Trim(first, `"`)))
	if ext != ".cmd" && ext != ".bat" {
		return line
	}
	return leading + at + "call " + body
}

func xdfileWindowsCommandScriptLineStartsDetachedGUI(dir string, body string) bool {
	if runtime.GOOS != "windows" || xdfileContainsShellOperators(body) {
		return false
	}

	parsed, err := xdfileParseShellCommand(body)
	if err != nil || parsed.Name == "" {
		return false
	}

	path, ok := xdfileResolveExternalExecutablePath(dir, parsed.Name)
	if !ok || !strings.EqualFold(filepath.Ext(path), ".exe") {
		return false
	}
	return xdfileIsDetachedGUIExecutableFunc(path)
}

func xdfileFirstWindowsCommandScriptToken(value string) (string, string) {
	value = strings.TrimLeft(value, " \t")
	if value == "" {
		return "", ""
	}
	if value[0] == '"' {
		if end := strings.IndexByte(value[1:], '"'); end >= 0 {
			tokenEnd := end + 2
			return value[:tokenEnd], value[tokenEnd:]
		}
		return value, ""
	}
	for i, r := range value {
		if r == ' ' || r == '\t' {
			return value[:i], value[i:]
		}
	}
	return value, ""
}

func xdfileStreamCommandOutput(reader io.Reader, emit func(string, bool, bool)) error {
	buffered := bufio.NewReader(reader)
	var chunk []byte
	pendingCarriageReturn := false

	flush := func(rewrite bool, finalize bool) {
		text := strings.TrimRight(strings.ReplaceAll(xdfileDecodeCommandOutput(chunk), "\r\n", "\n"), "\r\n")
		text = xdfileSanitizeManagedTerminalText(text)
		chunk = chunk[:0]
		if text == "" {
			return
		}
		emit(text, rewrite, finalize)
	}

	for {
		b, err := buffered.ReadByte()
		if err != nil {
			if pendingCarriageReturn {
				flush(true, false)
			}
			if len(chunk) > 0 {
				flush(false, true)
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		if pendingCarriageReturn {
			if b == '\n' {
				flush(true, true)
				pendingCarriageReturn = false
				continue
			}
			flush(true, false)
			pendingCarriageReturn = false
		}

		switch b {
		case '\r':
			pendingCarriageReturn = true
		case '\n':
			flush(false, true)
		default:
			chunk = append(chunk, b)
		}
	}
}

func xdfileCommandExecutionEnvironment(base []string) []string {
	env := append([]string(nil), base...)
	if runtime.GOOS != "windows" {
		env = xdfileSetCommandEnvValue(env, "TERM", "xterm-256color")
	}
	env = xdfileSetCommandEnvValue(env, "COLORTERM", "truecolor")
	env = xdfileSetCommandEnvValue(env, "CLICOLOR", "1")
	env = xdfileSetCommandEnvValue(env, "CLICOLOR_FORCE", "1")
	env = xdfileSetCommandEnvValue(env, "FORCE_COLOR", "1")
	env = xdfileSetCommandEnvValue(env, "TERM_PROGRAM", "XdfileManager")
	return env
}

func xdfileSetCommandEnvValue(env []string, key string, value string) []string {
	prefix := strings.ToUpper(key) + "="
	for i, entry := range env {
		if strings.HasPrefix(strings.ToUpper(entry), prefix) {
			env[i] = key + "=" + value
			return env
		}
	}
	return append(env, key+"="+value)
}

func xdfilePrepareExternalCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return command
	}
	if xdfileContainsShellOperators(command) {
		return command
	}

	parsed, err := xdfileParseShellCommand(command)
	if err != nil || !strings.EqualFold(parsed.Name, "git") || xdfileGitCommandHasExplicitColor(parsed.Args) {
		return command
	}

	remainder := strings.TrimSpace(command[len(parsed.Name):])
	if remainder == "" {
		return `git -c color.ui=always -c core.pager=cat`
	}
	return `git -c color.ui=always -c core.pager=cat ` + remainder
}

func xdfileGitCommandHasExplicitColor(args []string) bool {
	for i, arg := range args {
		lower := strings.ToLower(strings.TrimSpace(arg))
		if strings.HasPrefix(lower, "--color") {
			return true
		}
		if strings.HasPrefix(lower, "-c") {
			value := strings.TrimPrefix(lower, "-c")
			if value == "" && i+1 < len(args) {
				value = strings.ToLower(strings.TrimSpace(args[i+1]))
			}
			if strings.Contains(value, "color.ui=") {
				return true
			}
		}
	}
	return false
}

func xdfileHighlightManagedShellText(path string, content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	lexer := lexers.Match(filepath.Base(path))
	if lexer == nil {
		return content
	}
	if !xdfileShouldHighlightLexerName(lexer.Config().Name) {
		return content
	}
	highlighted, err := xdfileHighlightANSI(content, lexer.Config().Name)
	if err != nil || highlighted == "" {
		return content
	}
	return strings.TrimRight(highlighted, "\n")
}

func xdfileShouldHighlightLexerName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "plaintext", "text", "text only":
		return false
	default:
		return true
	}
}

func xdfileHighlightANSI(content string, lexerName string) (string, error) {
	var highlighted bytes.Buffer
	if err := quick.Highlight(&highlighted, content, lexerName, "terminal256", "monokai"); err != nil {
		return "", err
	}
	return highlighted.String(), nil
}

func xdfileNormalizeCommandError(err error) error {
	if err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		return err
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}

	code, raw := xdfileNormalizeWindowsExitCode(exitErr.ExitCode())
	if code < 0 {
		return fmt.Errorf("command exited with code %d (0x%08x)", code, raw)
	}
	return fmt.Errorf("command exited with code %d", code)
}

func xdfileNormalizeWindowsExitCode(code int) (int32, uint32) {
	raw := uint32(code)
	return int32(raw), raw
}

func xdfileSanitizeManagedTerminalText(text string) string {
	if text == "" {
		return ""
	}

	data := []byte(text)
	var builder strings.Builder
	builder.Grow(len(data))

	for i := 0; i < len(data); {
		if data[i] == 0x1b {
			sequence, consumed, keep := xdfileConsumeManagedANSISequence(data[i:])
			if consumed > 0 {
				if keep {
					builder.Write(sequence)
				}
				i += consumed
				continue
			}
			i++
			continue
		}

		if data[i] < 0x20 && data[i] != '\t' && data[i] != '\n' {
			i++
			continue
		}

		builder.WriteByte(data[i])
		i++
	}

	return builder.String()
}

func xdfileConsumeManagedANSISequence(data []byte) ([]byte, int, bool) {
	if len(data) < 2 || data[0] != 0x1b {
		return nil, 0, false
	}

	switch data[1] {
	case '[':
		for i := 2; i < len(data); i++ {
			if data[i] >= 0x40 && data[i] <= 0x7e {
				if data[i] == 'm' {
					sequence := data[:i+1]
					return sequence, i + 1, xdfileManagedANSIAllowsSGR(sequence)
				}
				return nil, i + 1, false
			}
		}
		return nil, len(data), false
	case ']':
		for i := 2; i < len(data); i++ {
			if data[i] == '\a' {
				return nil, i + 1, false
			}
			if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '\\' {
				return nil, i + 2, false
			}
		}
		return nil, len(data), false
	case 'P', 'X', '^', '_':
		for i := 2; i < len(data); i++ {
			if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '\\' {
				return nil, i + 2, false
			}
		}
		return nil, len(data), false
	default:
		return nil, min(2, len(data)), false
	}
}

func xdfileManagedANSIAllowsSGR(sequence []byte) bool {
	if len(sequence) < 3 || sequence[0] != 0x1b || sequence[1] != '[' || sequence[len(sequence)-1] != 'm' {
		return false
	}

	params := strings.TrimSuffix(string(sequence[2:]), "m")
	if params == "" {
		return true
	}

	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			part = "0"
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return false
		}

		switch {
		case value == 0, value == 1, value == 2, value == 3, value == 4,
			value == 22, value == 23, value == 24,
			value == 30, value == 31, value == 32, value == 33, value == 34, value == 35, value == 36, value == 37,
			value == 39,
			value == 40, value == 41, value == 42, value == 43, value == 44, value == 45, value == 46, value == 47,
			value == 49,
			value == 90, value == 91, value == 92, value == 93, value == 94, value == 95, value == 96, value == 97,
			value == 100, value == 101, value == 102, value == 103, value == 104, value == 105, value == 106, value == 107:
			continue
		case value == 38 || value == 48:
			if i+4 >= len(parts) {
				return false
			}
			mode := strings.TrimSpace(parts[i+1])
			if mode != "2" {
				return false
			}
			for _, rgb := range parts[i+2 : i+5] {
				component, err := strconv.Atoi(strings.TrimSpace(rgb))
				if err != nil || component < 0 || component > 255 {
					return false
				}
			}
			i += 4
		default:
			return false
		}
	}
	return true
}

func xdfileBuiltinCD(dir string, command string) (string, bool, error) {
	command = strings.TrimSpace(command)
	lower := strings.ToLower(command)
	if xdfileIsNetBoxPath(dir) {
		return xdfileBuiltinRemoteCD(dir, command)
	}
	if runtime.GOOS == "windows" && xdfileLooksLikeWindowsDriveCommand(command) {
		return filepath.Clean(command + `\`), true, nil
	}
	if lower == "cd" || lower == "chdir" {
		home, err := os.UserHomeDir()
		return home, true, err
	}
	if !strings.HasPrefix(lower, "cd ") && !strings.HasPrefix(lower, "chdir ") {
		return "", false, nil
	}

	target := strings.TrimSpace(command[2:])
	if strings.HasPrefix(lower, "chdir ") {
		target = strings.TrimSpace(command[5:])
	}
	if runtime.GOOS == "windows" && len(target) >= 2 && strings.EqualFold(target[:2], "/d") {
		target = strings.TrimSpace(target[2:])
	}
	target = strings.Trim(target, "\"'")
	if runtime.GOOS == "windows" && xdfileLooksLikeWindowsDriveCommand(target) {
		target += `\`
	}
	if strings.HasPrefix(target, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return dir, true, err
		}
		target = filepath.Join(home, strings.TrimPrefix(target, "~"))
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	target = filepath.Clean(target)

	info, err := os.Stat(target)
	if err != nil {
		return dir, true, err
	}
	if !info.IsDir() {
		return dir, true, fmt.Errorf("not a directory: %s", target)
	}
	return target, true, nil
}

func xdfileBuiltinRemoteCD(dir string, command string) (string, bool, error) {
	lower := strings.ToLower(strings.TrimSpace(command))
	target := ""
	switch {
	case lower == "cd" || lower == "chdir":
		target = "/"
	case strings.HasPrefix(lower, "cd "):
		target = strings.TrimSpace(command[2:])
	case strings.HasPrefix(lower, "chdir "):
		target = strings.TrimSpace(command[5:])
	default:
		return "", false, nil
	}
	target = strings.Trim(target, "\"'")
	if target == "" {
		target = "/"
	}

	resolved, err := xdfileResolveShellPath(dir, target)
	if err != nil {
		return dir, true, err
	}
	if _, err := xdfileReadEntries(resolved, false, xdfileSortModeName); err != nil {
		return dir, true, err
	}
	return resolved, true, nil
}

func xdfileLooksLikeWindowsDriveCommand(value string) bool {
	if len(value) != 2 || value[1] != ':' {
		return false
	}
	first := value[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
}
