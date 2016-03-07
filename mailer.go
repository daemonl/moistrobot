package moistrobot

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"os"
	"path"
)

// SMTPConfig provides connection information to establish an authenticated,
// encrypted SMTP client
type SMTPConfig struct {

	// Server is the address (IP or Domain) to actually dial
	Server string `json:"server"`

	// Hello is a custom string to send in the HELLO command, defaults to the
	// value of Server above
	Hello string `json:"hello"`

	Username string `json:"username"`
	Password string `json:"password"`
}

// dial establishes a connection given the configuration
func (config *SMTPConfig) dial() (*smtp.Client, error) {
	smtpClient, err := smtp.Dial(config.Server)
	if err != nil {
		return nil, err
	}

	hello := config.Hello
	if len(hello) < 1 {
		hello = config.Server
	}

	if err := smtpClient.Hello(hello); err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ServerName: config.Hello,
	}
	if err := smtpClient.StartTLS(tlsConfig); err != nil {
		return nil, err
	}

	simpleAuth := smtp.PlainAuth("", config.Username, config.Password, config.Hello)
	if err := smtpClient.Auth(simpleAuth); err != nil {
		return nil, fmt.Errorf("authenticating: %s", err.Error())
	}

	return smtpClient, nil
}

// Email represents an email to send via the SMTP server with attachments
type Email struct {
	To          string
	From        string
	Subject     string
	Body        io.Reader
	Attachments []Attachment
}

// AttachFile adds a filesystem file to an email
func (email *Email) AttachFile(filename string) error {
	attachment, err := os.Open(filename)
	if err != nil {
		return err
	}
	a := &ReaderAttachment{
		Content:     attachment,
		Filename:    path.Base(filename),
		ContentType: mime.TypeByExtension(path.Ext(filename)),
	}
	email.Attachments = append(email.Attachments, a)
	return nil
}

// Attachment can be attached to an email as a mime part
type Attachment interface {
	Attach(*multipart.Writer) error
}

// ReaderAttachment is an implementation of Attachment to send from a standard
// io.Reader
type ReaderAttachment struct {
	Content     io.Reader
	ContentType string
	Filename    string
}

// Attach creates writes the attachment as a mime part to the writer, and
// closes and Content readers which are also io.ReaderCloser
func (attachment *ReaderAttachment) Attach(mimeWriter *multipart.Writer) error {

	hdr := textproto.MIMEHeader{}
	hdr.Add("Content-Type", attachment.ContentType)
	hdr.Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, attachment.Filename))
	hdr.Add("Content-Transfer-Encoding", "base64")
	part, err := mimeWriter.CreatePart(hdr)
	if err != nil {
		return err
	}
	enc := base64.NewEncoder(base64.StdEncoding, part)
	io.Copy(enc, attachment.Content)
	enc.Close()
	if rc, ok := attachment.Content.(io.ReadCloser); ok {
		rc.Close()
	}
	return nil
}

// Mailer sends emails through a single SMTP server
type Mailer struct {
	// SMTP is the connection and security settings for an SMTP server to use
	SMTP SMTPConfig `json:"smtp"`

	// From is the default sender address, can be overridden in the email
	From string `json:"from"`
}

// Send connects to the SMTP server, writes the email, and disconnects
func (m *Mailer) Send(email *Email) error {

	smtpClient, err := m.SMTP.dial()
	if err != nil {
		return err
	}

	defer smtpClient.Quit()
	defer smtpClient.Close()

	from := email.From
	if len(from) < 1 {
		from = m.From
	}

	if err := smtpClient.Mail(from); err != nil {
		return err
	}

	if err := smtpClient.Rcpt(email.To); err != nil {
		return err
	}

	writer, err := smtpClient.Data()
	if err != nil {
		return err
	}
	defer writer.Close()

	mimeWriter := multipart.NewWriter(writer)

	headers := map[string]string{
		"From":         from,
		"To":           email.To,
		"Subject":      email.Subject,
		"MIME-Version": "1.0",
		"Content-Type": `multipart/mixed; boundary="` + mimeWriter.Boundary() + `"`,
		"Precedence":   "bulk",
	}

	for key, val := range headers {
		fmt.Fprintf(writer, "%s: %s\n", key, val)
	}

	writer.Write([]byte("\n"))

	textHeader := textproto.MIMEHeader{}
	textHeader.Add("Content-Type", "text/plain")
	textPart, err := mimeWriter.CreatePart(textHeader)
	if err != nil {
		return err
	}

	io.Copy(textPart, email.Body)

	for _, attachment := range email.Attachments {
		err := attachment.Attach(mimeWriter)
		if err != nil {
			return err
		}

	}

	mimeWriter.Close()
	return nil
}
