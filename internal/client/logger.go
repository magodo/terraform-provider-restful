package client

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type tflogger struct {
	ctx context.Context
}

var _ resty.Logger = tflogger{}

func (t tflogger) Debugf(format string, v ...interface{}) {
	tflog.Debug(t.ctx, fmt.Sprintf(format, v...))
}

func (t tflogger) Warnf(format string, v ...interface{}) {
	tflog.Warn(t.ctx, fmt.Sprintf(format, v...))
}

func (t tflogger) Errorf(format string, v ...interface{}) {
	tflog.Error(t.ctx, fmt.Sprintf(format, v...))
}
