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

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	client := http.DefaultClient

	inputFilename := "put.txt"
	if len(os.Args) > 1 {
		inputFilename = os.Args[1]
	}

	file, err := os.Open(inputFilename)
	if err != nil {
		log.Fatalf("failed to open %s: %v", inputFilename, err)
	}

	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReader(file)
	count := 0
	ctx := context.Background()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			log.Fatal("failed to read line:", err)
		}

		parts := strings.Split(strings.Trim(line, "\n"), " ")
		if len(parts) < 2 {
			panic(fmt.Errorf("invalid line: %s", line))
		}

		method := parts[0]
		key := parts[1]
		var value string
		if len(parts) > 2 {
			value = strings.Join(parts[2:], " ")
		}

		count++
		fmt.Printf("\r%d", count)
		switch method {
		case "DELETE":
			if err := deleteKey(ctx, client, key); err != nil {
				log.Fatal("failed to DELETE:", err)
			}

		case "PUT":
			if err := put(ctx, client, key, value); err != nil {
				log.Fatal("failed to PUT:", err)
			}

		case "GET":
			if err := get(ctx, client, key, value); err != nil {
				log.Fatal("failed to GET:", err)
			}
		}
	}
}

func put(
	ctx context.Context,
	client *http.Client,
	key string,
	value string,
) error {
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("http://localhost:8080/kv/%s", key),
		strings.NewReader(value),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	ctxReq := req.WithContext(ctx)
	resp, err := client.Do(ctxReq)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func deleteKey(ctx context.Context, client *http.Client, key string) error {
	req, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("http://localhost:8080/kv/%s", key),
		nil,
	)
	if err != nil {
		return err
	}

	ctxReq := req.WithContext(ctx)
	resp, err := client.Do(ctxReq)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("uxpected status code: %d", resp.StatusCode)
	}

	return nil
}

func get(
	ctx context.Context,
	client *http.Client,
	key string,
	value string,
) error {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("http://localhost:8080/kv/%s", key),
		nil,
	)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/plain; charset=utf-8")
	ctxReq := req.WithContext(ctx)
	resp, err := client.Do(ctxReq)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if value == "NOT_FOUND" {
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}

		return fmt.Errorf("expected NOT_FOUND for key %s", key)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"unexpected status code %d for key %s",
			resp.StatusCode,
			key,
		)
	}

	actualValue, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if string(actualValue) != value {
		return fmt.Errorf("expected %s for key %s", value, key)
	}

	return nil
}
