package sentry

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/getsentry/raven-go"
	"github.com/nuveo/log"
)

var now = time.Now

func init() {
	log.AddAdapter("sentry", log.AdapterPod{
		Adapter: sentryLog,
		Config: map[string]interface{}{
			"dsn":            "",
			"tags":           map[string]string{},
			"enableMsgTypes": []log.MsgType{log.ErrorLog},
		},
	})
}

func containsType(m log.MsgType, ts []log.MsgType) bool {
	for _, t := range ts {
		if m == t {
			return true
		}
	}
	return false
}

func sentryLog(m log.MsgType, o log.OutType, config map[string]interface{}, msg ...interface{}) {
	ts := config["enableMsgTypes"].([]log.MsgType)

	if !containsType(m, ts) {
		return
	}

	if m == log.DebugLog && !log.DebugMode {
		return
	}

	var debugInfo, lineBreak, output string

	if log.DebugMode {
		_, fn, line, _ := runtime.Caller(5)
		fn = filepath.Base(fn)
		debugInfo = fmt.Sprintf("%s:%d ", fn, line)
	}

	if o == log.FormattedOut {
		output = fmt.Sprintf(msg[0].(string), msg[1:]...)
	} else {
		output = fmt.Sprint(msg...)
		lineBreak = "\n"
	}

	output = fmt.Sprintf("%s [%s] %s%s",
		now().UTC().Format(log.TimeFormat),
		log.Prefixes[m],
		debugInfo,
		output)

	if len(output) > log.MaxLineSize {
		output = output[:log.MaxLineSize] + "..."
	}
	output = output + lineBreak

	raven.SetDSN(config["dsn"].(string))
	packet := &raven.Packet{Message: "err", Interfaces: []raven.Interface{raven.NewException(errors.New(output), raven.NewStacktrace(0, 5, nil)), nil}}
	_, ch := raven.Capture(packet, config["tags"].(map[string]string))
	if err := <-ch; err != nil {
		fmt.Println("error try to send", err)
	}
}
