package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/caarlos0/env/v11"
	models "github.com/sushihentaime/user-management-service/internal/db"

	"github.com/sushihentaime/user-management-service/internal/mail"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type application struct {
	config config
	logger *slog.Logger
	models *models.Models
	mailer *mail.Mailer
	wg     sync.WaitGroup
}

type config struct {
	Port string `env:"PORT,required"`
	Env  string `env:"ENV,required"`
	DB   struct {
		DB_HOST      string        `env:"DB_HOST,required"`
		DB_PORT      int           `env:"DB_PORT,required"`
		DB_USER      string        `env:"POSTGRES_USER,required"`
		DB_PASSWORD  string        `env:"POSTGRES_PASSWORD,required"`
		DB_NAME      string        `env:"POSTGRES_DB,required"`
		MaxOpenConns int           `env:"DB_MAX_OPEN_CONNS,required"`
		MaxIdleConns int           `env:"DB_MAX_IDLE_CONNS,required"`
		MaxIdleTime  time.Duration `env:"DB_CONN_MAX_IDLE_TIME,required"`
	}
	Mail struct {
		Host     string `env:"SMTP_HOST,required"`
		Port     int    `env:"SMTP_PORT,required"`
		Username string `env:"SMTP_USERNAME,required"`
		Password string `env:"SMTP_PASSWORD,required"`
		Sender   string `env:"SMTP_SENDER,required"`
	}
}

func main() {
	var envFile string

	flag.StringVar(&envFile, "env", ".env", "Environment variables file name")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	err := godotenv.Load(envFile)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	var cfg config

	err = env.Parse(&cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.DB.DB_USER, cfg.DB.DB_PASSWORD, cfg.DB.DB_HOST, cfg.DB.DB_PORT, cfg.DB.DB_NAME)

	db, err := OpenDB(dsn, cfg.DB.MaxOpenConns, cfg.DB.MaxIdleConns, cfg.DB.MaxIdleTime)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("Database connection established")

	app := &application{
		config: cfg,
		logger: logger,
		models: models.NewModels(db),
		mailer: mail.New(cfg.Mail.Host, cfg.Mail.Port, cfg.Mail.Username, cfg.Mail.Password, cfg.Mail.Sender),
	}

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func OpenDB(DBName string, maxOpenConns int, maxIdleConns int, maxIdleTime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", DBName)
	if err != nil {
		return nil, fmt.Errorf("error opening database connection: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxIdleTime(maxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	return db, nil
}
