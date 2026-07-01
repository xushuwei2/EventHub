package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERR
	FATAL
)

var levelNames = [...]string{"DEBUG", "INFO", "WARN", "ERR", "FATAL"}

type Logger struct {
	mu       sync.Mutex
	minLevel Level
	logDir   string
	file     *os.File
	curDate  string
}

var defaultLogger = New("log", INFO)

func Default() *Logger { return defaultLogger }

func Init(logDir string, minLevel Level) {
	defaultLogger = New(logDir, minLevel)
}

func New(logDir string, minLevel Level) *Logger {
	_ = os.MkdirAll(logDir, 0o755)
	return &Logger{logDir: logDir, minLevel: minLevel}
}

func (l *Logger) log(level Level, skip int, format string, args ...any) {
	if level < l.minLevel {
		return
	}
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		file, line = "???", 0
	} else {
		file = filepath.Base(file)
	}
	msg := fmt.Sprintf(format, args...)
	now := time.Now()
	entry := fmt.Sprintf("[%s] [%s] [%s:%d] %s\n",
		now.Format("2006-01-02 15:04:05"), levelNames[level], file, line, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	_, _ = os.Stdout.WriteString(entry)

	f, err := l.ensureFile(now)
	if err != nil {
		return
	}
	_, _ = f.WriteString(entry)

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) ensureFile(now time.Time) (*os.File, error) {
	date := now.Format("2006-01-02")
	if l.file != nil && l.curDate == date {
		return l.file, nil
	}
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	path := filepath.Join(l.logDir, date+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	l.file = f
	l.curDate = date
	return f, nil
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

func Debug(format string, args ...any) { defaultLogger.log(DEBUG, 1, format, args...) }
func Info(format string, args ...any)  { defaultLogger.log(INFO, 1, format, args...) }
func Warn(format string, args ...any)  { defaultLogger.log(WARN, 1, format, args...) }
func Error(format string, args ...any) { defaultLogger.log(ERR, 1, format, args...) }
func Fatal(format string, args ...any) { defaultLogger.log(FATAL, 1, format, args...) }

func Close() { defaultLogger.Close() }

func WriteCrashLog(reason string, detail string) {
	logDir := defaultLogger.logDir
	if logDir == "" {
		logDir = "log"
	}
	_ = os.MkdirAll(logDir, 0o755)
	path := filepath.Join(logDir, time.Now().Format("2006-01-02")+"_crash.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	now := time.Now().Format("2006-01-02 15:04:05")
	_, _ = io.WriteString(f, fmt.Sprintf("[%s] [FATAL] [crash] %s\n%s\n", now, reason, detail))
}

func RecoverAndLog() {
	if r := recover(); r != nil {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		WriteCrashLog(fmt.Sprint(r), string(buf[:n]))
		os.Exit(1)
	}
}
