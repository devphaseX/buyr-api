package db

import (
	"context"
	"database/sql"
	"math/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

func New(addr string, maxOpenConns, maxIdleConns int, maxIdleTime string) (*sql.DB, error) {
	db, err := sql.Open("postgres", addr)

	if err != nil {
		return nil, err
	}

	// Set the maximum number of open (in-use + idle) connections in the pool. Note that
	// passing a value less than or equal to 0 will mean there is no limit.
	db.SetMaxOpenConns(maxOpenConns)

	// Set the maximum number of idle connections in the pool. Again, passing a value
	// less than or equal to 0 will mean there is no limit.
	db.SetMaxIdleConns(maxIdleConns)

	// Use the time.ParseDuration() function to convert the idle timeout duration string
	// to a time.Duration type.

	maxIdleDuration, err := time.ParseDuration(maxIdleTime)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(maxIdleDuration)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	defer cancel()

	err = db.PingContext(ctx)

	if err != nil {
		return nil, err
	}

	return db, nil
}

func GenerateULID() string {
	// Create a new entropy source using the current time
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate a new ULID
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)

	return id.String()
}
