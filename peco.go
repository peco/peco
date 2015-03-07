package peco

import (
	"log"
	"os"
	"strconv"
)

type traceLogger interface {
	Printf(string, ...interface{})
}
type nullTraceLogger struct{}

func (ntl nullTraceLogger) Printf(_ string, _ ...interface{}) {}

var tracer traceLogger = nullTraceLogger{}

func init() {
	if v, err := strconv.ParseBool(os.Getenv("PECO_TRACE")); err == nil && v {
		tracer = log.New(os.Stderr, "peco: ", log.LstdFlags)
		tracer.Printf("==== INITIALIZED tracer ====")
	}
}
