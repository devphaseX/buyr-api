package main

import (
	"context"
	"flag"
	"log"
	"unicode/utf8"

	"github.com/devphaseX/buyr-api.git/internal/db"
	"github.com/devphaseX/buyr-api.git/internal/env"
	"github.com/devphaseX/buyr-api.git/internal/store"
)

type config struct {
	db dbConfig
}

type dbConfig struct {
	dsn          string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}

func main() {
	// Define flags for admin information
	firstName := flag.String("firstName", "", "First name of the admin")
	lastName := flag.String("lastName", "", "Last name of the admin")
	email := flag.String("email", "", "Email of the admin")
	adminLevel := flag.String("adminLevel", "none", "Admin level (none, basic, full)")

	password := flag.String("password", "none", "provide your password")
	// Parse the flags
	flag.Parse()

	// Validate required fields
	if *firstName == "" || *lastName == "" || *email == "" {
		log.Fatal("firstName, lastName, and email are required")
	}

	if utf8.RuneCountInString(*password) < 8 {
		log.Fatal("password should be 8 character long")

	}

	// Map the admin level from string to store.AdminLevel
	var level store.AdminLevel
	switch *adminLevel {
	case "none":
		level = store.AdminLevelNone
	case "super":
		level = store.AdminLevelSuper
	case "manager":
		level = store.AdminLevelManager
	default:
		log.Fatalf("Invalid adminLevel: %s", *adminLevel)
	}

	cfg := config{
		db: dbConfig{
			dsn:          env.GetString("DB_ADDR", "postgres://mingle:adminpassword@localhost/mingle?sslmode=disable"),
			maxOpenConns: env.GetInt("DB_MAX_OPEN_CONNS", 30),
			maxIdleConns: env.GetInt("DB_MAX_IDLE_CONNS", 30),
			maxIdleTime:  env.GetString("DB_MAX_IDLE_TIME", "15m"),
		},
	}

	db, err := db.New(cfg.db.dsn, cfg.db.maxOpenConns, cfg.db.maxIdleConns, cfg.db.maxIdleTime)
	if err != nil {
		log.Panic(err)
	}

	if err != nil {
		log.Panic(err)
	}

	userStore := store.NewUserModel(db)

	newAdminUser := &store.AdminUser{
		FirstName:  *firstName,
		LastName:   *lastName,
		AdminLevel: level,
		User: store.User{
			Email: *email,
			Role:  store.AdminRole,
		},
	}

	if err := newAdminUser.User.Password.Set(*password); err != nil {
		log.Panic(err)
	}

	ctx := context.Background()

	err = userStore.CreateAdminUser(ctx, newAdminUser)
	if err != nil {
		log.Panic(err)
	}

	log.Println("Admin user created successfully")
}
