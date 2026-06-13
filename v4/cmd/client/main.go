// Copyright 2026 Michael F. Collins, III
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISONG
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

// Client is a client program for testing the Key-Value Storage Engine.
// This program will read a put.txt file in the current working directory and
// will execute the commands in the file using the HTTP API exposed by the
// Key-Value Storage Engine.
//
// The put.txt file is a text file. Each line has a single command to be
// executed against the Key-Value Storage Engine. Each line has the format:
//
//	{method} {key} {value}\n
//
// The method field can either be PUT or GET. The key is a lowercase string
// that is the unique name for the value that is stored. The value is a string
// or string-encoded data that will be stored in the Key-Value Storage Engine.
// For GET requests, the string value is used to verify the response from the
// Key-Value Storage Engine. The value field extends until the end of the line.
//
// # Usage
//
//	client [--debug]
//
//	Flags:
//	  --debug: enables debug logging
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var levelVar slog.LevelVar

func main() {
	setupLogging()
	processFile()
}

func setupLogging() {
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		levelVar.Set(slog.LevelDebug)
	} else {
		levelVar.Set(slog.LevelInfo)
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: &levelVar,
	})
	slog.SetDefault(slog.New(handler))
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 300 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 60 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 300 * time.Second,
		},
	}
}

func processFile() {
	client := newHTTPClient()

	file, err := os.Open("put-delete.txt")
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			panic(err)
		}

		parts := strings.Split(strings.Trim(line, "\n"), " ")
		if len(parts) < 2 {
			panic(fmt.Errorf("invalid line: %s", line))
		}

		method := parts[0]
		key := parts[1]
		slog.Debug("executing_request", "method", method, "key", key)
		switch method {
		case "PUT":
			if len(parts) < 3 {
				panic(fmt.Errorf("invalid line: %s", line))
			}

			value := strings.Join(parts[2:], " ")
			if err := put(client, key, value); err != nil {
				panic(err)
			}

		case "GET":
			if len(parts) < 3 {
				panic(fmt.Errorf("invalid line: %s", line))
			}

			value := strings.Join(parts[2:], " ")
			if err := get(client, key, value); err != nil {
				panic(err)
			}

		case "DELETE":
			if err := delete(client, key); err != nil {
				panic(err)
			}

		default:
			slog.Error("invalid_method", "method", method)
			os.Exit(1)
		}
	}
}

func put(client *http.Client, key, value string) error {
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("http://localhost:8080/kv/%s", key),
		strings.NewReader(value),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain")
	ctx, cancel := context.WithTimeout(
		context.Background(),
		300*time.Second,
	)
	ctxReq := req.WithContext(ctx)
	resp, err := client.Do(ctxReq)
	cancel()
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf(
			"unexpected status code %d for PUT %s",
			resp.StatusCode,
			key,
		)
	}

	return nil
}

func get(client *http.Client, key, expectedValue string) error {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("http://localhost:8080/kv/%s", key),
		nil,
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	ctxReq := req.WithContext(ctx)
	resp, err := client.Do(ctxReq)
	cancel()
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if expectedValue == "NOT_FOUND" {
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}

		return fmt.Errorf("expected NOT_FOUND for key: %s", key)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"unexpected status code %d for GET %s",
			resp.StatusCode,
			key,
		)
	}

	value, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if string(value) != expectedValue {
		return fmt.Errorf("unexpected value: %s", value)
	}

	return nil
}

func delete(client *http.Client, key string) error {
	req, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("http://localhost:8080/kv/%s", key),
		nil,
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	ctxReq := req.WithContext(ctx)
	resp, err := client.Do(ctxReq)
	cancel()
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf(
			"unexpected status code %d for GET %s",
			resp.StatusCode,
			key,
		)
	}

	return nil
}
