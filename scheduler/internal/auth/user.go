package auth

const (
	RoleStudent  = "STUDENT"
	RoleLecturer = "LECTURER"
	RoleAdmin    = "ADMIN"
)

type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UserPage struct {
	Items    []User `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	HasNext  bool   `json:"has_next"`
}
