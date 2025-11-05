package session

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Session struct {
	ID        string
	UserID    uint32
	CreatedAt time.Time
}

func NewSession(userID uint32) *Session {
	// лучше генерировать из заданного алфавита, но так писать меньше и для учебного примера ОК
	randID := make([]byte, 16)
	rand.Read(randID)

	return &Session{
		ID:        fmt.Sprintf("%x", randID),
		UserID:    userID,
		CreatedAt: time.Now(),
	}
}

var (
	ErrNoAuth = errors.New("No session found")
)

type sessKey string

var SessionKey sessKey = "sessionKey"

func SessionFromContext(ctx context.Context) (*Session, error) {
	sess, ok := ctx.Value(SessionKey).(*Session)
	if !ok || sess == nil {
		return nil, ErrNoAuth
	}
	return sess, nil
}

func ContextWithSession(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, SessionKey, sess)
}

type SessionsManager struct {
	data map[string]*Session
	mu   *sync.RWMutex
}

func NewSessionsManager() *SessionsManager {
	return &SessionsManager{
		data: make(map[string]*Session, 10),
		mu:   &sync.RWMutex{},
	}
}

func (sm *SessionsManager) Check(r *http.Request) (*Session, error) {
	sessionCookie, err := r.Cookie("session_id")
	if err == http.ErrNoCookie {
		return nil, ErrNoAuth
	}

	sm.mu.RLock()
	sess, ok := sm.data[sessionCookie.Value]
	sm.mu.RUnlock()

	if !ok {
		return nil, ErrNoAuth
	}

	return sess, nil
}

func (sm *SessionsManager) Create(w http.ResponseWriter, userID uint32) (*Session, error) {
	sess := NewSession(userID)

	sm.mu.Lock()
	sm.data[sess.ID] = sess
	sm.mu.Unlock()

	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    sess.ID,
		Expires:  time.Now().Add(90 * 24 * time.Hour),
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // true for HTTPS
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	return sess, nil
}

func (sm *SessionsManager) DestroyCurrent(w http.ResponseWriter, r *http.Request) error {
	sess, err := SessionFromContext(r.Context())
	if err != nil {
		return err
	}

	sm.mu.Lock()
	delete(sm.data, sess.ID)
	sm.mu.Unlock()

	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Expires:  time.Now().AddDate(0, 0, -1),
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	return nil
}