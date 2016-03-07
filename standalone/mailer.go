package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/daemonl/moistrobot"
)

var configFilename string
var bodyName string
var email = &moistrobot.Email{}

var reFilename = regexp.MustCompile(`\.[0-9a-zA-Z]+$`)

func init() {
	flag.StringVar(&configFilename, "config", "/etc/moistrobot/config.json", "Config File")

	flag.StringVar(&email.To, "to", "", "The recipient's email address")
	flag.StringVar(&email.Subject, "subject", "moistrobot", "The email subject")
	flag.StringVar(&bodyName, "body", "-", "Body. '-' for stdin, if ends in \\.[a-zA-Z0-9]{3} will read a file, otherwise text")
}

func main() {
	flag.Parse()
	err := do()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func do() error {
	m := &moistrobot.Mailer{}
	err := fileNameToObject(configFilename, m)
	if err != nil {
		return err
	}

	if bodyName == "-" {
		email.Body = os.Stdin
		fmt.Println("Read body from stdin")
	} else if reFilename.MatchString(bodyName) {
		file, err := os.Open(bodyName)
		if err != nil {
			return err
		}
		defer file.Close()
		email.Body = file
		fmt.Printf("Read body from %s\n", bodyName)
	} else {
		email.Body = strings.NewReader(bodyName)
	}

	fmt.Printf("Send email to %s VIA %s\n", email.To, m.SMTP.Server)

	for _, arg := range flag.Args() {
		fmt.Printf("Add Attachment: %s\n", arg)
		err := email.AttachFile(arg)
		if err != nil {
			return err
		}
	}

	m.Send(email)
	return nil
}

func fileNameToObject(filename string, object interface{}) error {
	jsonFile, err := os.Open(filename)
	defer jsonFile.Close()
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(jsonFile)
	err = decoder.Decode(object)

	if err != nil {
		return err
	}

	return nil
}
