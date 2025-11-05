package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"sob/pkg/session"
)

func (h *Handler) EditProfilePage(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	user, err := h.UserRepo.GetByID(int(sess.UserID))
	if err != nil {
		h.Logger.Error("Get user error:", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	h.Tmpl.ExecuteTemplate(w, "edit_profile.html", map[string]interface{}{
		"User": user,
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	sess, err := session.SessionFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		h.Logger.Error("Parse multipart form error:", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	userID := int(sess.UserID)
	username := r.FormValue("username")
	email := r.FormValue("email")
	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")

	// Получаем текущего пользователя для проверки изменений
	currentUser, err := h.UserRepo.GetByID(userID)
	if err != nil {
		h.Logger.Error("Get current user error:", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Проверяем, есть ли изменения в username или email
	hasChanges := (username != "" && username != currentUser.Username) || 
	             (email != "" && email != currentUser.Email) || 
	             newPassword != ""

	// Проверяем текущий пароль только если есть изменения
	if hasChanges {
		valid, err := h.UserRepo.CheckPassword(userID, currentPassword)
		if err != nil || !valid {
			h.Logger.Error("Invalid current password:", err)
			http.Error(w, "Invalid current password", http.StatusBadRequest)
			return
		}
	}

	// Обновляем username если изменился
	if username != "" && username != currentUser.Username {
		err = h.UserRepo.UpdateUsername(userID, username)
		if err != nil {
			h.Logger.Error("Update username error:", err)
			http.Error(w, "Failed to update username", http.StatusInternalServerError)
			return
		}
	}

	// Обновляем email если изменился
	if email != "" && email != currentUser.Email {
		err = h.UserRepo.UpdateEmail(userID, email)
		if err != nil {
			h.Logger.Error("Update email error:", err)
			http.Error(w, "Failed to update email", http.StatusInternalServerError)
			return
		}
	}

	// Обновляем пароль если указан новый
	if newPassword != "" {
		err = h.UserRepo.UpdatePassword(userID, newPassword)
		if err != nil {
			h.Logger.Error("Update password error:", err)
			http.Error(w, "Failed to update password", http.StatusInternalServerError)
			return
		}
	}

	// Обрабатываем загрузку аватарки
	avatarFile, avatarHeader, err := r.FormFile("avatar")
	if err == nil {
		defer avatarFile.Close()

		// Создаем директорию для аватарок если не существует
		avatarDir := "static/avatars"
		err = os.MkdirAll(avatarDir, 0755)
		if err != nil {
			h.Logger.Error("Create avatar dir error:", err)
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		// Генерируем имя файла
		avatarExt := filepath.Ext(avatarHeader.Filename)
		avatarFilename := fmt.Sprintf("avatar_%d%s", userID, avatarExt)
		avatarPath := filepath.Join(avatarDir, avatarFilename)

		// Сохраняем файл
		dst, err := os.Create(avatarPath)
		if err != nil {
			h.Logger.Error("Create avatar file error:", err)
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		_, err = io.Copy(dst, avatarFile)
		if err != nil {
			h.Logger.Error("Save avatar error:", err)
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		// Обновляем путь к аватарке в базе данных
		err = h.UserRepo.UpdateAvatar(userID, avatarPath)
		if err != nil {
			h.Logger.Error("Update avatar in DB error:", err)
			// Удаляем загруженный файл при ошибке
			os.Remove(avatarPath)
			http.Error(w, "Failed to update avatar", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/profile", http.StatusFound)
}