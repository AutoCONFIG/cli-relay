package user

type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type RefreshRequest struct {
	Token string `json:"token"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type UpdateEmailRequest struct {
	Password string `json:"password"`
	Email    string `json:"email"`
}

type CreateKeyRequest struct {
	Name        string  `json:"name"`
	IPWhitelist string  `json:"ip_whitelist"`
	ExpiresAt   *string `json:"expires_at"`
	Models      string  `json:"models"`
	Permissions string  `json:"permissions"`
}

type ProfileResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Status    string `json:"status"`
	Balance   int64  `json:"balance"`
	CreatedAt string `json:"created_at"`
}

type KeyResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Key         string  `json:"key"`
	Enabled     bool    `json:"enabled"`
	IPWhitelist string  `json:"ip_whitelist"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	Models      string  `json:"models"`
	Permissions string  `json:"permissions"`
	CreatedAt   string  `json:"created_at"`
}

type SubscriptionResponse struct {
	PlanID   string `json:"plan_id"`
	PlanName string `json:"plan_name"`
	PlanType string `json:"plan_type"`
	Status   string `json:"status"`
}

type RedeemRequest struct {
	Code string `json:"code"`
}
