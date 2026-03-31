package domain

type ContextKey string

const (
	UserIDKey ContextKey = "user_id"
)

type LoginRequest struct {
	Username string `json:"username"`
}

type LoginResponse struct {
	Token string `json:"token"`
}
