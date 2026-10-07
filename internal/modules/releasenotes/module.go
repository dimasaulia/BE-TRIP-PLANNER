package releasenotes

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/controllers"
)

type ReleaseNoteModuleImpl struct {
	ReleaseNoteController controllers.ReleaseNoteController
}

func NewReleaseNoteModule(releaseNoteController controllers.ReleaseNoteController) *ReleaseNoteModuleImpl {
	return &ReleaseNoteModuleImpl{
		ReleaseNoteController: releaseNoteController,
	}
}

func (m *ReleaseNoteModuleImpl) Name() string {
	return "release_notes"
}

func (m *ReleaseNoteModuleImpl) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/release-notes", m.ReleaseNoteController.Find)
	mux.HandleFunc("GET /api/v1/release-notes/{id}", m.ReleaseNoteController.FindByID)
	mux.HandleFunc("POST /api/v1/release-notes", m.ReleaseNoteController.Create)
	mux.HandleFunc("PUT /api/v1/release-notes/{id}", m.ReleaseNoteController.Update)
	mux.HandleFunc("DELETE /api/v1/release-notes/{id}", m.ReleaseNoteController.Delete)
}
