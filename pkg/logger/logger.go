package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

func NewLogger(level string) *logrus.Logger {
	logger := logrus.New()

	// Set log format
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Set output
	logger.SetOutput(os.Stdout)

	// Set log level
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	return logger
}
