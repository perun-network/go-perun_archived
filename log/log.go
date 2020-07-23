// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

// Package log implements the logger interface of go-perun. Users are expected
// to pass an implementation of this interface to harmonize go-perun's logging
// with their application logging.
//
// It mimics the interface of logrus, which is go-perun's logger of choice
// It is also possible to pass a simpler logger like the standard library's log
// logger by converting it to a perun logger. Use the Fieldify and Levellify
// factories for that.
package log // import "perun.network/go-perun/log"

import "log"

// logger is the framework logger. Framework users should set this variable to
// their logger with Set(). It is set to the None non-logging logger by
// default.
var logger Logger = new(none)

// Set sets the framework logger. It is set to the none-logger by default. Set
// accepts nil and then sets the none-logger.
func Set(l Logger) {
	if l == nil {
		logger = new(none)
		return
	}
	logger = l
}

// Get returns the currently set framework logger.
func Get() Logger {
	return logger
}

// compile-time check that log.Logger implements a StdLogger
var _ StdLogger = &log.Logger{}

// StdLogger describes the interface of the standard library log package logger.
// It is the base for more complex loggers. A StdLogger can be converted into a
// LevelLogger by wrapping it with a Levellified struct.
type StdLogger interface {
	Printf(format string, args ...interface{})
	Print(...interface{})
	Println(...interface{})

	Fatalf(format string, args ...interface{})
	Fatal(...interface{})
	Fatalln(...interface{})

	Panicf(format string, args ...interface{})
	Panic(...interface{})
	Panicln(...interface{})
}

// LevelLogger is an extension to the StdLogger with different verbosity levels.
type LevelLogger interface {
	StdLogger

	Tracef(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})

	Trace(...interface{})
	Debug(...interface{})
	Info(...interface{})
	Warn(...interface{})
	Error(...interface{})

	Traceln(...interface{})
	Debugln(...interface{})
	Infoln(...interface{})
	Warnln(...interface{})
	Errorln(...interface{})
}

// Fields is a collection of fields that can be passed to FieldLogger.WithFields
type Fields map[string]interface{}

// Logger is a LevelLogger with structured field logging capabilities.
// This is the interface that needs to be passed to go-perun.
type Logger interface {
	LevelLogger

	WithField(key string, value interface{}) Logger
	WithFields(Fields) Logger
	WithError(error) Logger
}

// Printf calls Printf on the global Logger object.
func Printf(format string, args ...interface{}) { logger.Printf(format, args...) }

// Print calls Print on the global Logger object.
func Print(args ...interface{}) { logger.Print(args...) }

// Println calls Println on the global Logger object.
func Println(args ...interface{}) { logger.Println(args...) }

// Fatalf calls Fatalf on the global Logger object.
func Fatalf(format string, args ...interface{}) { logger.Fatalf(format, args...) }

// Fatal calls Fatal on the global Logger object.
func Fatal(args ...interface{}) { logger.Fatal(args...) }

// Fatalln calls Fatalln on the global Logger object.
func Fatalln(args ...interface{}) { logger.Fatalln(args...) }

// Panicf calls Panicf on the global Logger object.
func Panicf(format string, args ...interface{}) { logger.Panicf(format, args...) }

// Panic calls Panic on the global Logger object.
func Panic(args ...interface{}) { logger.Panic(args...) }

// Panicln calls Panicln on the global Logger object.
func Panicln(args ...interface{}) { logger.Panicln(args...) }

// Tracef calls Tracef on the global Logger object.
func Tracef(format string, args ...interface{}) { logger.Tracef(format, args...) }

// Trace calls Trace on the global Logger object.
func Trace(args ...interface{}) { logger.Trace(args...) }

// Traceln calls Traceln on the global Logger object.
func Traceln(args ...interface{}) { logger.Traceln(args...) }

// Debugf calls Debugf on the global Logger object.
func Debugf(format string, args ...interface{}) { logger.Debugf(format, args...) }

// Debug calls Debug on the global Logger object.
func Debug(args ...interface{}) { logger.Debug(args...) }

// Debugln calls Debugln on the global Logger object.
func Debugln(args ...interface{}) { logger.Debugln(args...) }

// Infof calls Infof on the global Logger object.
func Infof(format string, args ...interface{}) { logger.Infof(format, args...) }

// Info calls Info on the global Logger object.
func Info(args ...interface{}) { logger.Info(args...) }

// Infoln calls Infoln on the global Logger object.
func Infoln(args ...interface{}) { logger.Infoln(args...) }

// Warnf calls Warnf on the global Logger object.
func Warnf(format string, args ...interface{}) { logger.Warnf(format, args...) }

// Warn calls Warn on the global Logger object.
func Warn(args ...interface{}) { logger.Warn(args...) }

// Warnln calls Warnln on the global Logger object.
func Warnln(args ...interface{}) { logger.Warnln(args...) }

// Errorf calls Errorf on the global Logger object.
func Errorf(format string, args ...interface{}) { logger.Errorf(format, args...) }

// Error calls Error on the global Logger object.
func Error(args ...interface{}) { logger.Error(args...) }

// Errorln calls Errorln on the global Logger object.
func Errorln(args ...interface{}) { logger.Errorln(args...) }

// WithField calls WithField on the global Logger object.
func WithField(key string, value interface{}) Logger {
	return logger.WithField(key, value)
}

// WithFields calls WithFields on the global Logger object.
func WithFields(fs Fields) Logger {
	return logger.WithFields(fs)
}

// WithError calls WithError on the global Logger object.
func WithError(err error) Logger {
	return logger.WithError(err)
}

type (
	// An Owner owns a Logger that can be retrieved and a new Logger can be set.
	Owner interface {
		// Log returns the logger used by the Owner
		Log() Logger
		// SetLog sets the logger that the Owner uses in the future.
		SetLog(Logger)
	}

	// An Embedding can be embedded into any struct to endow it with a logger and
	// getter and setter for this logger. Embedding implements an Owner.
	Embedding struct {
		log Logger
	}
)

// AppendField sets the given field in the owner's logger. The resulting logger
// is also returned.
func AppendField(owner Owner, key string, value interface{}) Logger {
	l := owner.Log().WithField(key, value)
	owner.SetLog(l)
	return l
}

// AppendField sets the given fields in the owner's logger. The resulting logger
// is also returned.
func AppendFields(owner Owner, fs Fields) Logger {
	l := owner.Log().WithFields(fs)
	owner.SetLog(l)
	return l
}

// MakeEmbedding returns an Embedding around log.
func MakeEmbedding(log Logger) Embedding { return Embedding{log: log} }

// Log returns the embedded Logger.
func (em Embedding) Log() Logger { return em.log }

// SetLog sets the embedded Logger.
func (em Embedding) SetLog(l Logger) { em.log = l }
