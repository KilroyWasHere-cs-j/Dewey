package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Log levels
const (
	DEBUG = "DEBUG"
	INFO  = "INFO"
	WARN  = "WARN"
	FATAL = "FATAL"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorGray   = "\033[37m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorRed    = "\033[31m"
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

// Timestamp
func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// Core logging function (thread-safe + rotation)
func (l *Logger) log(level string, message string, color string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Rotate if needed
	if err := l.rotateIfNeeded(); err != nil {
		log.Printf("Log rotation failed: %v", err)
	}

	ts := timestamp()
	formatted := fmt.Sprintf("[%s] [%s] %s\n", ts, level, message)

	// Write to file
	if l.file != nil {
		_, err := l.file.WriteString(formatted)
		if err != nil {
			log.Printf("Error writing to log file: %v", err)
		}
	}

	// Console output
	fmt.Printf("%s%s%s", color, formatted, ColorReset)
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
		logger.log(DEBUG, msg, ColorGray)
	}
}

func Info(msg string) {
	if ensureLogger() {
		logger.log(INFO, msg, ColorGreen)
	}
}

func Warn(msg string) {
	if ensureLogger() {
		warns_logged++
		logger.log(WARN, msg, ColorYellow)
	}
}

func Fatal(msg string) {
	if ensureLogger() {
		logger.log(FATAL, msg, ColorRed)
		logger.Close()
	}
	os.Exit(1)
}
