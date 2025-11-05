package models

import (
	"database/sql"
	"strings"
)

type Book struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Description string  `json:"description"`
	Filename    string  `json:"filename"`
	FilePath    string  `json:"file_path"`
	FileSize    int64   `json:"file_size"`
	CoverImage  string  `json:"cover_image"`
	Tags        string  `json:"tags"`
	Rating      float64 `json:"rating"`
	RatingCount int     `json:"rating_count"`
	UserID      int     `json:"user_id"`
	Username    string  `json:"username"`
	UserRating  int     `json:"user_rating"`
	CreatedAt   string  `json:"created_at"`
}

type BookRepo struct {
	DB *sql.DB
}

func NewBookRepo(db *sql.DB) *BookRepo {
	return &BookRepo{DB: db}
}

func (r *BookRepo) Create(book *Book) (int64, error) {
	result, err := r.DB.Exec(
		"INSERT INTO books (title, author, description, filename, file_path, file_size, cover_image, tags, user_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		book.Title, book.Author, book.Description, book.Filename, book.FilePath, book.FileSize, book.CoverImage, book.Tags, book.UserID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *BookRepo) GetLatest(limit int) ([]*Book, error) {
	rows, err := r.DB.Query(`
		SELECT b.id, b.title, b.author, b.description, b.filename, b.file_path, b.file_size, 
		       b.cover_image, b.tags, b.rating, b.rating_count, b.user_id, u.username, b.created_at
		FROM books b
		JOIN users u ON b.user_id = u.id
		ORDER BY b.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []*Book
	for rows.Next() {
		book := &Book{}
		err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.Description, &book.Filename, 
			&book.FilePath, &book.FileSize, &book.CoverImage, &book.Tags, &book.Rating, 
			&book.RatingCount, &book.UserID, &book.Username, &book.CreatedAt)
		if err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, nil
}

func (r *BookRepo) GetByID(id int) (*Book, error) {
	book := &Book{}
	err := r.DB.QueryRow(`
		SELECT b.id, b.title, b.author, b.description, b.filename, b.file_path, b.file_size, 
		       b.cover_image, b.tags, b.rating, b.rating_count, b.user_id, u.username, b.created_at
		FROM books b
		JOIN users u ON b.user_id = u.id
		WHERE b.id = ?
	`, id).Scan(&book.ID, &book.Title, &book.Author, &book.Description, &book.Filename, 
		&book.FilePath, &book.FileSize, &book.CoverImage, &book.Tags, &book.Rating, 
		&book.RatingCount, &book.UserID, &book.Username, &book.CreatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return book, err
}

func (r *BookRepo) GetByIDWithUserRating(id, userID int) (*Book, error) {
	book := &Book{}
	err := r.DB.QueryRow(`
		SELECT b.id, b.title, b.author, b.description, b.filename, b.file_path, b.file_size, 
		       b.cover_image, b.tags, b.rating, b.rating_count, b.user_id, u.username, b.created_at,
		       COALESCE(rt.rating, 0) as user_rating
		FROM books b
		JOIN users u ON b.user_id = u.id
		LEFT JOIN ratings rt ON b.id = rt.book_id AND rt.user_id = ?
		WHERE b.id = ?
	`, userID, id).Scan(&book.ID, &book.Title, &book.Author, &book.Description, &book.Filename, 
		&book.FilePath, &book.FileSize, &book.CoverImage, &book.Tags, &book.Rating, 
		&book.RatingCount, &book.UserID, &book.Username, &book.CreatedAt, &book.UserRating)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return book, err
}

func (r *BookRepo) GetByUserID(userID int) ([]*Book, error) {
	rows, err := r.DB.Query(`
		SELECT b.id, b.title, b.author, b.description, b.filename, b.file_path, b.file_size, 
		       b.cover_image, b.tags, b.rating, b.rating_count, b.created_at
		FROM books b
		WHERE b.user_id = ?
		ORDER BY b.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []*Book
	for rows.Next() {
		book := &Book{}
		err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.Description, &book.Filename, 
			&book.FilePath, &book.FileSize, &book.CoverImage, &book.Tags, &book.Rating, 
			&book.RatingCount, &book.CreatedAt)
		if err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, nil
}

func (r *BookRepo) Search(query string, tags []string, sortBy string) ([]*Book, error) {
	var whereClause string
	var args []interface{}
	
	if query != "" {
		whereClause = "WHERE (b.title LIKE ? OR b.author LIKE ? OR b.description LIKE ? OR b.tags LIKE ?)"
		searchTerm := "%" + query + "%"
		args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
	}
	
	if len(tags) > 0 {
		if whereClause != "" {
			whereClause += " AND "
		} else {
			whereClause = "WHERE "
		}
		
		tagConditions := []string{}
		for _, tag := range tags {
			tagConditions = append(tagConditions, "b.tags LIKE ?")
			args = append(args, "%"+tag+"%")
		}
		whereClause += "(" + strings.Join(tagConditions, " OR ") + ")"
	}
	
	var orderBy string
	switch sortBy {
	case "rating":
		orderBy = "b.rating DESC, b.created_at DESC"
	case "newest":
		orderBy = "b.created_at DESC"
	case "popular":
		orderBy = "b.rating_count DESC, b.rating DESC"
	default:
		orderBy = "b.created_at DESC"
	}

	sqlQuery := `
		SELECT b.id, b.title, b.author, b.description, b.filename, b.file_path, b.file_size, 
		       b.cover_image, b.tags, b.rating, b.rating_count, b.user_id, u.username, b.created_at
		FROM books b
		JOIN users u ON b.user_id = u.id
		` + whereClause + `
		ORDER BY ` + orderBy

	rows, err := r.DB.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []*Book
	for rows.Next() {
		book := &Book{}
		err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.Description, &book.Filename, 
			&book.FilePath, &book.FileSize, &book.CoverImage, &book.Tags, &book.Rating, 
			&book.RatingCount, &book.UserID, &book.Username, &book.CreatedAt)
		if err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, nil
}

func (r *BookRepo) Delete(bookID, userID int) error {
	result, err := r.DB.Exec("DELETE FROM books WHERE id = ? AND user_id = ?", bookID, userID)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	
	return nil
}

func (r *BookRepo) RateBook(userID, bookID, rating int) error {
	// Начинаем транзакцию
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Вставляем или обновляем оценку
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO ratings (user_id, book_id, rating) 
		VALUES (?, ?, ?)
	`, userID, bookID, rating)
	if err != nil {
		return err
	}

	// Пересчитываем средний рейтинг для книги
	_, err = tx.Exec(`
		UPDATE books 
		SET rating = (
			SELECT AVG(rating) FROM ratings WHERE book_id = ?
		),
		rating_count = (
			SELECT COUNT(*) FROM ratings WHERE book_id = ?
		)
		WHERE id = ?
	`, bookID, bookID, bookID)
	if err != nil {
		return err
	}

	return tx.Commit()
}


func (r *BookRepo) GetPopularTags(limit int) ([]string, error) {
	rows, err := r.DB.Query(`
		WITH split_tags AS (
			SELECT DISTINCT trim(value) as tag
			FROM books, json_each('["' || replace(tags, ',', '","') || '"]')
			WHERE tags != ''
		)
		SELECT tag
		FROM split_tags
		WHERE tag != ''
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// GetUserRating возвращает оценку пользователя для книги
func (r *BookRepo) GetUserRating(userID, bookID int) (int, error) {
	var rating int
	err := r.DB.QueryRow(`
		SELECT rating FROM ratings 
		WHERE user_id = ? AND book_id = ?
	`, userID, bookID).Scan(&rating)
	
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return rating, err
}