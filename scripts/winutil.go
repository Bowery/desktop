package main

import (
	"archive/zip"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var cmds = map[string]func(...string) error{"zip": zips, "aws": aws}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: winutil <zip|aws> [arguments]")
		os.Exit(1)
	}

	cmd, ok := cmds[os.Args[1]]
	if !ok {
		fmt.Fprintln(os.Stderr, "Cmd", os.Args[1], "not found.")
		os.Exit(1)
	}

	err := cmd(os.Args[2:]...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func zips(args ...string) error {
	if len(args) < 2 {
		return errors.New("Usage: winutil zip <source\\dir> <output>")
	}

	output, err := os.Create(args[1])
	if err != nil {
		return err
	}
	defer output.Close()
	zipWriter := zip.NewWriter(output)
	defer zipWriter.Close()

	return filepath.Walk(args[0], func(path string, info os.FileInfo, err error) error {
		if err != nil || args[0] == path || info.IsDir() {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Get the relative path, and convert separators to /.
		relPath, err := filepath.Rel(args[0], path)
		if err != nil {
			return err
		}
		relPath = strings.Replace(relPath, string(filepath.Separator), "/", -1)
		header.Name = relPath

		partWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		source, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(partWriter, source)
		if err != nil {
			return err
		}

		return source.Close()
	})
}

func aws(args ...string) error {
	if len(args) < 1 {
		return errors.New("Usage: winutil aws <path>")
	}
	name := filepath.Base(args[0])
	bucket := "bowery.sh"
	resource := "/" + bucket + "/" + name
	contentType := "application/octet-stream"
	date := time.Now().UTC().Format("Mon, 2 Jan 2006 15:04:05 -0700")
	signingBody := []byte("PUT\n\n" + contentType + "\n" + date + "\n" + resource)
	key := "AKIAI6ICZKWF5DYYTETA"
	secret := []byte("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")

	hash := hmac.New(sha1.New, secret)
	hash.Write(signingBody)
	signature := make([]byte, base64.StdEncoding.EncodedLen(hash.Size()))
	base64.StdEncoding.Encode(signature, hash.Sum(nil))

	input, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer input.Close()

	stat, err := input.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", "https://"+bucket+".s3.amazonaws.com/"+name, input)
	if err != nil {
		return err
	}
	req.Header.Set("Host", bucket+".s3.amazonaws.com")
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "AWS "+key+":"+string(signature))
	req.ContentLength = stat.Size()

	// Let insecure ssl go through, s3 has trouble routing the certificate if bucket contains periods.
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.New("Http status code: " + res.Status)
	}

	return nil
}
