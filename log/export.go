package log

var DefaultLogger Logger = LogrusLogger{}

func Debug(args ...any) {
	DefaultLogger.Debug(args...)
}

func Debugf(format string, args ...any) {
	DefaultLogger.Debugf(format, args...)
}

func Info(args ...any) {
	DefaultLogger.Info(args...)
}

func Infof(format string, args ...any) {
	DefaultLogger.Infof(format, args...)
}

func Warn(args ...any) {
	DefaultLogger.Warn(args...)
}

func Warnf(format string, args ...any) {
	DefaultLogger.Warnf(format, args...)
}

func Error(args ...any) {
	DefaultLogger.Error(args...)
}

func Errorf(format string, args ...any) {
	DefaultLogger.Errorf(format, args...)
}

func Fatal(args ...any) {
	DefaultLogger.Fatal(args...)
}

func Fatalf(format string, args ...any) {
	DefaultLogger.Fatalf(format, args...)
}
