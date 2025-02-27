package util

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

// Logger is global logger object
var Logger = logrus.New()

// InitLoggerWith init the global logger
func InitLoggerWith(logLevel logrus.Level, logPath string, maxLogFileCount uint) {
	if len(logPath) > 0 {
		ext := filepath.Ext(logPath)
		base := filepath.Base(logPath)
		fileNameWithoutExt := base[:len(base)-len(ext)]
		dir := filepath.Dir(logPath)
		logRotateFilePattern := fmt.Sprintf("%s%c%s.%%Y%%m%%d%s", dir, byte(filepath.Separator), fileNameWithoutExt, ext)
		writer, err := rotatelogs.New(
			logRotateFilePattern,
			rotatelogs.WithLinkName(logPath),
			rotatelogs.WithRotationTime(time.Hour*24),
			rotatelogs.WithRotationCount(maxLogFileCount),
		)
		if err != nil {
			log.Fatal(err)
		}
		Logger.SetOutput(writer)
	} else {
		Logger.SetOutput(os.Stdout)
	}
	Logger.Formatter = &logrus.JSONFormatter{}
	Logger.SetReportCaller(logLevel >= logrus.DebugLevel)
	Logger.SetLevel(logLevel)
}
