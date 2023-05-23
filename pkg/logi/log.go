package logi

import (
	"os"
	"strings"

	"fmt"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	czap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	Viper      = viper.New()
	Log        *zap.Logger
	ZapOptions czap.Options
)

const (
	// 日志大小限制，单位MB
	MaxSize = 100
	// 历史日志文件保留天数
	MaxAge = 30
	// 最大保留历史日志数量
	MaxBackups = 10
)

func init() {
	Viper.AllowEmptyEnv(true)
	Viper.AutomaticEnv()
	Viper.SetTypeByDefaultValue(true)
	Viper.SetDefault("LogInfoLevel", true)
	Viper.SetDefault("MaxSize", MaxSize)
	Viper.SetDefault("MaxAge", MaxAge)
	Viper.SetDefault("MaxBackups", MaxBackups)

	_ = Viper.BindEnv("LogInfoLevel", "LOG_INFO_LEVEL")
	_ = Viper.BindEnv("MaxSize", "MAX_SIZE")
	_ = Viper.BindEnv("MaxAge", "MAX_AGE")
	_ = Viper.BindEnv("MaxBackups", "MAX_BACKUPS")

	GetLogger(Viper)
	// SetLogger(Build(GetConfig()))
	// SetOptions(GetOptions(Viper))
}

func SetLogger(l *zap.Logger) {
	Log = l
}

func SetOptions(o czap.Options) {
	ZapOptions = o
}

func GetLogger(viper *viper.Viper) {
	// 日志轮转
	writer := &lumberjack.Logger{
		// 日志名称
		Filename: "multicloud-mongo-operator.log",
		// 日志大小限制，单位MB
		MaxSize: viper.GetInt("MaxSize"),
		// 历史日志文件保留天数
		MaxAge: viper.GetInt("MaxAge"),
		// 最大保留历史日志数量
		MaxBackups: viper.GetInt("MaxBackups"),
		// 本地时区
		LocalTime: true,
		// 历史日志文件压缩标识 默认关闭n
		Compress: false,
	}
	sink := zapcore.AddSync(writer)
	// 添加输出到控制台的writer
	consoleDebugging := zapcore.Lock(os.Stdout)
	var enc zapcore.Encoder
	var level zapcore.Level
	var opts []zap.Option
	encCfg := zap.NewDevelopmentEncoderConfig()
	// NewJSONEncoder和NewConsoleEncoder 最后都会创建jsonEncoder
	// 只是bool类型参数spaced不一样 include spaces after colons and commas
	// 主要还是encCfg控制，考虑到用json filebeat认为是一行，这边使用 NewConsoleEncoder
	enc = zapcore.NewConsoleEncoder(encCfg)
	var zapConfig zap.Config
	if !viper.GetBool("LogInfoLevel") {
		level = zap.DebugLevel
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		level = zap.InfoLevel
		zapConfig = zap.NewProductionConfig()
	}

	opts = append(opts, zap.Development(), zap.AddStacktrace(zap.ErrorLevel))
	opts = append(opts, zap.AddCallerSkip(1), zap.ErrorOutput(sink))
	// 定义多个core封装writer
	var allCore []zapcore.Core
	allCore = append(allCore, zapcore.NewCore(
		enc,
		consoleDebugging,
		level,
	))
	allCore = append(allCore, zapcore.NewCore(
		enc,
		sink,
		level,
	))
	core := zapcore.NewTee(allCore...)

	logger := zap.New(core, zap.AddCaller())
	logger = logger.WithOptions(opts...)
	defer logger.Sync()
	SetLogger(logger)
}

func GetOptions(viper *viper.Viper) czap.Options {
	development := true
	if !viper.GetBool("LogInfoLevel") {
		development = false
	}
	destWriter, err := rotatelogs.New(
		"multicloud-mongo-operator.%Y%m%d.log",
		//rotatelogs.WithLinkName(logName),
		// 设置为一天分割一次
		rotatelogs.WithRotationTime(time.Hour*24),
		// 最大保存时间
		//rotatelogs.WithMaxAge(time.Hour*7*24),
		//最大保存数量
		rotatelogs.WithRotationCount(7),
	)
	if err != nil {
		// 打印到控制台,无法存log
		fmt.Printf("config local file system for logger err: %v", err)
	}
	opts := czap.Options{
		Development: development,
		DestWriter:  destWriter,
	}
	return opts
}

func GetConfig() zap.Config {
	var zapConfig zap.Config
	level := levelFromEnv()
	switch level {
	case zap.DebugLevel:
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	default:
		zapConfig = zap.NewProductionConfig()
	}

	zapConfig.Level = zap.NewAtomicLevelAt(level)

	return zapConfig
}

func Build(zapConfig zap.Config) *zap.Logger {
	log, err := zapConfig.Build()
	if err != nil {
		panic(err)
	}
	return log
}

func IsDebug() bool {
	return Log.Core().Enabled(zap.DebugLevel)
}

func levelFromEnv() zapcore.Level {
	levelStr, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		// 如果不存在 LOG_LEVEL，尝试获取 DEBUG 环境变量（应该从config获取，但目前存在循环依赖，所以直接读取环境变量）
		debug, ok := os.LookupEnv("DEBUG")
		if ok && strings.ToLower(debug) != "true" {
			return zap.InfoLevel
		}
		return zap.DebugLevel
	}

	var level zapcore.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		panic(err)
	}
	return level
}
