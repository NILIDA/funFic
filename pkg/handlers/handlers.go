package handlers

import (
	"html/template"

	"sob/pkg/models"
	"sob/pkg/session"

	"go.uber.org/zap"
)

type Handler struct {
	Tmpl      *template.Template
	Logger    *zap.SugaredLogger
	UserRepo  *models.UserRepo
	BookRepo  *models.BookRepo
	Sessions  *session.SessionsManager
	UploadDir string
}

