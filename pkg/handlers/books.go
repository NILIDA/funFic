package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sob/pkg/models"
	"sob/pkg/session"
	"sob/pkg/utils"

	"github.com/gorilla/mux"
)

func (h *Handler) UploadPage(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	user, _ := h.UserRepo.GetByID(int(sess.UserID))
	h.Tmpl.ExecuteTemplate(w, "upload.html", map[string]interface{}{
		"User": user,
	})
}

func (h *Handler) UploadBook(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	err = r.ParseMultipartForm(32 << 20) // 32 MB
	if err != nil {
		h.Logger.Error("Parse multipart form error:", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("book_file")
	if err != nil {
		h.Logger.Error("Get book file error:", err)
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	var coverPath string
	coverFile, coverHeader, err := r.FormFile("cover_image")
	if err == nil {
		defer coverFile.Close()
		coverExt := filepath.Ext(coverHeader.Filename)
		coverFilename := fmt.Sprintf("cover_%d%s", sess.UserID, coverExt)
		coverPath = filepath.Join("static/images", coverFilename)
		
		// Создаем директорию если не существует
		os.MkdirAll("static/images", 0755)
		
		dst, err := os.Create(coverPath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, coverFile)
		} else {
			h.Logger.Error("Save cover image error:", err)
		}
	}


	err = os.MkdirAll(h.UploadDir, 0755)
	if err != nil {
		h.Logger.Error("Create upload dir error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}


	filename := fmt.Sprintf("%d_%s", sess.UserID, header.Filename)
	filePath := filepath.Join(h.UploadDir, filename)


	dst, err := os.Create(filePath)
	if err != nil {
		h.Logger.Error("Create file error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		h.Logger.Error("Save file error:", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Create book record
	book := &models.Book{
		Title:       r.FormValue("title"),
		Author:      r.FormValue("author"),
		Description: r.FormValue("description"),
		Tags:        r.FormValue("tags"),
		Filename:    header.Filename,
		FilePath:    filePath,
		FileSize:    header.Size,
		CoverImage:  coverPath,
		UserID:      int(sess.UserID),
	}

	_, err = h.BookRepo.Create(book)
	if err != nil {
		h.Logger.Error("Create book record error:", err)
		// Удаляем загруженные файлы при ошибке
		os.Remove(filePath)
		if coverPath != "" {
			os.Remove(coverPath)
		}
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusFound)
}

func (h *Handler) BookDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	book, err := h.BookRepo.GetByID(id)
	if err != nil || book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Book": book,
	}

	// Получаем пользователя из сессии и его оценку для этой книги
	if sess, err := session.SessionFromContext(r.Context()); err == nil {
		user, err := h.UserRepo.GetByID(int(sess.UserID))
		if err != nil {
			h.Logger.Error("Get user by ID error:", err)
		} else {
			data["User"] = user
			
			// Получаем оценку пользователя для этой книги
			userRating, err := h.BookRepo.GetUserRating(int(sess.UserID), id)
			if err != nil {
				h.Logger.Error("Get user rating error:", err)
			} else {
				data["UserRating"] = userRating
			}
		}
	}

	h.Tmpl.ExecuteTemplate(w, "book_detail.html", data)
}

func (h *Handler) ReadBook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	book, err := h.BookRepo.GetByID(id)
	if err != nil || book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Book": book,
		"Ext":  strings.ToLower(filepath.Ext(book.Filename)),
	}

	if utils.IsTextFile(book.Filename) {
		content, err := utils.ReadBookContent(book.FilePath)
		if err != nil {
			h.Logger.Error("Read book file error:", err)
			data["Content"] = "Не удалось загрузить содержимое книги"
		} else {
			data["Content"] = content
			data["IsEditable"] = utils.IsEditableFormat(book.Filename)
		}
	}

	// Получаем пользователя из сессии
	if sess, err := session.SessionFromContext(r.Context()); err == nil {
		user, err := h.UserRepo.GetByID(int(sess.UserID))
		if err != nil {
			h.Logger.Error("Get user by ID error:", err)
		} else {
			data["User"] = user
			
			// Проверяем, может ли пользователь редактировать книгу
			if book.UserID == int(sess.UserID) {
				data["CanEdit"] = true
			}
		}
	}

	h.Tmpl.ExecuteTemplate(w, "read_book.html", data)
}

func (h *Handler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	// Проверяем, существует ли книга и принадлежит ли она пользователю
	book, err := h.BookRepo.GetByID(id)
	if err != nil || book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	// Проверяем, что пользователь удаляет свою книгу
	if book.UserID != int(sess.UserID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Удаляем файлы книги
	if book.FilePath != "" {
		os.Remove(book.FilePath)
	}
	if book.CoverImage != "" {
		os.Remove(book.CoverImage)
	}

	// Удаляем запись из базы данных
	err = h.BookRepo.Delete(id, int(sess.UserID))
	if err != nil {
		h.Logger.Error("Delete book error:", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusFound)
}


// RateBookHandler обрабатывает оценку книги
func (h *Handler) RateBook(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	bookID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	rating, err := strconv.Atoi(r.FormValue("rating"))
	if err != nil || rating < 1 || rating > 5 {
		http.Error(w, "Invalid rating", http.StatusBadRequest)
		return
	}

	err = h.BookRepo.RateBook(int(sess.UserID), bookID, rating)
	if err != nil {
		h.Logger.Error("Rate book error:", err)
		http.Error(w, "Failed to rate book", http.StatusInternalServerError)
		return
	}

	// Возвращаем на страницу книги
	http.Redirect(w, r, fmt.Sprintf("/books/%d", bookID), http.StatusFound)
}

func (h *Handler) AdvancedSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	tagsParam := r.URL.Query().Get("tags")
	sortBy := r.URL.Query().Get("sort")
	
	var tags []string
	if tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
	}

	books, err := h.BookRepo.Search(query, tags, sortBy)
	if err != nil {
		h.Logger.Error("Advanced search error:", err)
		books = []*models.Book{}
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
	if sess, err := session.SessionFromContext(r.Context()); err == nil {
		user, err := h.UserRepo.GetByID(int(sess.UserID))
		if err != nil {
			h.Logger.Error("Get user by ID error:", err)
		} else {
			data["User"] = user
		}
	}

	h.Tmpl.ExecuteTemplate(w, "index.html", data)
}


func (h *Handler) EditBookPage(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	book, err := h.BookRepo.GetByID(id)
	if err != nil || book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	// Проверяем, что пользователь - владелец книги
	if book.UserID != int(sess.UserID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Читаем содержимое книги если это редактируемый формат
	var content string
	if utils.IsEditableFormat(book.Filename) {
		contentBytes, err := os.ReadFile(book.FilePath)
		if err != nil {
			h.Logger.Error("Read book content error:", err)
			content = ""
		} else {
			content = string(contentBytes)
		}
	}

	data := map[string]interface{}{
		"Book":    book,
		"Content": content,
		"User":    nil,
		"CanEditContent": utils.IsEditableFormat(book.Filename),
	}

	user, _ := h.UserRepo.GetByID(int(sess.UserID))
	data["User"] = user

	h.Tmpl.ExecuteTemplate(w, "edit_book.html", data)
}



func (h *Handler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	// Проверяем, что книга существует и принадлежит пользователю
	book, err := h.BookRepo.GetByID(id)
	if err != nil || book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	if book.UserID != int(sess.UserID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	err = r.ParseForm()
	if err != nil {
		h.Logger.Error("Parse form error:", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Обновляем метаданные книги
	title := r.FormValue("title")
	author := r.FormValue("author")
	description := r.FormValue("description")
	tags := r.FormValue("tags")
	content := r.FormValue("content")

	// Обновляем содержимое файла если это текстовый формат
	if utils.IsTextFile(book.Filename) && content != "" {
		err = os.WriteFile(book.FilePath, []byte(content), 0644)
		if err != nil {
			h.Logger.Error("Update book file error:", err)
			http.Error(w, "Failed to update book content", http.StatusInternalServerError)
			return
		}
	}

	// Обновляем информацию в базе данных
	_, err = h.BookRepo.DB.Exec(`
		UPDATE books 
		SET title = ?, author = ?, description = ?, tags = ?
		WHERE id = ? AND user_id = ?
	`, title, author, description, tags, id, sess.UserID)

	if err != nil {
		h.Logger.Error("Update book record error:", err)
		http.Error(w, "Failed to update book", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/books/%d", id), http.StatusFound)
}