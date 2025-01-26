package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

func (app *application) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	userProfile, err := app.store.Users.FlattenUser(r.Context(), user)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"user": userProfile,
	})
}

func (app *application) getUser(ctx context.Context, userID string) (*store.User, error) {
	var (
		user *store.User
		err  error
	)

	if app.cfg.redisCfg.enabled {
		user, err = app.cacheStore.Users.Get(ctx, userID)

		if !(err == nil || errors.Is(err, store.ErrRecordNotFound)) {
			app.logger.Errorf("Error fetching user from cache: %v", err)
			return nil, err
		}

		if user != nil {
			app.logger.Infow("cache hit", "key", "user", "id", userID)
			return user, nil
		}
	}

	user, err = app.store.Users.GetByID(ctx, userID)

	if err != nil {
		return nil, fmt.Errorf("error fetching user from database: %w", err)
	}

	app.logger.Infof("fetched user %v from the database", userID)
	err = app.cacheStore.Users.Set(ctx, user)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (app *application) getNormalUsers(w http.ResponseWriter, r *http.Request) {
	fq := store.PaginateQueryFilter{
		Page:         1,
		PageSize:     20,
		Sort:         "created_at",
		SortSafelist: []string{"created_at", "-created_at"},
	}

	if err := fq.Parse(r); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	users, metadata, err := app.store.Users.GetNormalUsers(r.Context(), fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"users":    users,
		"metadata": metadata,
	})
}
