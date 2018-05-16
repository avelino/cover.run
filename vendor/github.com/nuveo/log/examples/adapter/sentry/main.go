package main

import (
	"fmt"
	"os"

	"github.com/nuveo/log"
	_ "github.com/nuveo/log/adapters/sentry"
)

func main() {

	config := make(map[string]interface{})
	config["dsn"] = os.Getenv("SENTRY_DSN")
	config["tags"] = map[string]string{"core": "auth"}

	config["enableMsgTypes"] = []log.MsgType{log.ErrorLog}
	log.SetAdapterConfig("sentry", config)

	log.Debugln("Debug message")

	log.DebugMode = false
	log.Debugln("Debug message that will be hidden")

	log.Println("Info message")
	log.Warningln("Warning message")
	log.Errorln("Error message")
	log.Fatal("Fatal error message")
	fmt.Println("I will never be printed because of Fatal()")
}
