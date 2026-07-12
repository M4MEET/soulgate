package gateway

// users.go — REST API handlers for user and team management.
//
// Routes (registered in Start()):
//
//	GET    /api/users          list all users         (admin)
//	POST   /api/users          create user            (admin)
//	GET    /api/users/me       current user identity  (any)
//	GET    /api/users/{id}     get user detail        (admin or self)
//	PUT    /api/users/{id}     update user            (admin or self)
//	DELETE /api/users/{id}     deactivate user        (admin)
//
//	GET    /api/teams          list teams             (any)
//	POST   /api/teams          create team            (admin)
//	GET    /api/teams/{id}     get team detail        (any)

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/M4MEET/soulgate/internal/auth"
)

// contextKey is the unexported type used for request-context values.
type contextKey string

const contextKeyUser contextKey = "sg_user"

// userFromContext retrieves the authenticated *auth.User stored by the
// user auth middleware, or nil when the middleware was not applied.
func userFromContext(ctx context.Context) *auth.User {
	v, _ := ctx.Value(contextKeyUser).(*auth.User)
	return v
}

// userAuthMiddleware is layered on top of apiAuthMiddleware. It reads the
// Bearer token, looks up the sg_user_ API key in the UserManager, and stores
// the resolved *auth.User in the request context. It also touches LastActive.
//
// If the gateway has no UserManager configured the middleware is a no-op.
func userAuthMiddleware(um *auth.UserManager, next http.Handler) http.Handler {
	if um == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token != "" && strings.HasPrefix(token, "sg_user_") {
			if u, err := um.GetUserByAPIKey(token); err == nil {
				um.TouchLastActive(u.ID)
				r = r.WithContext(context.WithValue(r.Context(), contextKeyUser, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireRole returns a middleware that enforces the calling user has one of
// the permitted roles. It falls through to the next handler when the gateway
// has no UserManager (auth not configured).
func requireRole(um *auth.UserManager, roles ...auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if um == nil {
			return next
		}
		roleSet := make(map[auth.Role]bool, len(roles))
		for _, r := range roles {
			roleSet[r] = true
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := userFromContext(r.Context())
			if u == nil {
				writeAPIAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !roleSet[u.Role] {
				writeAPIAuthError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- handleAPIUsers ---

// handleAPIUsers handles GET /api/users and POST /api/users.
func (g *Gateway) handleAPIUsers(w http.ResponseWriter, r *http.Request) {
	if g.userManager == nil {
		writeJSON401(w, "user management not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		g.listUsers(w, r)
	case http.MethodPost:
		g.createUser(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (g *Gateway) listUsers(w http.ResponseWriter, r *http.Request) {
	caller := userFromContext(r.Context())
	if caller == nil || caller.Role != auth.RoleAdmin {
		writeAPIAuthError(w, http.StatusForbidden, "admin role required")
		return
	}

	users := g.userManager.ListUsers()
	writeJSONOK(w, map[string]interface{}{"users": users})
}

func (g *Gateway) createUser(w http.ResponseWriter, r *http.Request) {
	caller := userFromContext(r.Context())
	if caller == nil || caller.Role != auth.RoleAdmin {
		writeAPIAuthError(w, http.StatusForbidden, "admin role required")
		return
	}

	var body struct {
		Username    string    `json:"username"`
		DisplayName string    `json:"display_name"`
		Email       string    `json:"email"`
		Role        auth.Role `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBadRequest(w, "invalid JSON: "+err.Error())
		return
	}
	if body.Username == "" {
		writeBadRequest(w, "username is required")
		return
	}
	if body.Role == "" {
		body.Role = auth.RoleDev
	}

	u, err := g.userManager.CreateUser(body.Username, body.DisplayName, body.Email, body.Role)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSONStatus(w, http.StatusCreated, u)
}

// --- handleAPIUserDetail ---

// handleAPIUserDetail routes GET/PUT/DELETE /api/users/{id} and
// GET /api/users/me.
func (g *Gateway) handleAPIUserDetail(w http.ResponseWriter, r *http.Request) {
	if g.userManager == nil {
		writeJSON401(w, "user management not configured")
		return
	}

	// Extract the segment after /api/users/
	segment := strings.TrimPrefix(r.URL.Path, "/api/users/")
	segment = strings.TrimSuffix(segment, "/")

	if segment == "me" {
		g.getMe(w, r)
		return
	}

	id := segment
	switch r.Method {
	case http.MethodGet:
		g.getUserDetail(w, r, id)
	case http.MethodPut:
		g.updateUser(w, r, id)
	case http.MethodDelete:
		g.deleteUser(w, r, id)
	default:
		writeMethodNotAllowed(w)
	}
}

func (g *Gateway) getMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	u := userFromContext(r.Context())
	if u == nil {
		writeAPIAuthError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	writeJSONOK(w, u)
}

func (g *Gateway) getUserDetail(w http.ResponseWriter, r *http.Request, id string) {
	caller := userFromContext(r.Context())
	if caller == nil {
		writeAPIAuthError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	// Allow admin or the user themselves.
	if caller.Role != auth.RoleAdmin && caller.ID != id {
		writeAPIAuthError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	u, err := g.userManager.GetUser(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONOK(w, u)
}

func (g *Gateway) updateUser(w http.ResponseWriter, r *http.Request, id string) {
	caller := userFromContext(r.Context())
	if caller == nil {
		writeAPIAuthError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if caller.Role != auth.RoleAdmin && caller.ID != id {
		writeAPIAuthError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeBadRequest(w, "invalid JSON: "+err.Error())
		return
	}

	// Non-admins cannot elevate their own role.
	if caller.Role != auth.RoleAdmin {
		delete(updates, "role")
		delete(updates, "active")
	}

	if err := g.userManager.UpdateUser(id, updates); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	u, err := g.userManager.GetUser(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONOK(w, u)
}

func (g *Gateway) deleteUser(w http.ResponseWriter, r *http.Request, id string) {
	caller := userFromContext(r.Context())
	if caller == nil || caller.Role != auth.RoleAdmin {
		writeAPIAuthError(w, http.StatusForbidden, "admin role required")
		return
	}

	if err := g.userManager.DeleteUser(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- handleAPITeams ---

// handleAPITeams handles GET /api/teams and POST /api/teams.
func (g *Gateway) handleAPITeams(w http.ResponseWriter, r *http.Request) {
	if g.userManager == nil {
		writeJSON401(w, "user management not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		g.listTeams(w, r)
	case http.MethodPost:
		g.createTeam(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (g *Gateway) listTeams(w http.ResponseWriter, r *http.Request) {
	writeJSONOK(w, map[string]interface{}{"teams": g.userManager.ListTeams()})
}

func (g *Gateway) createTeam(w http.ResponseWriter, r *http.Request) {
	caller := userFromContext(r.Context())
	if caller == nil || caller.Role != auth.RoleAdmin {
		writeAPIAuthError(w, http.StatusForbidden, "admin role required")
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBadRequest(w, "invalid JSON: "+err.Error())
		return
	}
	if body.Name == "" {
		writeBadRequest(w, "name is required")
		return
	}

	t, err := g.userManager.CreateTeam(body.Name, body.Description)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSONStatus(w, http.StatusCreated, t)
}

// --- handleAPITeamDetail ---

// handleAPITeamDetail handles GET /api/teams/{id}.
func (g *Gateway) handleAPITeamDetail(w http.ResponseWriter, r *http.Request) {
	if g.userManager == nil {
		writeJSON401(w, "user management not configured")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/teams/"), "/")
	t, err := g.userManager.GetTeam(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONOK(w, t)
}

// --- small response helpers ---

func writeJSONOK(w http.ResponseWriter, v interface{}) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeJSON401(w http.ResponseWriter, msg string) {
	writeAPIAuthError(w, http.StatusServiceUnavailable, msg)
}

func writeBadRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, msg)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"}) //nolint:errcheck
}
