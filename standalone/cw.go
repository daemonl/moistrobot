package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/cloudwatch"
)

var region string

var reFilename = regexp.MustCompile(`\.[0-9a-zA-Z]+$`)

func init() {

	flag.StringVar(&region, "region", "ap-southeast-2", "AWS Region")
	flag.StringVar(&bodyName, "body", "-", "Source '-' for stdin, *.[a-zA-Z0-9]+ for a file, or just text")

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
	auth, err := aws.EnvAuth()
	if err != nil {
		return err
	}
	cw, err := cloudwatch.NewCloudWatch(auth, aws.Regions[region].CloudWatchServicepoint)
	if err != nil {
		return err
	}

	var body io.Reader
	if bodyName == "-" {
		body = os.Stdin
		fmt.Println("Read body from stdin")
	} else if reFilename.MatchString(bodyName) {
		file, err := os.Open(bodyName)
		if err != nil {
			return err
		}
		defer file.Close()
		body = file
		fmt.Printf("Read body from %s\n", bodyName)
	} else {
		body = strings.NewReader(bodyName)
	}

	request := &cloudwatch.PutMetricData

	return nil
}
