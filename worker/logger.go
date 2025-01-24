package worker

import (
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	zap *zap.SugaredLogger
}

func NewLogger() asynq.Logger {
	logger := zap.Must(zap.NewProduction()).Sugar()
	return &Logger{
		zap: logger,
	}
}

func (l *Logger) Print(level zapcore.Level, args ...any) {
	l.zap.Log(level, args...)
}

func (l *Logger) Debug(args ...interface{}) {
	l.Print(zapcore.DebugLevel, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.Print(zapcore.InfoLevel, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.Print(zapcore.WarnLevel, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.Print(zapcore.ErrorLevel, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.Print(zapcore.FatalLevel, args...)
}
