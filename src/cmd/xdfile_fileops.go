package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

type xdfileFileOperationKind string

const (
	xdfileFileOperationCopy   xdfileFileOperationKind = "copy"
	xdfileFileOperationMove   xdfileFileOperationKind = "move"
	xdfileFileOperationDelete xdfileFileOperationKind = "delete"
	xdfileFileOperationRename xdfileFileOperationKind = "rename"
	xdfileFileOperationMkdir  xdfileFileOperationKind = "mkdir"
)

type xdfileFileOperation struct {
	Kind        xdfileFileOperationKind
	SourcePath  string
	SourcePaths []string
	TargetPath  string
	PanelIndex  int
}

type xdfileFileOperationFailure struct {
	SourcePath string
	TargetPath string
	Err        error
}

type xdfileFileOperationDoneMsg struct {
	Operation   xdfileFileOperation
	DeleteBatch xdfileDeleteUndoBatch
	Count       int
	Failures    []xdfileFileOperationFailure
	Err         error
}

type xdfileFileOperationProgress struct {
	Items atomic.Int64
	Bytes atomic.Int64
}

func (p *xdfileFileOperationProgress) addItem() {
	if p != nil {
		p.Items.Add(1)
	}
}

func (p *xdfileFileOperationProgress) addBytes(n int) {
	if p != nil && n > 0 {
		p.Bytes.Add(int64(n))
	}
}

func (m *xdfileModel) startFileOperation(op xdfileFileOperation) tea.Cmd {
	op = xdfileNormalizeFileOperation(op)

	if m.fileOperationActive() {
		m.fileOperationQueue = append(m.fileOperationQueue, op)
		m.setStatus("Queued %s (%d pending)", op.queueLabel(), len(m.fileOperationQueue))
		return nil
	}

	if m.backgroundTaskBusy {
		m.setStatus("Wait for the current background task to finish")
		return nil
	}

	return m.startFileOperationNow(op)
}

func (m *xdfileModel) startFileOperationNow(op xdfileFileOperation) tea.Cmd {
	op = xdfileNormalizeFileOperation(op)

	op.SourcePath = strings.TrimSpace(op.SourcePath)
	op.TargetPath = strings.TrimSpace(op.TargetPath)
	op.SourcePaths = append([]string(nil), op.SourcePaths...)

	ctx, cancel := context.WithCancel(context.Background())
	progress := &xdfileFileOperationProgress{}
	m.fileOperationCancel = cancel
	m.fileOperationProgress = progress

	m.setStatus("%s (Ctrl+C to cancel%s)", op.runningStatus(), m.fileOperationQueueSuffix())
	return tea.Batch(xdfileRunFileOperationCmd(ctx, op, progress), m.startBackgroundTask())
}

func xdfileRunFileOperationCmd(ctx context.Context, op xdfileFileOperation, progress *xdfileFileOperationProgress) tea.Cmd {
	return func() tea.Msg {
		msg := xdfileFileOperationDoneMsg{Operation: op}
		switch op.Kind {
		case xdfileFileOperationCopy:
			msg.Count, msg.Failures, msg.Err = xdfileRunTransferFileOperation(ctx, op, progress, false)
		case xdfileFileOperationMove:
			msg.Count, msg.Failures, msg.Err = xdfileRunTransferFileOperation(ctx, op, progress, true)
		case xdfileFileOperationDelete:
			paths := append([]string(nil), op.SourcePaths...)
			if len(paths) == 0 && op.SourcePath != "" {
				paths = []string{op.SourcePath}
			}
			msg.DeleteBatch, msg.Failures, msg.Err = xdfileStageDeletePathsContext(ctx, paths, progress)
			if msg.Err == nil {
				msg.Count = len(msg.DeleteBatch.Items)
			}
		case xdfileFileOperationRename:
			msg.Err = xdfileRenamePath(op.SourcePath, op.TargetPath)
			if msg.Err == nil {
				msg.Count = 1
			}
		case xdfileFileOperationMkdir:
			msg.Err = xdfileMkdirPath(op.TargetPath)
			if msg.Err == nil {
				msg.Count = 1
			}
		default:
			msg.Err = fmt.Errorf("unsupported file operation: %s", op.Kind)
		}
		return msg
	}
}

