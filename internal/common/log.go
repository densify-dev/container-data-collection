package common

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type LogLevel int

const (
	Debug LogLevel = iota
	Info
	Warn
	Error
	Unknown
)

const (
	ClusterFormat    = "cluster=%s"
	DefaultLogFormat = ClusterFormat + Space + "entity=%s"
)

func (ll LogLevel) String() string {
	var l string
	switch ll {
	case Debug:
		l = debugLevel
	case Info:
		l = infoLevel
	case Warn:
		l = warnLevel
	case Error:
		l = errorLevel
	default:
		l = unknownLevel
	}
	return "[" + l + "]"
}

type clusterLoggers map[LogLevel]*log.Logger
type loggers map[string]clusterLoggers

var noLogs = loggers{Empty: nil}
var logs loggers

func InitLogs() {
	logs = make(loggers, NumClusters())
	for _, cluster := range ClusterNames {
		logFile, err := os.OpenFile(filepath.Join(rootFolder, cluster, logFileName), logFileFlag, logFilePerm)
		if err != nil {
			log.Fatal(err)
		}
		m := make(clusterLoggers, Unknown)
		for level := Debug; level < Unknown; level++ {
			m[level] = log.New(logFile, fmt.Sprintf(prefixFormat, level), logFlag)
		}
		logs[cluster] = m
	}
}

func LogError(err error, format string, v ...any) {
	logError(2, err, format, v...)
}

func logError(callDepth int, err error, format string, v ...any) {
	LogErrorWithLevel(callDepth+1, Error, err, format, v...)
}

func LogErrorWithLevel(callDepth int, level LogLevel, err error, format string, v ...any) {
	nv := v
	f := format
	if err != nil {
		nv = append(nv, err)
		f += errorFormat
	}
	if strings.HasPrefix(format, ClusterFormat) && len(v) > 0 {
		if cluster, ok := v[0].(string); ok {
			LogCluster(callDepth+1, level, f, cluster, true, nv...)
		}
	} else {
		LogAll(callDepth+1, level, f, nv...)
	}
}

func FatalError(err error, format string, v ...any) {
	logError(2, err, format, v...)
	log.Fatalf(fatalMsg)
}

func DebugLogObjectMemStats(obj string) {
	DebugLogMemStats(2, fmt.Sprintf(objectMetricsFormat, obj))
}

func DebugLogMemStats(callDepth int, msg string) {
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)
	LogAll(callDepth+1, Debug, memStatsFormat1, msg)
	LogAll(callDepth+1, Debug, memStatsFormat2, MiB(memStats.Alloc), MiB(memStats.TotalAlloc), MiB(memStats.Sys), memStats.NumGC)
}

func LogAll(callDepth int, level LogLevel, format string, v ...any) {
	if shouldLog(level) {
		var ls loggers
		if len(logs) == 0 {
			// loggers not initialized yet, use noLogs with a nil map that will just make sure the log will go to stdout
			ls = noLogs
		} else {
			ls = logs
		}
		toStdOut := true
		for cluster := range ls {
			LogCluster(callDepth+1, level, format, cluster, toStdOut, v...)
			if toStdOut {
				toStdOut = false
			}
		}
	}
}

func LogCluster(callDepth int, level LogLevel, format string, cluster string, toStdOut bool, v ...any) {
	if shouldLog(level) {
		msg := fmt.Sprintf(format+lf, v...)
		if l, found := logs[cluster][level]; found {
			_ = l.Output(callDepth+1, msg)
		}
		if toStdOut {
			_, _ = fmt.Printf(levelFormat, level, msg)
		}
	}
}

func shouldLog(level LogLevel) bool {
	return level < Unknown && (level > Debug || Params.Debug)
}

const (
	logFileFlag         = os.O_WRONLY | os.O_CREATE
	logFilePerm         = 0644
	debugLevel          = "DEBUG"
	warnLevel           = "WARN"
	errorLevel          = "ERROR"
	unknownLevel        = "UNKNOWN"
	logFileName         = "log.txt"
	prefixFormat        = "%v "
	logFlag             = log.Ldate | log.Ltime | log.Lshortfile
	levelFormat         = prefixFormat + "%s"
	errorFormat         = " message=%v"
	memStatsFormat1     = "mem stats before %s:"
	memStatsFormat2     = "Alloc = %vMiB\tTotalAlloc = %vMiB\tSys = %vMiB\tNumGC = %v"
	objectMetricsFormat = "collecting %s metrics"
	fatalMsg            = "cannot proceed, exiting..."
)

var (
	infoLevel = strings.ToUpper(InfoSt)
)

// ClusterLeveledLogger implements rhttp.LeveledLogger to avoid clutter
type ClusterLeveledLogger struct {
	cluster string
}

func (cll *ClusterLeveledLogger) Error(msg string, keysAndValues ...interface{}) {
	cll.log(Error, msg, keysAndValues...)
}

func (cll *ClusterLeveledLogger) Warn(msg string, keysAndValues ...interface{}) {
	cll.log(Warn, msg, keysAndValues...)
}

func (cll *ClusterLeveledLogger) Info(msg string, keysAndValues ...interface{}) {
	cll.log(Info, msg, keysAndValues...)
}

func (cll *ClusterLeveledLogger) Debug(msg string, keysAndValues ...interface{}) {
	cll.log(Debug, msg, keysAndValues...)
}

func (cll *ClusterLeveledLogger) log(level LogLevel, msg string, keysAndValues ...interface{}) {
	message := msg
	n := len(keysAndValues) - 1
	for i := 0; i < n; i += 2 {
		message += fmt.Sprintf(", %v : %v", keysAndValues[i], keysAndValues[i+1])
	}
	if cll.cluster == Empty {
		LogAll(3, level, message)
	} else {
		LogCluster(3, level, message, cll.cluster, true)
	}
}
