package logger

import (
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/stroppy-io/stroppy-cloud/internal/core/build"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

type LogMod string

func (l LogMod) String() string {
	return string(l)
}

const (
	DevelopmentMod LogMod = "development"
	ProductionMod  LogMod = "production"
)

const (
	LogModEnvKey        = "LOG_MOD"
	LevelEnvKey         = "LOG_LEVEL"
	LogMappingEnvKey    = "LOG_MAPPING"
	LogSkipCallerEnvKey = "LOG_SKIP_CALLER"
)

type Config struct {
	LogMod     LogMod            `mapstructure:"mod" default:"production" validate:"oneof=production development"`
	LogLevel   string            `mapstructure:"level" default:"info" validate:"oneof=debug info warn error"`
	LogMapping map[string]LogMod `mapstructure:"mapping"`
	SkipCaller bool              `mapstructure:"skip_caller"`
}

func parseMapping(mappingStr string) map[string]LogMod {
	if mappingStr == "" {
		return nil
	}
	mapping := make(map[string]LogMod)
	for _, pair := range strings.Split(mappingStr, ",") {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			continue
		}
		mapping[kv[0]] = LogMod(kv[1])
	}
	return mapping
}

func configFromEnv() *Config {
	return &Config{
		LogMod:     LogMod(os.Getenv(LogModEnvKey)),
		LogLevel:   os.Getenv(LevelEnvKey),
		LogMapping: parseMapping(os.Getenv(LogMappingEnvKey)),
		SkipCaller: os.Getenv(LogSkipCallerEnvKey) == "true",
	}
}

var (
	globalLogger  = newDefault() //nolint:gochecknoglobals // global logger needed for all app.
	globalMapping = make(map[string]zapcore.Level)
)

// newDefault creates new default logger.
func newDefault(opts ...zap.Option) *zap.Logger {
	cfg := newZapCfg(DevelopmentMod, zapcore.DebugLevel)
	logger, _ := cfg.Build(opts...)

	return logger
}

// newZapCfg creates new zap config.
func newZapCfg(mod LogMod, logLevel zapcore.Level) zap.Config {
	var cfg zap.Config

	switch mod {
	case ProductionMod:
		cfg = zap.NewProductionConfig()
		cfg.Level.SetLevel(logLevel)
	case DevelopmentMod:
		cfg = zap.NewDevelopmentConfig()
	default:
		cfg = zap.NewDevelopmentConfig()
	}

	return cfg
}

// NewFromConfig creates new logger from config.
func NewFromConfig(cfg *Config, opts ...zap.Option) *zap.Logger {
	level, parseErr := zapcore.ParseLevel(cfg.LogLevel)
	if parseErr != nil {
		panic(parseErr)
	}

	zapCfg := newZapCfg(cfg.LogMod, level)
	logger, err := zapCfg.Build(opts...)
	if err != nil {
		panic(err)
	}

	//logger = bridge.AttachToZapLogger(logger)

	globalLogger = logger.With(
		zap.String("service", build.ServiceName),
		zap.String("version", build.Version),
		zap.String("instance", build.GlobalInstanceId),
	)

	for name, lvl := range cfg.LogMapping {
		globalMapping[name], err = zapcore.ParseLevel(string(lvl))
		if err != nil {
			panic(err)
		}
	}

	if cfg.SkipCaller {
		globalLogger = globalLogger.WithOptions(zap.WithCaller(false))
	}

	return globalLogger
}

func NewFromEnv(opts ...zap.Option) *zap.Logger {
	return NewFromConfig(configFromEnv(), opts...)
}

func getNamedLoggerLevel(name string) zapcore.Level {
	if globalMapping == nil {
		return Global().Level()
	}
	if level, ok := globalMapping[name]; ok {
		return level
	}
	return Global().Level()
}

// Global returns the global logger.
func Global() *zap.Logger {
	return globalLogger
}

func Named(name string) *zap.Logger {
	return globalLogger.Named(name).WithOptions(zap.IncreaseLevel(getNamedLoggerLevel(name)))
}

func Slog() *slog.Logger {
	return NewSlogFromLogger(Global())
}

func NamedSlog(name string) *slog.Logger {
	return NewSlogFromLogger(Named(name))
}

func NewSlogFromLogger(lg *zap.Logger) *slog.Logger {
	return slog.New(zapslog.NewHandler(lg.Core()))
}

func StdLog() *log.Logger {
	stdOutLogger, err := zap.NewStdLogAt(Global(), Global().Level())
	if err != nil {
		panic(err)
	}
	return stdOutLogger
}

func Zerolog() *zerolog.Logger {
	var zeroLvl zerolog.Level
	switch Global().Level() {
	case zapcore.DebugLevel:
		zeroLvl = zerolog.DebugLevel
	case zapcore.InfoLevel:
		zeroLvl = zerolog.InfoLevel
	case zapcore.WarnLevel:
		zeroLvl = zerolog.WarnLevel
	case zapcore.ErrorLevel:
		zeroLvl = zerolog.ErrorLevel
	case zapcore.DPanicLevel:
		zeroLvl = zerolog.PanicLevel
	case zapcore.PanicLevel:
		zeroLvl = zerolog.PanicLevel
	case zapcore.FatalLevel:
		zeroLvl = zerolog.FatalLevel
	default:
		zeroLvl = zerolog.InfoLevel
	}
	stdOutLogger := StdLog()
	logger := zerolog.New(stdOutLogger.Writer()).Level(zeroLvl).With().Fields(
		map[string]any{
			"service":  build.ServiceName,
			"version":  build.Version,
			"instance": build.GlobalInstanceId,
		},
	).Logger()
	return &logger
}