func (m *xdfileModel) applyFileOperationDone(msg xdfileFileOperationDoneMsg) tea.Cmd {
	m.stopBackgroundTask()
	m.fileOperationCancel = nil
	m.fileOperationProgress = nil

	if msg.Operation.Kind == xdfileFileOperationDelete && msg.Count > 0 {
		m.pushDeleteUndoBatch(msg.DeleteBatch)
	}

	m.reloadAfterFileOperation(msg.Operation)

	if msg.Err != nil {
		if errors.Is(msg.Err, context.Canceled) {
			if msg.Count > 0 {
				m.setStatus("File operation canceled after %d item(s)", msg.Count)
			} else {
				m.setStatus("File operation canceled")
			}
			return m.startNextQueuedFileOperation()
		}
		if len(msg.Failures) > 0 {
			m.openFileOperationErrorModal(msg)
			return nil
		}
		m.setStatusErr(msg.Err)
		return m.startNextQueuedFileOperation()
	}

	if msg.Operation.Kind == xdfileFileOperationDelete && msg.Count == 0 && len(msg.Failures) == 0 {
		m.setStatus("Select a file or directory to delete")
		return m.startNextQueuedFileOperation()
	}

	if len(msg.Failures) > 0 {
		if msg.Count == 0 {
			m.openFileOperationErrorModal(msg)
			return nil
		}
		m.openFileOperationErrorModal(msg)
		return nil
	}

	m.setStatus("%s", msg.Operation.doneStatus(msg.Count))
	return m.startNextQueuedFileOperation()
}

func xdfileRunTransferFileOperation(
	ctx context.Context,
	op xdfileFileOperation,
	progress *xdfileFileOperationProgress,
	move bool,
) (int, []xdfileFileOperationFailure, error) {
	sources := op.transferSources()
	if len(sources) == 0 {
		return 0, nil, nil
	}

	if len(sources) == 1 {
		err := xdfileRunSingleTransfer(ctx, sources[0], op.TargetPath, progress, move)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return 0, nil, err
			}
			return 0, []xdfileFileOperationFailure{
				{
					SourcePath: sources[0],
					TargetPath: op.TargetPath,
					Err:        err,
				},
			}, nil
		}
		return 1, nil, nil
	}

	successes := 0
	failures := make([]xdfileFileOperationFailure, 0)
	for _, sourcePath := range sources {
		if err := xdfileCheckFileOperationContext(ctx); err != nil {
			return successes, failures, err
		}

		targetPath := filepath.Join(op.TargetPath, filepath.Base(sourcePath))
		if !move {
			target, err := xdfileUniqueCopyTarget(targetPath)
			if err != nil {
				failures = append(failures, xdfileFileOperationFailure{
					SourcePath: sourcePath,
					TargetPath: targetPath,
					Err:        err,
				})
				continue
			}
			targetPath = target
		}

		err := xdfileRunSingleTransfer(ctx, sourcePath, targetPath, progress, move)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return successes, failures, err
			}
			failures = append(failures, xdfileFileOperationFailure{
				SourcePath: sourcePath,
				TargetPath: targetPath,
				Err:        err,
			})
			continue
		}
		successes++
	}
	return successes, failures, nil
}

func xdfileRunSingleTransfer(
	ctx context.Context,
	sourcePath string,
	targetPath string,
	progress *xdfileFileOperationProgress,
	move bool,
) error {
	if move {
		return xdfileMovePathContext(ctx, sourcePath, targetPath, progress)
	}
	return xdfileCopyPathContext(ctx, sourcePath, targetPath, progress)
}

func (op xdfileFileOperation) transferSources() []string {
	if len(op.SourcePaths) > 0 {
		sources := make([]string, 0, len(op.SourcePaths))
		for _, source := range op.SourcePaths {
			source = strings.TrimSpace(source)
			if source != "" {
				sources = append(sources, source)
			}
		}
		return sources
	}
	if strings.TrimSpace(op.SourcePath) == "" {
		return nil
	}
	return []string{op.SourcePath}
}

