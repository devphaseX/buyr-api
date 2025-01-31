package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

func (app *application) getAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
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

	auditLogs, metadata, err := app.store.AuditLogs.GetAuditLogs(r.Context(), fq, user.AdminUser.AdminLevel)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"audit_logs": auditLogs,
		"metadata":   metadata,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getAuditLogByID(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	logID := app.readStringID(r, "logID")

	log, err := app.store.AuditLogs.GetAuditLogByID(r.Context(), logID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "audit log not found")
		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	if user.AdminUser.AdminLevel.GetRank() < log.AccessLevel {
		app.notFoundResponse(w, r, "audit log not found")
		return
	}

	response := envelope{
		"audit_log": log,
	}

	app.successResponse(w, http.StatusOK, response)
}
