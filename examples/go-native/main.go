// Example: Using Stowry with stowry-go native signing
//
// This example demonstrates using presigned URLs with Stowry's native
// signing scheme via the stowry-go SDK.
//
// Run Stowry first:
//
//	stowry serve --config ../config.yaml
//
// Then run this example:
//
//	cd examples/go-native
//	go mod tidy
//	go run main.go
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	stowry "github.com/sagarc03/stowry-go"
	"gopkg.in/yaml.v3"
)

const (
	stowryEndpoint = "http://localhost:5708"
	configPath     = "../config.yaml"
)

type exampleConfig struct {
	Auth struct {
		Keys struct {
			Inline []struct {
				AccessKey string `yaml:"access_key"`
				SecretKey string `yaml:"secret_key"`
			} `yaml:"inline"`
		} `yaml:"keys"`
	} `yaml:"auth"`
}

func main() {
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Auth.Keys.Inline) == 0 {
		log.Fatal("no auth keys found in config")
	}

	// Create stowry-go client
	client := stowry.NewClient(
		stowryEndpoint,
		cfg.Auth.Keys.Inline[0].AccessKey,
		cfg.Auth.Keys.Inline[0].SecretKey,
	)

	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Upload a file
	key := "/hello.txt"
	content := []byte("Hello from stowry-go!")
	contentType := "text/plain"

	fmt.Println("=== Upload ===")
	err = uploadFile(client, httpClient, key, content, contentType)
	if err != nil {
		log.Fatalf("upload failed: %v", err)
	}
	fmt.Printf("Uploaded: %s\n", key)

	// Download the file
	fmt.Println("\n=== Download ===")
	downloaded, err := downloadFile(client, httpClient, key)
	if err != nil {
		log.Fatalf("download failed: %v", err)
	}
	fmt.Printf("Content: %s\n", string(downloaded))

	// Generate presigned URLs
	fmt.Println("\n=== Presigned URLs ===")

	getURL := client.PresignGet(key, 900)
	fmt.Printf("GET URL: %s\n", getURL)

	putURL := client.PresignPut("/presigned-upload.txt", 900)
	fmt.Printf("PUT URL: %s\n", putURL)

	deleteURL := client.PresignDelete(key, 900)
	fmt.Printf("DELETE URL: %s\n", deleteURL)

	// Delete the file
	fmt.Println("\n=== Delete ===")
	err = deleteFile(client, httpClient, key)
	if err != nil {
		log.Fatalf("delete failed: %v", err)
	}
	fmt.Printf("Deleted: %s\n", key)
}

func loadConfig(path string) (*exampleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg exampleConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func uploadFile(client *stowry.Client, httpClient *http.Client, key string, content []byte, contentType string) error {
	presignURL := client.PresignPut(key, 900)

	req, err := http.NewRequest(http.MethodPut, presignURL, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s - %s", resp.Status, string(body))
	}

	return nil
}

func downloadFile(client *stowry.Client, httpClient *http.Client, key string) ([]byte, error) {
	presignURL := client.PresignGet(key, 900)

	req, err := http.NewRequest(http.MethodGet, presignURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed: %s - %s", resp.Status, string(body))
	}

	return io.ReadAll(resp.Body)
}

func deleteFile(client *stowry.Client, httpClient *http.Client, key string) error {
	presignURL := client.PresignDelete(key, 900)

	req, err := http.NewRequest(http.MethodDelete, presignURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s - %s", resp.Status, string(body))
	}

	return nil
}
