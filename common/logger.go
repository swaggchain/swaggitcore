package common

import (
	log "github.com/sirupsen/logrus"
	"os"
)

func init() {
	log.SetFormatter(&log.TextFormatter{})
	// default output to console
	log.SetOutput(os.Stdout)
	//default level is warn
	log.SetLevel(log.WarnLevel)
}

type Logger struct {
	Get *log.Logger
}

func (l *Logger) EnableTraceByDefault() {
	l.Get.SetLevel(log.TraceLevel)
}

func (l *Logger) LogSoftError(err error) {
	if err != nil {
		log.Error(err)
	}
}

func (l *Logger) LogAndQuit(err error) {
	if err != nil {
		log.Fatalf("a fatal error occured the program will abort. Error: %v", err)
	}
}

func (l *Logger) LogAndPanic(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func (l *Logger) EnableDebugMode() {
	l.Get.SetLevel(log.DebugLevel)
}

func (l *Logger) ShowTrace(data interface{}) {
	l.Get.Trace(data)
}


func CreateNewLogger(prodMode bool) *Logger {

	l := &Logger{}
	l.Get = log.New()
	if prodMode == false {
		l.EnableTraceByDefault()
		l.EnableDebugMode()
	}

	return l

}