package domain

const (
	RoleUser  int8 = 0
	RoleAdmin int8 = 1

	StatusNormal int8 = 0
	StatusFrozen int8 = 1
)

func RoleName(role int8) string {
	if role == RoleAdmin {
		return "admin"
	}
	return "user"
}
