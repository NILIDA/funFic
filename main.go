package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"sob/pkg/handlers"
	"sob/pkg/middleware"
	"sob/pkg/models"
	"sob/pkg/session"
	"sob/pkg/utils"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func main() {
	// Инициализация логгера
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	// Инициализация базы данных
	db, err := initDB()
	if err != nil {
		sugar.Fatal("Failed to init database:", err)
	}
	defer db.Close()

	// Инициализация репозиториев
	userRepo := models.NewUserRepo(db)
	bookRepo := models.NewBookRepo(db)

	// Создаем директории если не существуют
	os.MkdirAll("static/uploads", 0755)
	os.MkdirAll("static/images", 0755)
	os.MkdirAll("static/avatars", 0755)
	os.MkdirAll("templates", 0755)

	// Сначала создаем funcMap с функциями для шаблонов
	funcMap := template.FuncMap{
		"FirstChar": func(s string) string {
			if len(s) > 0 {
				return strings.ToUpper(string(s[0]))
			}
			return "U"
		},
		"split": strings.Split,
		"join":  strings.Join,
		"formatFileSize": utils.FormatFileSize,
		"without": func(tags []string, exclude string) []string {
			var result []string
			for _, tag := range tags {
				if tag != exclude {
					result = append(result, tag)
				}
			}
			return result
		},
		"seq": func(n int) []int {
			var result []int
			for i := 1; i <= n; i++ {
				result = append(result, i)
			}
			return result
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		
	}

	// Загрузка шаблонов С функциями
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/*.html"))

	// Инициализация менеджера сессий
	sessionsManager := session.NewSessionsManager()

	// Инициализация обработчиков
	handler := &handlers.Handler{
		Tmpl:      tmpl,
		Logger:    sugar,
		UserRepo:  userRepo,
		BookRepo:  bookRepo,
		Sessions:  sessionsManager,
		UploadDir: "static/uploads",
	}

	// Создание маршрутизатора
	router := mux.NewRouter()

	// Глобальные middleware
	router.Use(middleware.Panic)
	router.Use(middleware.AccessLog(sugar))
	router.Use(middleware.Auth(sessionsManager)) // Всегда проверяем сессии

	// Статические файлы
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Публичные маршруты (доступны всем)
	router.HandleFunc("/", handler.Index)
	router.HandleFunc("/login", handler.LoginPage).Methods("GET")
	router.HandleFunc("/login", handler.Login).Methods("POST")
	router.HandleFunc("/register", handler.RegisterPage).Methods("GET")
	router.HandleFunc("/register", handler.Register).Methods("POST")
	router.HandleFunc("/books/{id}", handler.BookDetail)
	router.HandleFunc("/books/{id}/read", handler.ReadBook)
	router.HandleFunc("/search", handler.AdvancedSearch)
	router.HandleFunc("/books/{id}/rate", handler.RateBook).Methods("POST")
	router.HandleFunc("/books/{id}", handler.BookDetail)



	// Защищенные маршруты (требуют авторизации)
	protected := router.PathPrefix("").Subrouter()
	protected.Use(middleware.RequireAuth(sessionsManager))
	
	protected.HandleFunc("/upload", handler.UploadPage).Methods("GET")
	protected.HandleFunc("/upload", handler.UploadBook).Methods("POST")
	protected.HandleFunc("/profile", handler.Profile)
	protected.HandleFunc("/edit-profile", handler.EditProfilePage).Methods("GET")
	protected.HandleFunc("/update-profile", handler.UpdateProfile).Methods("POST")
	protected.HandleFunc("/books/{id}/delete", handler.DeleteBook).Methods("POST")
	protected.HandleFunc("/logout", handler.Logout).Methods("POST")
	protected.HandleFunc("/books/{id}/rate", handler.RateBook).Methods("POST")
	protected.HandleFunc("/books/{id}/edit", handler.EditBookPage).Methods("GET")
	protected.HandleFunc("/books/{id}/update", handler.UpdateBook).Methods("POST")

	// Запуск сервера
	port := ":8080"
	sugar.Infow("Starting server",
		"port", port,
		"url", "http://localhost"+port,
	)

	if err := http.ListenAndServe(port, router); err != nil {
		sugar.Fatal("Server error:", err)
	}
}

func initDB() (*sql.DB, error) {
	// Создаем директорию если не существует
	os.MkdirAll("data", 0755)
	
	db, err := sql.Open("sqlite3", "data/app.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Создаем таблицы
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	// Таблица пользователей
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username VARCHAR(50) UNIQUE NOT NULL,
			email VARCHAR(100) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			avatar VARCHAR(255) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %v", err)
	}

	// Таблица книг
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title VARCHAR(255) NOT NULL,
			author VARCHAR(255) NOT NULL,
			description TEXT,
			filename VARCHAR(255) NOT NULL,
			file_path VARCHAR(500) NOT NULL,
			file_size INTEGER NOT NULL,
			cover_image VARCHAR(500),
			tags TEXT DEFAULT '',
			rating FLOAT DEFAULT 0,
			rating_count INTEGER DEFAULT 0,
			user_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create books table: %v", err)
	}

	// Таблица оценок
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ratings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			book_id INTEGER NOT NULL,
			rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
			FOREIGN KEY (book_id) REFERENCES books (id) ON DELETE CASCADE,
			UNIQUE(user_id, book_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create ratings table: %v", err)
	}

	// Создаем индексы
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_books_search ON books(title, author, description, tags)`,
		`CREATE INDEX IF NOT EXISTS idx_books_user ON books(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_user_book ON ratings(user_id, book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_book ON ratings(book_id)`,
	}

	for _, index := range indexes {
		_, err = db.Exec(index)
		if err != nil {
			return fmt.Errorf("failed to create index: %v", err)
		}
	}

	// Добавляем новые поля если они не существуют
	alterStatements := []string{
		`ALTER TABLE books ADD COLUMN tags TEXT DEFAULT ''`,
		`ALTER TABLE books ADD COLUMN rating FLOAT DEFAULT 0`,
		`ALTER TABLE books ADD COLUMN rating_count INTEGER DEFAULT 0`,
	}

	for _, alter := range alterStatements {
		db.Exec(alter) // Игнорируем ошибки если поля уже существуют
	}

	return nil
}