package log

import "fmt"

type Level int
const DebugLevel Level = 1

func Debugf(format string, args ...interface{})  { _ = fmt.Sprintf(format, args...) }
func Infof(format string, args ...interface{})   {}
func Warnf(format string, args ...interface{})   {}
func Errorf(format string, args ...interface{})  {}
func Fatal(args ...interface{})                 {}
func SetVerbose()                                {}
