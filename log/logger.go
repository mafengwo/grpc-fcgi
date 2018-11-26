package log

import (
	"errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Options struct {
	AccessLogPath string
	ErrorLogLevel string // supports: debug; info; warn; error
	ErrorLogPath  string
}

type Logger struct {
	accessLogger *zap.Logger
	errorLogger  *zap.Logger
}

var (
	levelMap = map[string]zapcore.Level{
		"debug": zapcore.DebugLevel,
		"info":  zapcore.InfoLevel,
		"warn":  zapcore.WarnLevel,
		"error": zapcore.ErrorLevel,
	}
)

func NewLogger(opt *Options) (*Logger, error) {
	ec := zap.NewProductionEncoderConfig()
	al, err := zap.Config{
		Development:       false,
		DisableCaller:     true,
		DisableStacktrace: true,
		EncoderConfig:     ec,
		Encoding:          "json",
		ErrorOutputPaths:  []string{opt.AccessLogPath},
		Level:             zap.NewAtomicLevelAt(zapcore.InfoLevel),
		OutputPaths:       []string{opt.AccessLogPath},
	}.Build()
	if err != nil {
		return nil, err
	}

	lv, exist := levelMap[opt.ErrorLogLevel]
	if !exist {
		return nil, errors.New("Unsupported log level:" + opt.ErrorLogLevel)
	}
	el, err := zap.Config{
		Development:       lv <= zapcore.DebugLevel,
		DisableCaller:     lv > zapcore.DebugLevel,
		DisableStacktrace: lv > zapcore.DebugLevel,
		EncoderConfig:     zap.NewProductionEncoderConfig(),
		Encoding:          "json",
		ErrorOutputPaths:  []string{opt.ErrorLogPath},
		Level:             zap.NewAtomicLevelAt(lv),
		OutputPaths:       []string{opt.ErrorLogPath},
	}.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{
		accessLogger: al,
		errorLogger:  el,
	}, nil
}

func (l *Logger) AcquireErrorLogger() *zap.Logger {
	clone := *l.errorLogger
	return &clone
}

func (l *Logger) AcquireAccessLogger() *zap.Logger {
	clone := *l.accessLogger
	return &clone
}

