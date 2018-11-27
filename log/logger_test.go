package log

import (
	"github.com/jinzhu/configor"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
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

func TestYaml(t *testing.T) {
	fc := `
boolval: false
`
	file := os.TempDir() + "/test.yaml"
	defer os.Remove(file)

	ioutil.WriteFile(file, []byte(fc), 0777)

	type Conf struct {
		BoolVal bool `required:"true" yaml:"boolval"`
	}

	var conf Conf
	//err := yaml.Unmarshal([]byte(fc), &conf)
	err := configor.Load(&conf, file)
	if err != nil {
		t.Errorf(err.Error())
	} else {
		t.Logf("conf: %v", conf)
	}

}
