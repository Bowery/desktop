package main

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var cmds = map[string]func(...string) error{"zip": zips, "aws": aws, "json": jsonReplace}

func init() {
	// Let insecure ssl go through, s3 has trouble routing the certificate if bucket contains periods.
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: winutil <zip|aws|json> [arguments]")
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

// zips takes a direcory and writes the contents to a destination.
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

	// Walk the tree and copy files to the zip writer.
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

// aws takes a path and uploads its contents to aws, if a directory is given
// the directories children are uploaded in parallel.
func aws(args ...string) error {
	var (
		done = make(chan error, 1)
		wg   sync.WaitGroup
	)
	if len(args) < 1 {
		return errors.New("Usage: winutil aws <path>")
	}

	stat, err := os.Stat(args[0])
	if err != nil {
		return err
	}

	// If the path isn't a directory just upload it.
	if !stat.IsDir() {
		return upload(args[0], stat)
	}

	dir, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer dir.Close()

	stats, err := dir.Readdir(0)
	if err != nil {
		return err
	}

	// Loop stats and start uploads in parallel.
	for _, stat := range stats {
		wg.Add(1)

		go func(info os.FileInfo) {
			defer wg.Done()

			err := upload(filepath.Join(args[0], info.Name()), info)
			if err != nil {
				done <- err
			}
		}(stat)
	}

	// Wait and then signal done.
	go func() {
		wg.Wait()
		done <- nil
	}()

	return <-done
}

// json takes a path to a json file and the key/value to set.
func jsonReplace(args ...string) error {
	if len(args) < 3 {
		return errors.New("Usage: winutil json <path> <key> <value>")
	}

	file, err := os.OpenFile(args[0], os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	contents := make(map[string]interface{})
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&contents)
	if err != nil {
		return err
	}

	contents[args[1]] = args[2]
	data, err := json.MarshalIndent(contents, "", "  ")
	if err != nil {
		return err
	}

	// Seek back to the beginning of the file after truncating.
	err = file.Truncate(0)
	if err != nil {
		return err
	}
	_, err = file.Seek(0, os.SEEK_SET)
	if err != nil {
		return err
	}

	_, err = io.Copy(file, bytes.NewBuffer(data))
	return err
}

// upload reads a file and uploads its contents to aws.
func upload(path string, stat os.FileInfo) error {
	name := filepath.Base(path)
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

	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()

	req, err := http.NewRequest("PUT", "https://"+bucket+".s3.amazonaws.com/"+name, input)
	if err != nil {
		return err
	}
	req.Header.Set("Host", bucket+".s3.amazonaws.com")
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "AWS "+key+":"+string(signature))
	req.ContentLength = stat.Size()

	fmt.Println("Started upload for", name+"...")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.New("Http status code: " + res.Status)
	}

	fmt.Println("Upload for", name, "complete!")
	return nil
}
