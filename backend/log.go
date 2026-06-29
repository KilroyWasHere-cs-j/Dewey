package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Log levels
const (
	DEBUG = "DEBUG"
	INFO  = "INFO "
	WARN  = "WARN "
	FATAL = "FATAL"
)

// ANSI color codes
const (
	ColorReset = "\033[0m"

	// Source tag — bold cyan so [DEWEY] pops against other log sources in a shared terminal
	ColorTag = "\033[1;36m"

	// Per-level colors for the terminal badge
	ColorDebug = "\033[90m"   // dark gray  — low noise
	ColorInfo  = "\033[32m"   // green
	ColorWarn  = "\033[1;33m" // bold yellow
	ColorFatal = "\033[1;31m" // bold red
)

// Logger struct with rotation support
type Logger struct {
	dir      string
	baseName string
	file     *os.File
	curDate  string
	mu       sync.Mutex
}

// Global logger
var logger *Logger

// Initialize global logger
func InitLogger(dir, baseName string) {
	logger = NewLogger(dir, baseName)
}

// Create a new logger
func NewLogger(dir, baseName string) *Logger {
	l := &Logger{
		dir:      dir,
		baseName: baseName,
	}
	err := l.rotateIfNeeded()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	return l
}

// Create filename like: logs/app-2026-04-07.log
func (l *Logger) getFilename(date string) string {
	filename := fmt.Sprintf("%s-%s.log", l.baseName, date)
	return filepath.Join(l.dir, filename)
}

// Rotate file if date changed
func (l *Logger) rotateIfNeeded() error {
	today := time.Now().Format("2006-01-02")

	if l.file != nil && today == l.curDate {
		return nil
	}

	// Close old file
	if l.file != nil {
		l.file.Close()
	}

	// Ensure directory exists
	err := os.MkdirAll(l.dir, os.ModePerm)
	if err != nil {
		return err
	}

	// Open new file
	filename := l.getFilename(today)
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	l.file = file
	l.curDate = today
	return nil
}

// Close logger
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.file.Close()
	}
}

// Core logging function (thread-safe + rotation)
func (l *Logger) log(level, message, color string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotateIfNeeded(); err != nil {
		log.Printf("Log rotation failed: %v", err)
	}

	now := time.Now()

	// File: full timestamp, plain text, no ANSI
	fileLine := fmt.Sprintf("[%s] [%s] %s\n", now.Format("2006-01-02 15:04:05"), level, message)
	if l.file != nil {
		if _, err := l.file.WriteString(fileLine); err != nil {
			log.Printf("Error writing to log file: %v", err)
		}
	}

	// Terminal: bold cyan source tag + short time + colored level badge
	// Format: [DEWEY] 15:04:05  LEVEL  message
	fmt.Printf(
		"%s[DEWEY]%s %s  %s%s%s  %s\n",
		ColorTag, ColorReset,
		now.Format("15:04:05"),
		color, level, ColorReset,
		message,
	)
}

// Safety check
func ensureLogger() bool {
	if logger == nil {
		log.Println("Logger not initialized")
		return false
	}
	return true
}

// Public functions
func Debug(msg string) {
	if ensureLogger() {
		logger.log(DEBUG, msg, ColorDebug)
	}
}

func Info(msg string) {
	if ensureLogger() {
		logger.log(INFO, msg, ColorInfo)
	}
}

func Warn(msg string) {
	if ensureLogger() {
		logger.log(WARN, msg, ColorWarn)
	}
}

func Fatal(msg string) {
	if ensureLogger() {
		logger.log(FATAL, msg, ColorFatal)
		logger.Close()
	}
	os.Exit(1)
}

// Banner prints the Dewey startup header. Call once after InitLogger.
func Banner() {
	c := ColorTag
	r := ColorReset
	fmt.Println()
	fmt.Printf("  %s╔══════════════════════════════════╗%s\n", c, r)
	fmt.Printf("  %s║          D E W E Y               ║%s\n", c, r)
	fmt.Printf("  %s║   document management system      ║%s\n", c, r)
	fmt.Printf("  %s╚══════════════════════════════════╝%s\n", c, r)
	fmt.Println()
}

// Section prints a labelled divider to visually separate startup phases.
func Section(title string) {
	rule := strings.Repeat("─", 16)
	fmt.Printf("\n  %s%s  %s  %s%s\n\n", ColorTag, rule, strings.ToUpper(title), rule, ColorReset)
}

// Ok prints a green check with a message — use for key success events (plugin loaded, db ready, etc.).
func Ok(msg string) {
	fmt.Printf("  %s✓%s  %s\n", ColorInfo, ColorReset, msg)
}

