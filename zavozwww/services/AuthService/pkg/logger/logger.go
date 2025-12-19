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

// Logger представляет собой настраиваемый логгер с уровнем логирования и путем к файлу.
type Logger struct {
	level string `yaml:"level"`
	slog.Logger
}

// configFile представляет собой структуру конфигурационного файла для логгера.
type configFile struct {
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
}

// NewLogger инициализирует и возвращает новый экземпляр Logger на основе конфигурационного файла.
func NewLogger() (*Logger, error) {
	const op = "pkg.logger.NewLogger"
	var cfgFile configFile

	cfgPath := os.Getenv("APP_CONFIG_PATH")
	if cfgPath == "" {
		return nil, fmt.Errorf("%s: environment variable APP_CONFIG_PATH is not set", op)
	}

	if err := cleanenv.ReadConfig(cfgPath, &cfgFile); err != nil {
		return nil, fmt.Errorf("%s: read config %q: %w", op, cfgPath, err)
	}

	cfg := Logger{
		level: cfgFile.Logging.Level,
	}

	out := os.Stdout

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     getLevel(cfg.level),
		AddSource: false,
	}

	if cfgFile.Logging.Format == "JSON" {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		// Используем PrettyHandler для красивого вывода в консоль/докер
		handler = NewPrettyHandler(out, opts)
	}

	slogLogger := slog.New(handler)

	l := &Logger{
		level:  cfg.level,
		Logger: *slogLogger,
	}

	return l, nil
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
		color = "\033[35m"
	case slog.LevelInfo:
		level = "INF"
		color = "\033[32m"
	case slog.LevelWarn:
		level = "WRN"
		color = "\033[33m"
	case slog.LevelError:
		level = "ERR"
		color = "\033[31m"
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
	return h
}

func getLevel(level string) slog.Level {
	switch level {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Level возвращает уровень логирования.
func (l *Logger) Level() string {
	return l.level
}