func (m *xdfileModel) cancelFileOperationIfBusy() bool {
	if m == nil || m.fileOperationCancel == nil {
		return false
	}
	cancel := m.fileOperationCancel
	pending := len(m.fileOperationQueue)
	m.fileOperationCancel = nil
	m.fileOperationQueue = nil
	cancel()
	if pending > 0 {
		m.setStatus("Canceling file operation and %d queued task(s)...", pending)
	} else {
		m.setStatus("Canceling file operation...")
	}
	return true
}

func (m *xdfileModel) fileOperationActive() bool {
	return m != nil && m.fileOperationCancel != nil
}

func (m *xdfileModel) startNextQueuedFileOperation() tea.Cmd {
	if m == nil || len(m.fileOperationQueue) == 0 || m.backgroundTaskBusy || m.modal.Kind != xdfileModalNone {
		return nil
	}

	next := m.fileOperationQueue[0]
	copy(m.fileOperationQueue, m.fileOperationQueue[1:])
	m.fileOperationQueue[len(m.fileOperationQueue)-1] = xdfileFileOperation{}
	m.fileOperationQueue = m.fileOperationQueue[:len(m.fileOperationQueue)-1]
	return m.startFileOperationNow(next)
}

func (m *xdfileModel) fileOperationProgressStatus() string {
	if m == nil || m.fileOperationProgress == nil || !m.backgroundTaskBusy {
		return ""
	}

	items := m.fileOperationProgress.Items.Load()
	bytes := m.fileOperationProgress.Bytes.Load()
	pending := len(m.fileOperationQueue)
	pendingLabel := ""
	if pending > 0 {
		pendingLabel = fmt.Sprintf(", %d queued", pending)
	}
	switch {
	case bytes > 0 && items > 0:
		return fmt.Sprintf("[%s, %d items%s]", xdfileHumanSize(bytes), items, pendingLabel)
	case bytes > 0:
		return fmt.Sprintf("[%s%s]", xdfileHumanSize(bytes), pendingLabel)
	case items > 0:
		return fmt.Sprintf("[%d items%s]", items, pendingLabel)
	case pending > 0:
		return fmt.Sprintf("[%d queued]", pending)
	default:
		return ""
	}
}

func (m *xdfileModel) reloadAfterFileOperation(op xdfileFileOperation) {
	switch op.Kind {
	case xdfileFileOperationRename, xdfileFileOperationMkdir:
		if m.validPanelIndex(op.PanelIndex) {
			if err := m.reloadPanel(op.PanelIndex); err != nil {
				m.setStatusErr(err)
			}
			return
		}
	}
	m.reloadAllPanels()
}

func (m *xdfileModel) openFileOperationErrorModal(msg xdfileFileOperationDoneMsg) {
	summary := msg.Operation.partialStatus(msg.Count, len(msg.Failures))
	lines := []string{summary, ""}
	limit := min(len(msg.Failures), 60)
	for i := 0; i < limit; i++ {
		failure := msg.Failures[i]
		line := "- " + failure.SourcePath
		if failure.TargetPath != "" {
			line += " -> " + failure.TargetPath
		}
		if failure.Err != nil {
			line += ": " + failure.Err.Error()
		}
		lines = append(lines, line)
	}
	if len(msg.Failures) > limit {
		lines = append(lines, "", fmt.Sprintf("... %d more failed item(s)", len(msg.Failures)-limit))
	}
	m.openTextModalWithAction(xdfileActionFileOperationErrors, "File Operation Errors", strings.Join(lines, "\n"), "Enter/Esc close | Up/Down/PgUp/PgDn scroll")
	m.setStatusErr(fmt.Errorf("%s", summary))
}

