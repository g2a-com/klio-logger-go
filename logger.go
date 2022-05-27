// This logger is meant to be used for building Klio commands (https://github.com/g2a-com/klio).
// It writes logs decorated with control sequences interpreted by Klio (https://github.com/g2a-com/klio/blob/main/docs/output-handling.md).
// It doesn't filter or modify messages besides that.

package logger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Level type.
type Level string
type Mode string

const (
	// FatalLevel level. Errors causing a command to exit immediately.
	FatalLevel Level = "fatal"
	// ErrorLevel level. Errors which cause a command to fail, but not immediately.
	ErrorLevel Level = "error"
	// WarnLevel level. Information about unexpected situations and minor errors
	// (not causing a command to fail).
	WarnLevel Level = "warn"
	// InfoLevel level. Generally useful information (things happen).
	InfoLevel Level = "info"
	// VerboseLevel level. More granular but still useful information.
	VerboseLevel Level = "verbose"
	// DebugLevel level. Information helpful for command developers.
	DebugLevel Level = "debug"
	// SpamLevel level. Give me EVERYTHING.
	SpamLevel Level = "spam"
	// DefaultLevel is an alias for "info" level.
	DefaultLevel = InfoLevel
	// LineMode tells klio log parser to decorate log sequence according to configuration
	LineMode Mode = "line"
	// RawMode tells klio log parser to leave the log sequence without any decoration
	RawMode Mode = "raw"
	// DefaultMode is an alias for line mode.
	DefaultMode = LineMode
)

var (
	standardLogger = NewMutable(os.Stdout)
	errorLogger    = NewMutable(os.Stderr)
	levelsMap      = map[string]Level{
		string(FatalLevel):   FatalLevel,
		string(ErrorLevel):   ErrorLevel,
		string(WarnLevel):    WarnLevel,
		string(InfoLevel):    InfoLevel,
		string(VerboseLevel): VerboseLevel,
		string(DebugLevel):   DebugLevel,
		string(SpamLevel):    SpamLevel,
	}
)

func init() {
	errorLogger.SetLevel(ErrorLevel)
}

// ParseLevel converts level name to Level. It is case insensitive, returns
// DefaultLevel if value cannot be converted.
func ParseLevel(s string) (level Level, ok bool) {
	level, ok = levelsMap[strings.ToLower(s)]
	if !ok {
		level = DefaultLevel
	}
	return level, ok
}

// Logger interface. All methods dedicated to change something don't alter
// existing logger instance, they create new instance instead.
type Logger interface {
	io.Writer
	// Printf writes log line. Arguments are handled in the manner of fmt.Print.
	Print(...interface{}) Logger
	// Printf writes log line. Arguments are handled in the manner of fmt.Printf.
	Printf(string, ...interface{}) Logger
	// WithLevel creates new logger instance logging at specified level. It
	// doesn't change existing logger instance.
	WithLevel(Level) Logger
	// Level returns log level used by a logger.
	Level() Level
	// WithTags creates new logger instance with specified tags. Tags are
	// prepended to each line produced by a logger. It doesn't change existing
	// logger instance.
	WithTags(...string) Logger
	// Tags returns tags used by a logger. Tags are prepended to each line
	// produced by a logger.
	Tags() []string
	// WithOutput creates new logger instance using specified Writer to print
	// logs. It doesn't change existing logger instance.
	WithOutput(io.Writer) Logger
	// Output returns writer used by a logger.
	Output() io.Writer
	// Mode returns printing mode used by a logger.
	Mode() Mode
	// WithMode creates new logger instance logging with a specified mode. It
	// doesn't change existing logger instance.
	WithMode(mode Mode) Logger
}

// MutableLogger is the same as a Logger, but it can be altered.
type MutableLogger interface {
	Logger
	// SetOutput changes Writer used to print logs. It modifies logger instance
	// instead creating a new one.
	SetOutput(io.Writer)
	// SetLevel changes level at which logs ar produced. It modifies existing
	// logger instance instead of creating new one.
	SetLevel(Level)
	// SetTags changes tags used to decorate each line produced by logger. It
	// modifies existing logger instance instead of creating new one.
	SetTags(...string)
	// SetMode changes mode with which logs ar produced. It modifies existing
	// logger instance instead of creating new one.
	SetMode(mode Mode)
}

type logger struct {
	output     io.Writer
	tags       []string
	level      Level
	linePrefix string
	mode       Mode
}

// New creates new instance of the Logger.
func New(output io.Writer) Logger {
	l := &logger{
		output: output,
		tags:   []string{},
		level:  DefaultLevel,
		mode:   DefaultMode,
	}

	l.updateLinePrefix()

	return l
}

func (l *logger) updateLinePrefix() {
	level, err := json.Marshal(l.level)
	if err != nil {
		level = []byte("\"" + DefaultLevel + "\"")
	}
	mode, err := json.Marshal(l.mode)
	if err != nil {
		mode = []byte("\"" + DefaultMode + "\"")
	}
	tags, err := json.Marshal(l.tags)
	if err != nil || string(tags) == "null" {
		tags = []byte("[]")
	}
	l.linePrefix = fmt.Sprintf(
		"\033_klio_log_level %s\033\\\033_klio_tags %s\033\\\033_klio_mode %s\033\\", level, tags, mode,
	)
}

func (l *logger) Tags() []string {
	r := make([]string, len(l.tags))
	copy(r, l.tags)
	return r
}

