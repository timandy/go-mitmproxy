package log

import "github.com/sirupsen/logrus"

type LogrusLogger struct {
}

func (l LogrusLogger) Debug(args ...any) {
	logrus.Debug(args...)
}

func (l LogrusLogger) Debugf(format string, args ...any) {
	logrus.Debugf(format, args...)
}

func (l LogrusLogger) Info(args ...any) {
	logrus.Info(args...)
}

func (l LogrusLogger) Infof(format string, args ...any) {
	logrus.Infof(format, args...)
}

func (l LogrusLogger) Warn(args ...any) {
	logrus.Warn(args...)
}

func (l LogrusLogger) Warnf(format string, args ...any) {
	logrus.Warnf(format, args...)
}

func (l LogrusLogger) Error(args ...any) {
	logrus.Error(args...)
}

func (l LogrusLogger) Errorf(format string, args ...any) {
	logrus.Errorf(format, args...)
}

func (l LogrusLogger) Fatal(args ...any) {
	logrus.Fatal(args...)
}

func (l LogrusLogger) Fatalf(format string, args ...any) {
	logrus.Fatalf(format, args...)
}
