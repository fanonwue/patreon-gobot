package logging

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	LevelPanic = iota
	LevelFatal
	LevelError
	LevelWarn
	LevelInfo
	LevelDebug
)

var levelNames = map[int]string{
	LevelPanic: "PANIC - ",
	LevelFatal: "FATAL - ",
	LevelError: "ERROR - ",
	LevelWarn:  "WARN  - ",
	LevelInfo:  "INFO  - ",
	LevelDebug: "DEBUG - ",
}

var logLevel = LevelInfo

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmicroseconds)
}

func callerInfo(calldepth int) string {
	_, file, no, _ := runtime.Caller(calldepth)

	fileParts := strings.Split(file, "/")
	caller := fileParts[len(fileParts)-2:]
	return strings.Join(caller, "/") + ":" + strconv.Itoa(no)
}

func SetLogLevel(level int) {
	logLevel = level
}

func Logf(level int, msg string, args ...any) {
	if logLevel < level {
		return
	}
	newArgs := []any{levelNames[level]}
	err := log.Output(3, fmt.Sprintf("\t%s"+msg, append(newArgs, args...)...))
	if err != nil {
		fmt.Printf("ERROR: Could not write log message: %v", err)
	}
}

func Info(msg string) {
	Logf(LevelInfo, msg)
}

func Infof(msg string, args ...any) {
	Logf(LevelInfo, msg, args...)
}

func Warn(msg string) {
	Logf(LevelWarn, msg)
}

func Warnf(msg string, args ...any) {
	Logf(LevelWarn, msg, args...)
}

func Error(msg string) {
	Logf(LevelError, msg)
}

func Errorf(msg string, args ...any) {
	Logf(LevelError, msg, args...)
}

func Debug(msg string) {
	Logf(LevelDebug, msg)
}

func Debugf(msg string, args ...any) {
	Logf(LevelDebug, msg, args...)
}

func Panic(msg string) {
	Logf(LevelPanic, msg)
}

func Panicf(msg string, args ...any) {
	Logf(LevelPanic, msg, args...)
}

func Fatal(msg string) {
	Logf(LevelFatal, msg)
}

func Fatalf(msg string, args ...any) {
	Logf(LevelFatal, msg, args...)
}
