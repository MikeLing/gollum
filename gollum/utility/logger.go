package utility

import (
	"log/syslog"
	"sync"

	"github.com/sirupsen/logrus"
	logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
)

// GollumLogger is the logger for gollum
var GollumLogger *logrus.Logger

var logonce sync.Once

// GetLogger will init and return a Logger
func GetLogger() *logrus.Logger {
	logonce.Do(func() {
		c := GetConfig()
		GollumLogger = logrus.New()
		GollumLogger.SetFormatter(&logrus.JSONFormatter{})
		hook, err := logrus_syslog.NewSyslogHook("", c.Logpath, syslog.LOG_INFO, "gollum.exe")
		if err == nil {
			GollumLogger.Hooks.Add(hook)
		} else {
			GollumLogger.Info("Failed to log to syslog, using default stderr")
		}
	})

	return GollumLogger

}
