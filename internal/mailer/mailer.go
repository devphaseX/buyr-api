package mailer

import "embed"

//go:embed "templates"
var templateFS embed.FS

var (
	ActivateAccountEmailTemplate = "activate_account_email.tmpl"
)

type Client interface {
	Send(option *MailOption, data any) error
}
