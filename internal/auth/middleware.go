package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

// contextKey avoids context key collisions.
type contextKey string

const userContextKey contextKey = "user"

// UserContext holds the authenticated user info injected into request context.
type UserContext struct {
	UserID string
	Role   models.Role
}

// GetUser retrieves UserContext from context; returns nil when unauthenticated.
func GetUser(ctx context.Context) *UserContext {
	v, _ := ctx.Value(userContextKey).(*UserContext)
	return v
}

var errAccountDisabled = errors.New("account disabled")

// AuthMiddleware returns an HTTP middleware that authenticates requests.
// When auth.enabled=false it injects an anonymous admin UserContext and passes through.
func AuthMiddleware(
	enabled bool,
	jwtMgr *JWTManager,
	userStore *store.UserStore,
	tokenStore *store.TokenStore,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				ctx := context.WithValue(r.Context(), userContextKey, &UserContext{
					UserID: "anonymous",
					Role:   models.RoleAdmin,
				})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			token := extractBearer(r)
			if token == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			var uc *UserContext
			var err error
			if IsAPIToken(token) {
				uc, err = authenticateAPIToken(token, userStore, tokenStore)
			} else {
				uc, err = authenticateJWT(token, jwtMgr, userStore)
			}

			if err != nil {
				status := http.StatusUnauthorized
				if err == errAccountDisabled {
					status = http.StatusForbidden
				}
				http.Error(w, `{"error":"`+err.Error()+`"}`, status)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, uc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole wraps a handler and enforces that the caller has one of the given roles.
func RequireRole(roles ...models.Role) func(http.Handler) http.Handler {
	allowed := make(map[models.Role]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uc := GetUser(r.Context())
			if uc == nil || !allowed[uc.Role] {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractBearer pulls the token from the Authorization: Bearer <token> header.
func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func authenticateJWT(token string, jwtMgr *JWTManager, userStore *store.UserStore) (*UserContext, error) {
	claims, err := jwtMgr.Verify(token)
	if err != nil {
		return nil, errors.New("invalid token")
	}
	user, err := userStore.GetByID(claims.UserID)
	if err != nil {
		return nil, errors.New("invalid token")
	}
	if !user.Enabled {
		return nil, errAccountDisabled
	}
	return &UserContext{UserID: user.ID, Role: user.Role}, nil
}

func authenticateAPIToken(token string, userStore *store.UserStore, tokenStore *store.TokenStore) (*UserContext, error) {
	hash := Hash(token)
	apiToken, err := tokenStore.GetByHash(hash)
	if err != nil {
		return nil, errors.New("invalid token")
	}
	if apiToken.ExpiresAt != nil && apiToken.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("token expired")
	}
	user, err := userStore.GetByID(apiToken.UserID)
	if err != nil {
		return nil, errors.New("invalid token")
	}
	if !user.Enabled {
		return nil, errAccountDisabled
	}
	go tokenStore.UpdateLastUsed(apiToken.ID)
	return &UserContext{UserID: user.ID, Role: user.Role}, nil
}