func (l *logger) Level() Level {
	return l.level
}

func (l *logger) Mode() Mode {
	return l.mode
}

func (l *logger) Output() io.Writer {
	return l.output
}

func (l *logger) WithLevel(level Level) Logger {
	n := *l
	n.level = level
	n.updateLinePrefix()
	return &n
}

func (l *logger) WithMode(mode Mode) Logger {
	n := *l
	n.mode = mode
	n.updateLinePrefix()
	return &n
}

func (l *logger) WithTags(tags ...string) Logger {
	n := *l
	n.tags = tags
	n.updateLinePrefix()
	return &n
}

func (l *logger) WithOutput(output io.Writer) Logger {
	n := *l
	n.output = output
	return &n
}

func (l *logger) Print(v ...interface{}) Logger {
	line := l.linePrefix + fmt.Sprint(v...) + "\033_klio_reset\033\\\n"
	l.output.Write([]byte(line))
	return l
}

func (l *logger) Printf(format string, v ...interface{}) Logger {
	return l.Print(fmt.Sprintf(format, v...))
}

func (l *logger) Write(p []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(p)) // Scan lines
	for scanner.Scan() {
		l.Print(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return len(p), nil
}

type mutableLogger struct {
	*logger
}

// New creates new instance of the MutableLogger.
func NewMutable(output io.Writer) MutableLogger {
	l := &mutableLogger{
		&logger{
			output: output,
			tags:   []string{},
			level:  DefaultLevel,
		},
	}

	l.updateLinePrefix()

	return l
}

// StandardLogger returns global mutable logger instance for writing non-error logs. By default it writes to stdout at "info" level.
func StandardLogger() MutableLogger {
	return standardLogger
}

// ErrorLogger returns global mutable logger instance for writing error logs. By default it writes to stderr at "error" level.
func ErrorLogger() MutableLogger {
	return errorLogger
}

func (l *mutableLogger) SetTags(tags ...string) {
	l.tags = tags
	l.updateLinePrefix()
}

func (l *mutableLogger) SetLevel(level Level) {
	l.level = level
	l.updateLinePrefix()
}

func (l *mutableLogger) SetOutput(output io.Writer) {
	l.output = output
}

func (l *mutableLogger) SetMode(mode Mode) {
	l.mode = mode
	l.updateLinePrefix()
}

// Spam writes a message at level Spam on the standard logger. Arguments are handled in the manner of fmt.Print.
func Spam(v ...interface{}) {
	standardLogger.WithLevel(SpamLevel).Print(v...)
}

// Debug writes a message at level Debug on the standard logger. Arguments are handled in the manner of fmt.Print.
func Debug(v ...interface{}) {
	standardLogger.WithLevel(DebugLevel).Print(v...)
}

// Verbose writes a message at level Verbose on the standard logger. Arguments are handled in the manner of fmt.Print.
func Verbose(v ...interface{}) {
	standardLogger.WithLevel(VerboseLevel).Print(v...)
}

// Info writes a message at level Info on the standard logger. Arguments are handled in the manner of fmt.Print.
func Info(v ...interface{}) {
	standardLogger.WithLevel(InfoLevel).Print(v...)
}

// Warn writes a message at level Warn on the standard logger. Arguments are handled in the manner of fmt.Print.
func Warn(v ...interface{}) {
	standardLogger.WithLevel(WarnLevel).Print(v...)
}

// Error writes a message at level Error on the standard logger. Arguments are handled in the manner of fmt.Print.
func Error(v ...interface{}) {
	standardLogger.WithLevel(ErrorLevel).Print(v...)
}

// Fatal writes a message at level Fatal on the standard logger. Arguments are handled in the manner of fmt.Print.
func Fatal(v ...interface{}) {
	standardLogger.WithLevel(FatalLevel).Print(v...)
}

// Spamf writes a message at level Spam on the standard logger. Arguments are handled in the manner of fmt.Printf.
func Spamf(format string, v ...interface{}) {
	standardLogger.WithLevel(SpamLevel).Printf(format, v...)
}

// Debugf writes a message at level Debug on the standard logger. Arguments are handled in the manner of fmt.Printf.
func Debugf(format string, v ...interface{}) {
	standardLogger.WithLevel(DebugLevel).Printf(format, v...)
}

// Verbosef writes a message at level Verbose on the standard logger. Arguments are handled in the manner of fmt.Printf.
func Verbosef(format string, v ...interface{}) {
	standardLogger.WithLevel(VerboseLevel).Printf(format, v...)
}

// Infof writes a message at level Info on the standard logger. Arguments are handled in the manner of fmt.Printf.
func Infof(format string, v ...interface{}) {
	standardLogger.WithLevel(InfoLevel).Printf(format, v...)
}

// Warnf writes a message at level Warn on the standard logger. Arguments are handled in the manner of fmt.Printf.
func Warnf(format string, v ...interface{}) {
	standardLogger.WithLevel(WarnLevel).Printf(format, v...)
}

// Errorf writes a message at level Error on the standard logger. Arguments are handled in the manner of fmt.Printf.
func Errorf(format string, v ...interface{}) {
	standardLogger.WithLevel(ErrorLevel).Printf(format, v...)
}

// Fatalf writes a message at level Fatal on the standard logger. Arguments are handled in the manner of fmt.Printf.
func Fatalf(format string, v ...interface{}) {
	standardLogger.WithLevel(FatalLevel).Printf(format, v...)
}
