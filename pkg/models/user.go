package models

import (
	"database/sql"
	"errors"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Avatar   string `json:"avatar"`
}

type UserRepo struct {
	DB *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{DB: db}
}

var (
	ErrNoUser  = errors.New("user not found")
	ErrBadPass = errors.New("invalid password")
)

func (r *UserRepo) Create(user *User) error {
	_, err := r.DB.Exec(
		"INSERT INTO users (username, email, password) VALUES (?, ?, ?)",
		user.Username, user.Email, user.Password,
	)
	return err
}

func (r *UserRepo) GetByUsername(username string) (*User, error) {
	user := &User{}
	err := r.DB.QueryRow(
		"SELECT id, username, email, password, avatar FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.Avatar)
	
	if err == sql.ErrNoRows {
		return nil, ErrNoUser
	}
	return user, err
}

func (r *UserRepo) GetByID(id int) (*User, error) {
	user := &User{}
	err := r.DB.QueryRow(
		"SELECT id, username, email, avatar FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Avatar)
	
	if err == sql.ErrNoRows {
		return nil, ErrNoUser
	}
	return user, err
}

func (r *UserRepo) Authorize(username, password string) (*User, error) {
	user, err := r.GetByUsername(username)
	if err != nil {
		return nil, ErrNoUser
	}
	
	if user.Password != password {
		return nil, ErrBadPass
	}
	
	return user, nil
}

// Новые методы для редактирования профиля
func (r *UserRepo) UpdateUsername(userID int, newUsername string) error {
	_, err := r.DB.Exec(
		"UPDATE users SET username = ? WHERE id = ?",
		newUsername, userID,
	)
	return err
}

func (r *UserRepo) UpdateEmail(userID int, newEmail string) error {
	_, err := r.DB.Exec(
		"UPDATE users SET email = ? WHERE id = ?",
		newEmail, userID,
	)
	return err
}

func (r *UserRepo) UpdatePassword(userID int, newPassword string) error {
	_, err := r.DB.Exec(
		"UPDATE users SET password = ? WHERE id = ?",
		newPassword, userID,
	)
	return err
}

func (r *UserRepo) UpdateAvatar(userID int, avatarPath string) error {
	_, err := r.DB.Exec(
		"UPDATE users SET avatar = ? WHERE id = ?",
		avatarPath, userID,
	)
	return err
}

func (r *UserRepo) CheckPassword(userID int, password string) (bool, error) {
	var dbPassword string
	err := r.DB.QueryRow(
		"SELECT password FROM users WHERE id = ?",
		userID,
	).Scan(&dbPassword)
	
	if err != nil {
		return false, err
	}
	
	return dbPassword == password, nil
}