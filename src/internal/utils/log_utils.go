package utils

import (
	"fmt"
	"log/slog"
	"os"
)

var rootLogFile *os.File

func resetRootLoggerFile() {
	if rootLogFile != nil {
		_ = rootLogFile.Close()
		rootLogFile = nil
	}
}

// Print line to stderr and exit with status 1
// Cannot use log.Fataln() as slog.SetDefault() causes those lines to
// go into log file
func PrintlnAndExit(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

// Print formatted output line to stderr and exit with status 1
// Cannot use log.Fataln() as slog.SetDefault() causes those lines to
// go into log file
func PrintfAndExitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

// Used in unit test
func SetRootLoggerToStdout(debug bool) {
	resetRootLoggerFile()
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{Level: level})))
}

func SetRootLoggerToFile(path string, debug bool) error {
	resetRootLoggerFile()
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, LogFilePerm)
	if err != nil {
		return err
	}

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	rootLogFile = file
	slog.SetDefault(slog.New(slog.NewTextHandler(
		file, &slog.HandlerOptions{Level: level})))
	return nil
}

// Used in unit test
func SetRootLoggerToDiscarded() {
	resetRootLoggerFile()
	slog.SetDefault(slog.New(slog.DiscardHandler))
}
