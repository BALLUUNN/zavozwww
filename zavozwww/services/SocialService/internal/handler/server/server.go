package config

import (
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config представляет собой конфигурацию сервера.
type Config struct {
	Server   Server   `yaml:"server"`
	Services Services `yaml:"services"`
}

// Server представляет собой конфигурацию сервера.
type Server struct {
	Host string `yaml:"host" env-default:"0.0.0.0"`
	Port string `yaml:"port" env-default:"8080"`
}

// Services представляет собой конфигурацию внешних сервисов.
type Services struct {
	UserServiceURL  string `yaml:"user_service_url" env-default:"http://localhost:8081"`
	MovieServiceURL string `yaml:"movie_service_url" env-default:"http://localhost:8082"`
}

// MustLoad загружает конфигурацию из файла и завершает работу программы при ошибке.
func MustLoad() *Config {
	configPath := os.Getenv("APP_CONFIG_PATH")
	if configPath == "" {
		configPath = "./internal/config/config.yaml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config: %s", err)
	}

	return &cfg
}
