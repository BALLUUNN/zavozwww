package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
)

// Logger представляет собой настраиваемый логгер с уровнем логирования и форматом вывода.
type Logger struct {
	slog.Logger
	level  string
	format string
	output *os.File
}

// Config представляет собой структуру конфигурации для логгера.
type Config struct {
	Logging struct {
		Level  string `yaml:"level" env:"LOG_LEVEL" env-default:"INFO"`
		Format string `yaml:"format" env:"LOG_FORMAT" env-default:"TEXT"`
		Output string `yaml:"output" env:"LOG_OUTPUT" env-default:"stdout"`
	} `yaml:"logger"`
}

// NewLogger инициализирует и возвращает новый экземпляр Logger на основе конфигурационного файла.
func NewLogger() (*Logger, error) {
	const op = "pkg.logger.NewLogger"

	cfgPath := os.Getenv("APP_CONFIG_PATH")
	if cfgPath == "" {
		return nil, fmt.Errorf("%s: environment variable APP_CONFIG_PATH is not set", op)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(cfgPath, &cfg); err != nil {
		return nil, fmt.Errorf("%s: failed to read config from %q: %w", op, cfgPath, err)
	}

	return NewLoggerWithConfig(cfg)
}

// NewLoggerWithConfig создает логгер на основе переданной конфигурации.
func NewLoggerWithConfig(cfg Config) (*Logger, error) {
	const op = "pkg.logger.NewLoggerWithConfig"

	level := cfg.Logging.Level
	format := cfg.Logging.Format

	slogLevel := parseLevel(level)

	var handler slog.Handler
	handlerOpts := &slog.HandlerOptions{
		Level:     slogLevel,
		AddSource: slogLevel == slog.LevelDebug,
	}

	switch format {
	case "JSON":
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	case "TEXT":
		handler = NewPrettyHandler(os.Stdout, handlerOpts)
	default:
		return nil, fmt.Errorf("%s: unsupported log format %q, expected TEXT or JSON", op, format)
	}

	slogLogger := slog.New(handler)

	return &Logger{
		Logger: *slogLogger,
		level:  level,
		format: format,
	}, nil
}

// PrettyHandler - кастомный хендлер для красивого вывода логов
type PrettyHandler struct {
	opts  slog.HandlerOptions
	w     io.Writer
	mu    sync.Mutex
	attrs []slog.Attr
}

func NewPrettyHandler(w io.Writer, opts *slog.HandlerOptions) *PrettyHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &PrettyHandler{
		w:    w,
		opts: *opts,
	}
}

func (h *PrettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	color := "\033[0m"

	switch r.Level {
	case slog.LevelDebug:
		level = "DBG"
		color = "\033[35m" // Magenta
	case slog.LevelInfo:
		level = "INF"
		color = "\033[32m" // Green
	case slog.LevelWarn:
		level = "WRN"
		color = "\033[33m" // Yellow
	case slog.LevelError:
		level = "ERR"
		color = "\033[31m" // Red
	}

	timeStr := r.Time.Format("15:04:05")

	// Собираем буфер
	buf := make([]byte, 0, 1024)

	// Время (серый)
	buf = append(buf, "\033[90m"...)
	buf = append(buf, timeStr...)
	buf = append(buf, "\033[0m "...)

	// Уровень
	buf = append(buf, color...)
	buf = append(buf, level...)
	buf = append(buf, "\033[0m "...)

	// Сообщение
	buf = append(buf, r.Message...)

	// Атрибуты
	appendAttr := func(a slog.Attr) {
		if a.Key == "" {
			return
		}
		buf = append(buf, ' ')
		buf = append(buf, "\033[36m"...) // Cyan для ключей
		buf = append(buf, a.Key...)
		buf = append(buf, "="...)
		buf = append(buf, "\033[0m"...)

		val := a.Value.Resolve()
		buf = append(buf, fmt.Sprint(val.Any())...)
	}

	for _, a := range h.attrs {
		appendAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(a)
		return true
	})

	buf = append(buf, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	newAttrs = append(newAttrs, h.attrs...)
	newAttrs = append(newAttrs, attrs...)

	return &PrettyHandler{
		opts:  h.opts,
		w:     h.w,
		attrs: newAttrs,
	}
}

func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	// Группы пока игнорируем для простоты
	return h
}

// parseLevel преобразует строковое представление уровня логирования в slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Level возвращает строковое представление уровня логирования.
func (l *Logger) Level() string {
	return l.level
}

// Format возвращает формат логирования (TEXT или JSON).
func (l *Logger) Format() string {
	return l.format
}

// WithAttrs создает новый логгер с дополнительными атрибутами.
func (l *Logger) WithAttrs(attrs ...slog.Attr) *Logger {
	return &Logger{
		Logger: *slog.New(l.Handler().WithAttrs(attrs)),
		level:  l.level,
		format: l.format,
	}
}

// WithGroup создает новый логгер с группой атрибутов.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		Logger: *slog.New(l.Handler().WithGroup(name)),
		level:  l.level,
		format: l.format,
	}
}

// Close закрывает ресурсы логгера, если необходимо.
func (l *Logger) Close() error {
	if l.output != nil && l.output != os.Stdout && l.output != os.Stderr {
		return l.output.Close()
	}
	return nil
}
