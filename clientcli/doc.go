// Package clientcli provides a client library for interacting with Stowry object storage servers.
//
// It supports upload, download, delete, and list operations with AWS Signature V4 authentication.
// The package includes profile-based configuration for managing connections to multiple servers.
//
// # Basic Usage
//
// Create a client and upload a file:
//
//	cfg := &clientcli.Config{
//		Endpoint:  "http://localhost:5708",
//		AccessKey: "your-access-key",
//		SecretKey: "your-secret-key",
//	}
//
//	client, err := clientcli.New(cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	results, err := client.Upload(ctx, clientcli.UploadOptions{
//		LocalPath:  "./file.txt",
//		RemotePath: "documents/file.txt",
//	})
//
// # Profile Configuration
//
// Use profiles to manage multiple server configurations:
//
//	configFile, err := clientcli.LoadConfigFile("~/.stowry/config.yaml")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	profile, err := configFile.GetProfile("production")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	cfg := clientcli.ConfigFromProfile(profile)
//	client, err := clientcli.New(cfg)
//
// # Output Formatting
//
// Use formatters for human-readable or JSON output:
//
//	formatter := clientcli.NewFormatter(jsonOutput, quiet)
//	formatter.FormatUpload(os.Stdout, results)
package clientcli
