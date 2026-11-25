package entity

// EmailKind names a transactional email and its template (ADR 0010).
type EmailKind string

// Email kinds. Invitations are added in M2.
const (
	EmailPasswordReset   EmailKind = "password_reset"
	EmailPasswordChanged EmailKind = "password_changed"
)

// Valid reports whether k is a known kind.
func (k EmailKind) Valid() bool {
	switch k {
	case EmailPasswordReset, EmailPasswordChanged:
		return true
	default:
		return false
	}
}

// Email is a row of the outbox: an email to send at least once after the change that caused it commits.
type Email struct {
	ID   string
	Kind EmailKind
	To   string
	// Data fills the template. It is cleared once the email is sent: it may carry a raw token.
	Data EmailData
	// Attempts counts sends started, including the one in progress after a claim.
	Attempts int
}

// EmailData is what the templates need; which fields are set depends on the kind.
type EmailData struct {
	FullName string
	// Token is the raw password reset token (EmailPasswordReset).
	Token string
}

// Values returns the non-empty field values. None of them may appear in logs, traces or stored
// errors: the payload is personal data and may hold a token.
func (d EmailData) Values() []string {
	var vs []string
	for _, v := range []string{d.FullName, d.Token} {
		if v != "" {
			vs = append(vs, v)
		}
	}
	return vs
}
