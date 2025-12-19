package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"social_service/internal/clients"
	"social_service/internal/handler/rout"
	config "social_service/internal/handler/server"
	repo "social_service/internal/repositories"
	"social_service/internal/services"
	"social_service/pkg/logger"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

// Helper function для чтения env (чтобы не загромождать main)
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("FATAL PANIC DETECTED: %v\n", r)
			os.Exit(1)
		}
	}()

	fmt.Println("DEBUG: Starting main function")

	if err := godotenv.Load(".env"); err != nil {
		fmt.Println("Warning: .env file not found")
	}

	fmt.Printf("DEBUG: APP_CONFIG_PATH=%s\n", os.Getenv("APP_CONFIG_PATH"))

	ctx := context.Background()
	fmt.Println("DEBUG: Context created")

	fmt.Println("DEBUG: Before logger initialization")
	lgr, err := logger.NewLogger()
	if err != nil {
		fmt.Println("Error initializing logger:", "error", err)
		return
	}
	lgr.Info("Logger initialized successfully.")

	cfg := config.MustLoad()
	lgr.Info(fmt.Sprintf("Config loaded. UserSvc: %s, MovieSvc: %s", cfg.Services.UserServiceURL, cfg.Services.MovieServiceURL))

	// Считываем параметры подключения к БД из переменных окружения
	pgHost := getEnv("POSTGRES_HOST", "localhost")
	pgPort := getEnv("POSTGRES_PORT", "5432")
	pgUser := getEnv("POSTGRES_USER", "social_user")
	pgPassword := getEnv("POSTGRES_PASSWORD", "social_password")
	pgDB := getEnv("POSTGRES_DB", "social_db")
	pgSSL := getEnv("POSTGRES_SSLMODE", "disable")

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		pgUser, pgPassword, pgHost, pgPort, pgDB, pgSSL)

	const migrationsPath = "file://internal/migrations"

	var m *migrate.Migrate
	var migrateErr error

	for i := 0; i < 15; i++ {
		m, migrateErr = migrate.New(migrationsPath, dbURL)
		if migrateErr == nil {
			break
		}
		lgr.Info(fmt.Sprintf("Failed to create migrator, retrying in 2s... (attempt %d/15). Error: %v", i+1, migrateErr))
		time.Sleep(2 * time.Second)
	}

	if migrateErr != nil {
		lgr.Error("Error creating migrator (check path)", "err", migrateErr)
	} else {
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			lgr.Error("Failed to apply migrations", "err", err)
			return
		}
		lgr.Info("Migrations applied successfully.")
	}

	pgxPool, err := repo.NewPostgresPool(ctx, dbURL)
	if err != nil {
		lgr.Error("Error connecting to Postgres:", "error", err)
		return
	}
	defer pgxPool.Close()
	lgr.Info("Successfully connected to Postgres")

	socialRepo := repo.NewPostgresSocialRepository(pgxPool)
	userClient := clients.NewUserClient(cfg.Services.UserServiceURL)

	jwtSecret := getEnv("JWT_SECRET", "secret")
	socialService := services.NewSocialService(socialRepo, userClient, jwtSecret)
	lgr.Info("Services initialized successfully")

	handler := rout.NewHandler(socialService, lgr)
	router := handler.InitRoutes()

	serverHost := cfg.Server.Host
	serverPort := cfg.Server.Port

	lgr.Info(fmt.Sprintf("Starting server on %s:%s", serverHost, serverPort))

	serverAddr := serverHost + ":" + serverPort
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		lgr.Error("Failed to start server", "error", err)
	}
}
