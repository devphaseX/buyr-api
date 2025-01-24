package mailer

import (
	"bytes"
	"fmt"
	"text/template"

	"go.uber.org/zap"
	gomail "gopkg.in/mail.v2"
)

type MailTrapClient struct {
	fromEmail       string
	smtpAddr        string
	smtpSandboxAddr string
	smtpPort        int
	username        string
	isSandbox       bool
	password        string
	logger          *zap.SugaredLogger
}

func NewMailTrapClient(fromEmail, smtpAddr, smtpSandboxAddr, username, password string, smtpPort int, isSandbox bool, logger *zap.SugaredLogger) *MailTrapClient {
	return &MailTrapClient{
		smtpAddr:        smtpAddr,
		smtpSandboxAddr: smtpSandboxAddr,
		smtpPort:        smtpPort,
		username:        username,
		isSandbox:       isSandbox,
		password:        password,
		fromEmail:       fromEmail,
		logger:          logger,
	}
}

type MailOption struct {
	FromEmail    string
	TemplateFile string
	To           []string
	CC           []string
	BCC          []string
	AttachFiles  []string
}

func (c *MailTrapClient) Send(option *MailOption, data any) error {
	if option == nil {
		return fmt.Errorf("nil mail option received")
	}

	message := gomail.NewMessage()

	templateFile := option.TemplateFile

	tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)

	if err != nil {
		return err
	}

	subject := new(bytes.Buffer)

	err = tmpl.ExecuteTemplate(subject, "subject", data)

	if err != nil {
		return err
	}

	body := new(bytes.Buffer)

	err = tmpl.ExecuteTemplate(body, "body", data)
	if err != nil {
		return err
	}

	message.SetHeader("From", c.fromEmail)
	message.SetHeader("To", option.To...)
	message.SetHeader("Subject", subject.String())

	message.SetBody("text/html", body.String())

	smtpAddr := c.smtpAddr

	if c.isSandbox {
		smtpAddr = c.smtpSandboxAddr
	}

	dailer := gomail.NewDialer(smtpAddr, c.smtpPort, c.username, c.password)
	err = dailer.DialAndSend(message)

	if err != nil {
		c.logger.Errorw("Failed to send email", "email", option.To)
		return err
	}

	c.logger.Infow("Email sent", "email", option.To)
	return nil
}
