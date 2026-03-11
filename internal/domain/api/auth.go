package api

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/stroppy-io/hatchet-workflow/internal/auth"
	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
)

func (h *Handler) Login(
	ctx context.Context,
	req *connect.Request[pb.LoginRequest],
) (*connect.Response[pb.LoginResponse], error) {
	msg := req.Msg

	var userID, username, encryptedPassword, role string
	err := h.pool.QueryRow(ctx,
		`SELECT id, username, encrypted_password, role FROM users WHERE username = $1`,
		msg.GetUsername(),
	).Scan(&userID, &username, &encryptedPassword, &role)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errInvalidCredentials)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(encryptedPassword), []byte(msg.GetPassword())); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errInvalidCredentials)
	}

	sessionID := ulid.Make().String()
	expiresAt := time.Now().Add(h.jwt.RefreshExpiresIn())

	if _, err := h.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES ($1, $2, now(), $3)`,
		sessionID, userID, expiresAt,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accessToken, _, err := h.jwt.GenerateAccessToken(userID, username, role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rawRefresh, err := h.jwt.GenerateRefreshToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(rawRefresh), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := h.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (id, session_id, token_hash) VALUES ($1, $2, $3)`,
		ulid.Make().String(), sessionID, string(hash),
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		User: &pb.User{
			Id:       userID,
			Username: username,
			Role:     role,
		},
	}), nil
}

func (h *Handler) RefreshToken(
	ctx context.Context,
	req *connect.Request[pb.RefreshTokenRequest],
) (*connect.Response[pb.RefreshTokenResponse], error) {
	rows, err := h.pool.Query(ctx,
		`SELECT rt.id, rt.session_id, rt.token_hash
		 FROM refresh_tokens rt
		 JOIN sessions s ON s.id = rt.session_id
		 WHERE rt.revoked = false AND s.expires_at > now()`,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var matchID, matchSessionID string
	for rows.Next() {
		var id, sessID, tokenHash string
		if err := rows.Scan(&id, &sessID, &tokenHash); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if bcrypt.CompareHashAndPassword([]byte(tokenHash), []byte(req.Msg.GetRefreshToken())) == nil {
			matchID = id
			matchSessionID = sessID
			break
		}
	}

	if matchID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errInvalidRefreshToken)
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = true WHERE id = $1`, matchID,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	newRefresh, err := h.jwt.GenerateRefreshToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newRefresh), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO refresh_tokens (id, session_id, token_hash) VALUES ($1, $2, $3)`,
		ulid.Make().String(), matchSessionID, string(newHash),
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var userID, username, role string
	err = tx.QueryRow(ctx,
		`SELECT u.id, u.username, u.role FROM users u
		 JOIN sessions s ON s.user_id = u.id WHERE s.id = $1`,
		matchSessionID,
	).Scan(&userID, &username, &role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accessToken, _, err := h.jwt.GenerateAccessToken(userID, username, role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
	}), nil
}

func (h *Handler) Logout(
	ctx context.Context,
	req *connect.Request[pb.LogoutRequest],
) (*connect.Response[pb.LogoutResponse], error) {
	claims, err := h.claimsFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = true
		 WHERE session_id IN (SELECT id FROM sessions WHERE user_id = $1)`,
		claims.UserID,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.LogoutResponse{}), nil
}

func (h *Handler) GetCurrentUser(
	ctx context.Context,
	req *connect.Request[pb.GetCurrentUserRequest],
) (*connect.Response[pb.GetCurrentUserResponse], error) {
	claims, err := h.claimsFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetCurrentUserResponse{
		User: &pb.User{
			Id:       claims.UserID,
			Username: claims.Username,
			Role:     claims.Role,
		},
	}), nil
}

func (h *Handler) claimsFromCtx(ctx context.Context) (*auth.Claims, error) {
	token, ok := ctx.Value(ctxKeyToken).(string)
	if !ok || token == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errMissingToken)
	}

	claims, err := h.jwt.ValidateToken(token)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	return claims, nil
}

type ctxKey string

const ctxKeyToken ctxKey = "auth_token"

func extractBearerToken(authorization string) string {
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}
	return ""
}