func (op xdfileFileOperation) runningStatus() string {
	sourceCount := len(op.transferSources())
	switch op.Kind {
	case xdfileFileOperationCopy:
		if sourceCount > 1 {
			return fmt.Sprintf("Copying %d items", sourceCount)
		}
		return fmt.Sprintf("Copying %s", xdfileFileOperationBase(op.SourcePath))
	case xdfileFileOperationMove:
		if sourceCount > 1 {
			return fmt.Sprintf("Moving %d items", sourceCount)
		}
		return fmt.Sprintf("Moving %s", xdfileFileOperationBase(op.SourcePath))
	case xdfileFileOperationDelete:
		count := len(op.SourcePaths)
		if count > 1 {
			return fmt.Sprintf("Deleting %d items", count)
		}
		return fmt.Sprintf("Deleting %s", xdfileFileOperationBase(op.SourcePath))
	case xdfileFileOperationRename:
		return fmt.Sprintf("Renaming %s", xdfileFileOperationBase(op.SourcePath))
	case xdfileFileOperationMkdir:
		return fmt.Sprintf("Creating %s", xdfileFileOperationBase(op.TargetPath))
	default:
		return "Running file operation"
	}
}

func (op xdfileFileOperation) doneStatus(count int) string {
	switch op.Kind {
	case xdfileFileOperationCopy:
		if len(op.transferSources()) > 1 {
			return fmt.Sprintf("Copied %d item(s) to %s", count, op.TargetPath)
		}
		return fmt.Sprintf("Copied to %s", op.TargetPath)
	case xdfileFileOperationMove:
		if len(op.transferSources()) > 1 {
			return fmt.Sprintf("Moved %d item(s) to %s", count, op.TargetPath)
		}
		return fmt.Sprintf("Moved to %s", op.TargetPath)
	case xdfileFileOperationDelete:
		label := op.SourcePath
		if count > 1 {
			label = fmt.Sprintf("%d items", count)
		}
		return fmt.Sprintf("Deleted %s (Ctrl+Z to undo)", label)
	case xdfileFileOperationRename:
		return fmt.Sprintf("Renamed to %s", filepath.Base(op.TargetPath))
	case xdfileFileOperationMkdir:
		return fmt.Sprintf("Created %s", op.TargetPath)
	default:
		return "File operation completed"
	}
}

func (op xdfileFileOperation) partialStatus(successCount int, failureCount int) string {
	switch op.Kind {
	case xdfileFileOperationCopy:
		return fmt.Sprintf("Copied %d item(s), %d failed", successCount, failureCount)
	case xdfileFileOperationMove:
		return fmt.Sprintf("Moved %d item(s), %d failed", successCount, failureCount)
	case xdfileFileOperationDelete:
		return fmt.Sprintf("Deleted %d item(s), %d failed", successCount, failureCount)
	default:
		return fmt.Sprintf("Completed %d item(s), %d failed", successCount, failureCount)
	}
}

func xdfileFileOperationBase(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "item"
	}
	return filepath.Base(path)
}

func xdfileNormalizeFileOperation(op xdfileFileOperation) xdfileFileOperation {
	op.SourcePath = strings.TrimSpace(op.SourcePath)
	op.TargetPath = strings.TrimSpace(op.TargetPath)
	if len(op.SourcePaths) > 0 {
		sources := make([]string, 0, len(op.SourcePaths))
		for _, source := range op.SourcePaths {
			source = strings.TrimSpace(source)
			if source != "" {
				sources = append(sources, source)
			}
		}
		op.SourcePaths = sources
	}
	return op
}

func (m *xdfileModel) fileOperationQueueSuffix() string {
	if m == nil || len(m.fileOperationQueue) == 0 {
		return ""
	}
	return fmt.Sprintf(", %d queued", len(m.fileOperationQueue))
}

func (op xdfileFileOperation) queueLabel() string {
	switch op.Kind {
	case xdfileFileOperationCopy:
		return "copy"
	case xdfileFileOperationMove:
		return "move"
	case xdfileFileOperationDelete:
		return "delete"
	case xdfileFileOperationRename:
		return "rename"
	case xdfileFileOperationMkdir:
		return "mkdir"
	default:
		return "file operation"
	}
}
