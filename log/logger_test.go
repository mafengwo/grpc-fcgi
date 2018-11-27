package log

import (
	"go.uber.org/zap"
	"testing"
)

var (
	opt1 = &Options{
		AccessLogPath: "stdout",
		ErrorLogPath: "stderr",
		ErrorLogLevel: "info",
	}
	opt2 = &Options{
		AccessLogPath: "stdout",
		ErrorLogPath: "stderr",
		ErrorLogLevel: "debug",
	}
)

func TestLogger_ErrorLog(t *testing.T) {
	l, err := NewLogger(opt1)
	if err != nil {
		t.Errorf("failed to initialize logger: %v", err)
		return
	}

	newlog := l.AcquireErrorLogger()
	newlog = newlog.With(zap.String("onekey", "onevalue"))
	newlog = newlog.With(zap.String("msg", "newmsg"))
	newlog.Error("debug info")
}

func TestLogger_ErrorLogTrace(t *testing.T) {
	l, err := NewLogger(opt2)
	if err != nil {
		t.Errorf("failed to initialize logger: %v", err)
		return
	}

	newlog := l.AcquireErrorLogger()
	newlog.Error("error info")
}

