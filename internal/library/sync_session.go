package library

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
)

type syncSession struct{ scope, cookies, token string }
type syncSessionContextKey struct{}

var errSyncSessionChanged = errors.New("同步身份已变更，保留原任务等待重试")

func (s *Service) currentSyncSession() syncSession {
	var session syncSession
	var userID string
	if s.authSession != nil {
		userID, session.cookies, session.token = s.authSession()
	} else {
		session.cookies, session.token = s.credentials()
	}
	if session.cookies == "" {
		return session
	}
	if userID != "" && userID != "anonymous" && userID != "0" {
		session.scope = "user:" + userID
	} else {
		// Compatibility for credentials without a validated profile. Store only
		// a one-way fingerprint, never the credential itself, in queue ownership.
		session.scope = fmt.Sprintf("session:%x", sha256.Sum256([]byte(session.cookies)))
	}
	return session
}

func (s *Service) bindSyncSession(ctx context.Context) context.Context {
	if _, ok := ctx.Value(syncSessionContextKey{}).(syncSession); ok {
		return ctx
	}
	return context.WithValue(ctx, syncSessionContextKey{}, s.currentSyncSession())
}

func (s *Service) sessionForRequest(ctx context.Context) (syncSession, error) {
	session, ok := ctx.Value(syncSessionContextKey{}).(syncSession)
	if !ok {
		session = s.currentSyncSession()
	}
	if session.cookies == "" || !s.syncSessionCurrent(session) {
		return session, errSyncSessionChanged
	}
	return session, nil
}

func (s *Service) syncSessionCurrent(session syncSession) bool {
	return session.scope == s.currentSyncSession().scope
}
