package handlers

import (
	"net/http"
	"strings"

	"sob/pkg/models"
	"sob/pkg/session"
)

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	books, err := h.BookRepo.GetLatest(12)
	if err != nil {
		h.Logger.Error("GetLatest books error:", err)
		books = []*models.Book{}
	}

	data := map[string]interface{}{
		"Books": books,
	}

	// Получаем пользователя из сессии
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		h.Logger.Debug("No user session in context:", err)
	} else {
		h.Logger.Debugf("Found session for user ID: %d", sess.UserID)
		user, err := h.UserRepo.GetByID(int(sess.UserID))
		if err != nil {
			h.Logger.Error("Get user by ID error:", err)
		} else {
			data["User"] = user
			h.Logger.Debugf("User %s added to template data", user.Username)
		}
	}

	h.Tmpl.ExecuteTemplate(w, "index.html", data)
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Если пользователь уже авторизован, перенаправляем на главную
	if sess, err := session.SessionFromContext(r.Context()); err == nil && sess != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	h.Tmpl.ExecuteTemplate(w, "login.html", nil)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.UserRepo.Authorize(username, password)
	if err != nil {
		h.Logger.Error("Login error:", err)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	h.Logger.Infof("User %s logged in successfully", username)
	h.Sessions.Create(w, uint32(user.ID))
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	// Если пользователь уже авторизован, перенаправляем на главную
	if sess, err := session.SessionFromContext(r.Context()); err == nil && sess != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	h.Tmpl.ExecuteTemplate(w, "register.html", nil)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")

	user := &models.User{
		Username: username,
		Email:    email,
		Password: password,
	}

	err := h.UserRepo.Create(user)
	if err != nil {
		h.Logger.Error("Register error:", err)
		http.Error(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	h.Logger.Infof("User %s registered successfully", username)

	// После регистрации сразу логиним пользователя
	user, err = h.UserRepo.Authorize(username, password)
	if err != nil {
		h.Logger.Error("Auto login after register error:", err)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	h.Sessions.Create(w, uint32(user.ID))
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.Sessions.DestroyCurrent(w, r)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	books, err := h.BookRepo.GetByUserID(int(sess.UserID))
	if err != nil {
		h.Logger.Error("Get user books error:", err)
		books = []*models.Book{}
	}

	user, _ := h.UserRepo.GetByID(int(sess.UserID))

	h.Tmpl.ExecuteTemplate(w, "profile.html", map[string]interface{}{
		"Books": books,
		"User":  user,
	})
}

func (h *Handler) SearchBooks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	tagsParam := r.URL.Query().Get("tags")
	sortBy := r.URL.Query().Get("sort")
	
	var tags []string
	if tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
	}

	var books []*models.Book
	var err error

	if query != "" || len(tags) > 0 {
		books, err = h.BookRepo.Search(query, tags, sortBy)
		if err != nil {
			h.Logger.Error("Search books error:", err)
			books = []*models.Book{}
		}
	} else {
		books, err = h.BookRepo.GetLatest(12)
		if err != nil {
			h.Logger.Error("GetLatest books error:", err)
			books = []*models.Book{}
		}
	}

	// Получаем популярные теги для фильтра
	popularTags, _ := h.BookRepo.GetPopularTags(20)

	data := map[string]interface{}{
		"Books":       books,
		"Query":       query,
		"Tags":        tags,
		"SortBy":      sortBy,
		"PopularTags": popularTags,
	}

	// Получаем пользователя из сессии
	if sess, err := session.SessionFromContext(r.Context()); err == nil && sess != nil {
		user, err := h.UserRepo.GetByID(int(sess.UserID))
		if err != nil {
			h.Logger.Error("Get user by ID error:", err)
		} else {
			data["User"] = user
		}
	}

	h.Tmpl.ExecuteTemplate(w, "index.html", data)
}