// Package log provides structured logging for the service. It follows the
// philosophy that only actionable events should be logged and thus it provides
// an Info level but no Error level. Beyond the info level, a Debug level is
// also provided to aid during development. No other log levels are provided
// till there is proof that they are needed.
//
// See: https://dave.cheney.net/2015/11/05/lets-talk-about-logging
//
// The package is structured in such a way so that the implementation can be
// swapped in the future if there is proof that it is not performant enough.
//
// The logging functions encourage structured logging through fields instead of
// the traditional Printf style which is harder to parse. Each logging function
// allows a message and an optional number of key value pairs which will be
// translated to structured fields. If a value is not provided, the key will be
// ignored.
package log

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

// Logger is a structured logger.
type Logger struct {
	logger *logrus.Logger
}

// New creates a new structured logger. When develop is true, it provides the
// output in logfmt format. Otherwise the output is JSON.
func New(out io.Writer, level string) *Logger {
	var log = logrus.New()
	log.SetOutput(out)

	// 	log.Formatter = &logrus.JSONFormatter{}
	log.Formatter = &logrus.TextFormatter{}
	if level == "debug" {
		log.SetLevel(logrus.DebugLevel)
	}

	// Add aditional fields here
	log.WithFields(logrus.Fields{})

	return &Logger{logger: log}
}

// Info logs at info log level. For each key, a value should also be provided. If
// a value is not provided, the key will be ignored.
func (l *Logger) Info(message string, keyvals ...interface{}) {
	l.logger.WithFields(toMap(keyvals...)).Info(message)
}

// Debug logs at debug log level. For each key, a value should also be provided. If
// a value is not provided, the key will be ignored.
func (l *Logger) Debug(message string, keyvals ...interface{}) {
	l.logger.WithFields(toMap(keyvals...)).Debug(message)
}

// Error logs at error log level. For each key, a value should also be provided. If
// a value is not provided, the key will be ignored.
func (l *Logger) Error(message string, keyvals ...interface{}) {
	l.logger.WithFields(toMap(keyvals...)).Error(message)
}

func toMap(keyvals ...interface{}) map[string]interface{} {
	if len(keyvals)%2 != 0 {
		keyvals = keyvals[:len(keyvals)-1]
	}
	m := make(map[string]interface{}, len(keyvals)/2)
	for i := 0; i < len(keyvals); i += 2 {
		k, v := fmt.Sprint(keyvals[i]), keyvals[i+1]
		m[k] = v
	}
	return m
}
