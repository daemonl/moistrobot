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

type Mailer struct {
	SMTP SMTP   `json:"smtp"`
	From string `json:"from"`
}
type SMTP struct {
	Server   string `json:"server"`
	Hello    string `json:"hello"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Email struct {
	To          string
	Subject     string
	Body        io.Reader
	Attachments []Attachment
}

func (email *Email) AttachFile(filename string) error {
	attachment, err := os.Open(filename)
	if err != nil {
		return err
	}
	a := Attachment{
		Content:     attachment,
		Filename:    path.Base(filename),
		ContentType: mime.TypeByExtension(path.Ext(filename)),
	}
	email.Attachments = append(email.Attachments, a)
	return nil
}

type Attachment struct {
	Content     io.Reader
	ContentType string
	Filename    string
}

func (m *Mailer) Send(email *Email) error {

	smtpClient, err := smtp.Dial(m.SMTP.Server)
	if err != nil {
		return err
	}
	if err := smtpClient.Hello(m.SMTP.Hello); err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		ServerName: m.SMTP.Hello,
	}
	if err := smtpClient.StartTLS(tlsConfig); err != nil {
		return err
	}

	simpleAuth := smtp.PlainAuth("", m.SMTP.Username, m.SMTP.Password, m.SMTP.Hello)
	if err := smtpClient.Auth(simpleAuth); err != nil {
		return fmt.Errorf("AUTH: %s", err.Error())
	}
	defer smtpClient.Quit()
	defer smtpClient.Close()

	if err := smtpClient.Mail(m.From); err != nil {
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

	mw := multipart.NewWriter(writer)

	headers := map[string]string{
		"From":         m.From,
		"To":           email.To,
		"Subject":      email.Subject,
		"MIME-Version": "1.0",
		"Content-Type": `multipart/mixed; boundary="` + mw.Boundary() + `"`,
		"Precedence":   "bulk",
	}

	for key, val := range headers {
		fmt.Fprintf(writer, "%s: %s\n", key, val)
	}

	fmt.Fprintln(writer, "")

	textHeader := textproto.MIMEHeader{}
	textHeader.Add("Content-Type", "text/plain")
	textPart, err := mw.CreatePart(textHeader)
	if err != nil {
		return err
	}

	io.Copy(textPart, email.Body)

	for _, attachment := range email.Attachments {
		hdr := textproto.MIMEHeader{}
		hdr.Add("Content-Type", attachment.ContentType)
		hdr.Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, attachment.Filename))
		hdr.Add("Content-Transfer-Encoding", "base64")
		part, err := mw.CreatePart(hdr)
		if err != nil {
			return err
		}
		enc := base64.NewEncoder(base64.StdEncoding, part)
		io.Copy(enc, attachment.Content)
		enc.Close()
		if rc, ok := attachment.Content.(io.ReadCloser); ok {
			rc.Close()
		}
	}

	mw.Close()
	return nil
}
