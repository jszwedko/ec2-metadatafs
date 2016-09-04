package logging

// Heavily inspired by logrus, but only the bare minimum needed

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
)

// Level is the log level
type Level uint8

const (
	FatalLevel Level = iota
	ErrorLevel
	WarningLevel
	InfoLevel
	DebugLevel
)

// String returns the string representation of the levels (used when logging)
func (level Level) String() string {
	switch level {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarningLevel:
		return "WARNING"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	}

	return "UNKNOWN"
}

// Logger is a leveled logger capable of optionally logging to syslog
type Logger struct {
	MinLevel Level

	syslogEnabled bool
	syslogWriter  *syslog.Writer

	stdlibLogger *log.Logger
}

// NewLogger returns a new logger with levels
func NewLogger() *Logger {
	return &Logger{
		MinLevel:     InfoLevel,
		stdlibLogger: log.New(os.Stderr, "", log.LstdFlags),
	}
}

// EnableSyslog enables sending of log messages to syslog
//
// It returns any errors occurring while opening the connection (it does not
// enable syslog logging if there are errors)
func (l *Logger) EnableSyslog(facility syslog.Priority) (err error) {
	l.syslogWriter, err = syslog.New(syslog.LOG_NOTICE|syslog.LOG_USER, "ec2-metadatafs")
	if err != nil {
		return err
	}
	l.syslogEnabled = true
	return nil
}

// DisableSyslog always disables writing to syslog, but returns an errors from
// closing the connection
func (l *Logger) DisableSyslog() error {
	l.syslogEnabled = false
	if l.syslogWriter == nil {
		return nil
	}
	return l.syslogWriter.Close()
}

// Fatalf logs a fatal message and exits
func (l *Logger) Fatalf(m string, args ...interface{}) {
	l.Logf(FatalLevel, m, args...)
	os.Exit(1)
}

// Debugf logs a debug message
func (l *Logger) Debugf(m string, args ...interface{}) {
	l.Logf(DebugLevel, m, args...)
}

// Errorf logs an error message
func (l *Logger) Errorf(m string, args ...interface{}) {
	l.Logf(ErrorLevel, m, args...)
}

// Warningf logs a warning message
func (l *Logger) Warningf(m string, args ...interface{}) {
	l.Logf(WarningLevel, m, args...)
}

// Infof logs an info message
func (l *Logger) Infof(m string, args ...interface{}) {
	l.Logf(InfoLevel, m, args...)
}

// Logf logs a message with the given level
func (l *Logger) Logf(level Level, m string, args ...interface{}) {
	if l.MinLevel < level {
		return
	}

	message := fmt.Sprintf(m, args...)
	l.stdlibLogger.Printf("[%s] %s", level, message)

	if l.syslogEnabled {
		l.sendToSyslog(level, message)
	}
}

// Writer returns a io.Writer that results in leveled log output using the
// given level
//
// Caller is expected to close the writer
func (logger *Logger) Writer(level Level) io.WriteCloser {
	reader, writer := io.Pipe()

	var printFunc func(m string, args ...interface{})
	switch level {
	case DebugLevel:
		printFunc = logger.Debugf
	case InfoLevel:
		printFunc = logger.Infof
	case WarningLevel:
		printFunc = logger.Warningf
	case ErrorLevel:
		printFunc = logger.Errorf
	case FatalLevel:
		printFunc = logger.Fatalf
	default:
		printFunc = logger.Infof
	}

	go logger.writerScanner(reader, printFunc)

	return writer
}

// Close closes any open connections the logger has
func (l *Logger) Close() error {
	return l.DisableSyslog()
}

func (logger *Logger) writerScanner(reader *io.PipeReader, printFunc func(m string, args ...interface{})) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		printFunc(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		logger.Errorf("Error while reading from Writer: %s", err)
	}
	reader.Close()
}

func (l *Logger) sendToSyslog(level Level, m string) error {
	switch level {
	case DebugLevel:
		return l.syslogWriter.Debug(m)
	case InfoLevel:
		return l.syslogWriter.Info(m)
	case WarningLevel:
		return l.syslogWriter.Warning(m)
	case ErrorLevel:
		return l.syslogWriter.Err(m)
	case FatalLevel:
		return l.syslogWriter.Crit(m)
	default:
		return l.syslogWriter.Info(m)
	}
}
