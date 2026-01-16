// Example: Using Stowry with AWS SDK for Go v2
//
// This example demonstrates using presigned URLs with Stowry.
// Stowry only supports query-string authentication (presigned URLs),
// not header-based authentication.
//
// Run Stowry first:
//
//	stowry serve --config ../../config.yaml
//
// Then run this example:
//
//	cd examples/aws/go
//	go mod tidy
//	go run main.go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gopkg.in/yaml.v3"
)

const (
	stowryEndpoint = "http://localhost:5708"
	configPath     = "../../config.yaml"
	bucket         = "example"
)

type exampleConfig struct {
	Auth struct {
		Region  string `yaml:"region"`
		Service string `yaml:"service"`
		Keys    []struct {
			AccessKey string `yaml:"access_key"`
			SecretKey string `yaml:"secret_key"`
		} `yaml:"keys"`
	} `yaml:"auth"`
}

func main() {
	ctx := context.Background()

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Auth.Keys) == 0 {
		log.Fatal("no auth keys found in config")
	}

	presignClient, err := newPresignClient(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Upload a file
	key := "hello.txt"
	content := []byte("Hello from AWS SDK for Go!")
	contentType := "text/plain"

	fmt.Println("=== Upload ===")
	err = uploadFile(ctx, presignClient, httpClient, key, content, contentType)
	if err != nil {
		log.Fatalf("upload failed: %v", err)
	}
	fmt.Printf("Uploaded: %s/%s\n", bucket, key)

	// Download the file
	fmt.Println("\n=== Download ===")
	downloaded, err := downloadFile(ctx, presignClient, httpClient, key)
	if err != nil {
		log.Fatalf("download failed: %v", err)
	}
	fmt.Printf("Content: %s\n", string(downloaded))

	// Generate presigned URLs
	fmt.Println("\n=== Presigned URLs ===")

	downloadURL, err := presignGetURL(ctx, presignClient, key, 15*time.Minute)
	if err != nil {
		log.Fatalf("presign get failed: %v", err)
	}
	fmt.Printf("GET URL: %s\n", downloadURL)

	uploadURL, err := presignPutURL(ctx, presignClient, "presigned-upload.txt", contentType, 15*time.Minute)
	if err != nil {
		log.Fatalf("presign put failed: %v", err)
	}
	fmt.Printf("PUT URL: %s\n", uploadURL)

	deleteURL, err := presignDeleteURL(ctx, presignClient, key, 15*time.Minute)
	if err != nil {
		log.Fatalf("presign delete failed: %v", err)
	}
	fmt.Printf("DELETE URL: %s\n", deleteURL)

	// Delete the file
	fmt.Println("\n=== Delete ===")
	err = deleteFile(ctx, presignClient, httpClient, key)
	if err != nil {
		log.Fatalf("delete failed: %v", err)
	}
	fmt.Printf("Deleted: %s/%s\n", bucket, key)
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

func newPresignClient(ctx context.Context, exCfg *exampleConfig) (*s3.PresignClient, error) {
	region := exCfg.Auth.Region
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			exCfg.Auth.Keys[0].AccessKey,
			exCfg.Auth.Keys[0].SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(stowryEndpoint)
		o.UsePathStyle = true
	})

	return s3.NewPresignClient(client), nil
}

func uploadFile(ctx context.Context, presignClient *s3.PresignClient, httpClient *http.Client, key string, content []byte, contentType string) error {
	presignURL, err := presignPutURL(ctx, presignClient, key, contentType, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("presign: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignURL, bytes.NewReader(content))
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

func downloadFile(ctx context.Context, presignClient *s3.PresignClient, httpClient *http.Client, key string) ([]byte, error) {
	presignURL, err := presignGetURL(ctx, presignClient, key, 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("presign: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignURL, nil)
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

func deleteFile(ctx context.Context, presignClient *s3.PresignClient, httpClient *http.Client, key string) error {
	presignURL, err := presignDeleteURL(ctx, presignClient, key, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("presign: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, presignURL, nil)
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

func presignGetURL(ctx context.Context, client *s3.PresignClient, key string, expires time.Duration) (string, error) {
	resp, err := client.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return resp.URL, nil
}

func presignPutURL(ctx context.Context, client *s3.PresignClient, key, contentType string, expires time.Duration) (string, error) {
	resp, err := client.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return resp.URL, nil
}

func presignDeleteURL(ctx context.Context, client *s3.PresignClient, key string, expires time.Duration) (string, error) {
	resp, err := client.PresignDeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return resp.URL, nil
}
