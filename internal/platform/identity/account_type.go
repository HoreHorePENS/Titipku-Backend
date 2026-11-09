package identity

// AccountType membedakan persona pengguna di aplikasi:
// penitip (produsen yang menitipkan dagangan) dan pemilik_toko.
type AccountType string

const (
	AccountTypePenitip     AccountType = "penitip"
	AccountTypePemilikToko AccountType = "pemilik_toko"
)

// ValidAccountType mengembalikan true untuk nilai yang dikenal.
// Token lama tanpa claim account_type diperlakukan sebagai penitip.
func ValidAccountType(value string) bool {
	return AccountType(value).Valid()
}

func (t AccountType) Valid() bool {
	switch t {
	case AccountTypePenitip, AccountTypePemilikToko:
		return true
	default:
		return false
	}
}
