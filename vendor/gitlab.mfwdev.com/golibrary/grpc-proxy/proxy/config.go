package proxy

import (
	"github.com/jinzhu/configor"
	"github.com/pkg/errors"
)

type TargetOptions struct {
	Host        string `required:"true"`
	Port        int    `required:"true"`
	Name        string
	ScriptPath  string `required:"true"`
	ScriptName  string `required:"true"`
	ClientIP    string
	ReturnError bool
}

type Options struct {
	Debug        bool   `default:"false"`
	Host         string `required:"true"`
	KeyFile      string
	CrtFile      string
	InstanceName string `required:"true"`
	Target       TargetOptions
}

func LoadConfig(file string) (*Options, error) {
	options := &Options{}
	err := configor.Load(options, file)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load configuration")
	}

	return options, nil
}
