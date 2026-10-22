package identity

type Role string

const (
	RoleOwner    Role = "owner"
	RoleCustomer Role = "customer"
	RoleCourier  Role = "courier"
	RoleAdmin    Role = "admin"
)

func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleCustomer, RoleCourier, RoleAdmin:
		return true
	default:
		return false
	}
}
