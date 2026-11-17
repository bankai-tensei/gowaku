package platform

// SecureInputWindow is a Window that can treat what is typed as a secret,
// while a password field has the keyboard. With secure input on, the IME
// stays off whatever SetIMEEnabled says, so that no input method composes or
// learns the text, and where the system has a way to keep other programs from
// reading the keys, it is used: secure event input on macOS, for as long as
// the window is the key window. Repeated calls with the same value count as
// one. All four backends implement it. See docs/ADR/0088-input-obscure.md.
type SecureInputWindow interface {
	SetSecureInput(secure bool)
}
