package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	_  = iota             // 0
	KB = 1 << (10 * iota) // 1 << 10 = 1024
	MB                    // 1 << 20 = 1,048,576
)

// UploadHandler handles file upload requests.
func (app *application) uploadImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(MB * 10); err != nil {
		app.badRequestResponse(w, r, errors.New("invalid body paylaod"))
		return
	}
	formData := r.MultipartForm
	files := formData.File["images"]

	filePath := []string{}
	for _, header := range files {
		file, err := header.Open()

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		defer file.Close()

		imageURL, err := app.fileobject.UploadFile(r.Context(), "images", header.Filename, file)

		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		filePath = append(filePath, imageURL)
	}

	response := envelope{
		"file_urls": filePath,
	}

	app.successResponse(w, http.StatusOK, response)
}

// static files from a http.FileSystem.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
