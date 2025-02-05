package mailer

import "embed"

//go:embed "templates"
var templateFS embed.FS

var (
	ActivateAccountEmailTemplate = "activate_account_email.tmpl"
	RecoverAccountEmailTemplate  = "recover_account_email.tmpl"
	VendorActivationTemplate     = "vendor_activation_email.tmpl"
	AdminOnboardTemplate         = "admin_activation_email.tmpl"
	VerifyEmailTemplate          = "verify_email.tmpl"
)

type Client interface {
	Send(option *MailOption, data any) error
}
