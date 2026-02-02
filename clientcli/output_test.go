package clientcli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sagarc03/stowry/clientcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFormatter(t *testing.T) {
	t.Run("json formatter", func(t *testing.T) {
		formatter := clientcli.NewFormatter(true, false)
		_, ok := formatter.(*clientcli.JSONFormatter)
		assert.True(t, ok)
	})

	t.Run("human formatter", func(t *testing.T) {
		formatter := clientcli.NewFormatter(false, false)
		_, ok := formatter.(*clientcli.HumanFormatter)
		assert.True(t, ok)
	})

	t.Run("human formatter quiet", func(t *testing.T) {
		formatter := clientcli.NewFormatter(false, true)
		hf, ok := formatter.(*clientcli.HumanFormatter)
		require.True(t, ok)
		assert.True(t, hf.Quiet)
	})
}

func TestHumanFormatter_FormatUpload(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		formatter := &clientcli.HumanFormatter{}
		results := []clientcli.UploadResult{
			{
				LocalPath:  "local.txt",
				RemotePath: "remote.txt",
				Size:       1024,
				ETag:       "abc123",
			},
		}

		var buf bytes.Buffer
		err := formatter.FormatUpload(&buf, results)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Uploaded: remote.txt")
		assert.Contains(t, output, "1.0 KB")
		assert.Contains(t, output, "ETag: abc123")
	})

	t.Run("with error", func(t *testing.T) {
		formatter := &clientcli.HumanFormatter{}
		results := []clientcli.UploadResult{
			{
				LocalPath: "local.txt",
				Err:       errors.New("upload failed"),
			},
		}

		var buf bytes.Buffer
		err := formatter.FormatUpload(&buf, results)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Error: local.txt - upload failed")
	})

	t.Run("quiet mode", func(t *testing.T) {
		formatter := &clientcli.HumanFormatter{Quiet: true}
		results := []clientcli.UploadResult{
			{
				LocalPath:  "local.txt",
				RemotePath: "remote.txt",
				Size:       1024,
			},
		}

		var buf bytes.Buffer
		err := formatter.FormatUpload(&buf, results)
		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})
}

func TestHumanFormatter_FormatDownload(t *testing.T) {
	formatter := &clientcli.HumanFormatter{}
	result := &clientcli.DownloadResult{
		RemotePath: "remote.txt",
		LocalPath:  "local.txt",
		Size:       2048,
		ETag:       "etag123",
	}

	var buf bytes.Buffer
	err := formatter.FormatDownload(&buf, result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Downloaded: remote.txt -> local.txt")
	assert.Contains(t, output, "2.0 KB")
	assert.Contains(t, output, "ETag: etag123")
}

func TestHumanFormatter_FormatDelete(t *testing.T) {
	formatter := &clientcli.HumanFormatter{}
	results := []clientcli.DeleteResult{
		{Path: "file1.txt", Deleted: true},
		{Path: "file2.txt", Deleted: false, Err: errors.New("not found")},
	}

	var buf bytes.Buffer
	err := formatter.FormatDelete(&buf, results)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Deleted: file1.txt")
	assert.Contains(t, output, "Error: file2.txt - not found")
}

func TestHumanFormatter_FormatList(t *testing.T) {
	t.Run("with items", func(t *testing.T) {
		formatter := &clientcli.HumanFormatter{}
		result := &clientcli.ListResult{
			Items: []clientcli.ObjectInfo{
				{
					Path:      "file1.txt",
					Size:      1024,
					UpdatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				},
				{
					Path:      "file2.txt",
					Size:      2048,
					UpdatedAt: time.Date(2024, 1, 14, 9, 15, 0, 0, time.UTC),
				},
			},
			NextCursor: "cursor123",
		}

		var buf bytes.Buffer
		err := formatter.FormatList(&buf, result)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "PATH")
		assert.Contains(t, output, "SIZE")
		assert.Contains(t, output, "UPDATED")
		assert.Contains(t, output, "file1.txt")
		assert.Contains(t, output, "file2.txt")
		assert.Contains(t, output, "2 object(s)")
		assert.Contains(t, output, "3.0 KB total")
		assert.Contains(t, output, `--cursor "cursor123"`)
	})

	t.Run("empty list", func(t *testing.T) {
		formatter := &clientcli.HumanFormatter{}
		result := &clientcli.ListResult{
			Items: []clientcli.ObjectInfo{},
		}

		var buf bytes.Buffer
		err := formatter.FormatList(&buf, result)
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "No objects found")
	})
}

func TestJSONFormatter_FormatUpload(t *testing.T) {
	formatter := &clientcli.JSONFormatter{}
	id := uuid.New()
	now := time.Now()

	results := []clientcli.UploadResult{
		{
			LocalPath:   "local.txt",
			RemotePath:  "remote.txt",
			ID:          id,
			ContentType: "text/plain",
			ETag:        "abc123",
			Size:        1024,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	var buf bytes.Buffer
	err := formatter.FormatUpload(&buf, results)
	require.NoError(t, err)

	var output []map[string]any
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Len(t, output, 1)
	assert.Equal(t, "local.txt", output[0]["local_path"])
	assert.Equal(t, "remote.txt", output[0]["remote_path"])
	assert.Equal(t, id.String(), output[0]["id"])
}

func TestJSONFormatter_FormatDelete(t *testing.T) {
	formatter := &clientcli.JSONFormatter{}
	results := []clientcli.DeleteResult{
		{Path: "file1.txt", Deleted: true},
		{Path: "file2.txt", Deleted: false, Err: errors.New("not found")},
	}

	var buf bytes.Buffer
	err := formatter.FormatDelete(&buf, results)
	require.NoError(t, err)

	var output map[string][]map[string]any
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Len(t, output["results"], 2)
	assert.Equal(t, "file1.txt", output["results"][0]["path"])
	assert.Equal(t, true, output["results"][0]["deleted"])
	assert.Equal(t, "file2.txt", output["results"][1]["path"])
	assert.Equal(t, false, output["results"][1]["deleted"])
	assert.Equal(t, "not found", output["results"][1]["error"])
}

func TestJSONFormatter_FormatError(t *testing.T) {
	formatter := &clientcli.JSONFormatter{}

	var buf bytes.Buffer
	err := formatter.FormatError(&buf, errors.New("test error"))
	require.NoError(t, err)

	var output map[string]string
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	assert.Equal(t, "test error", output["error"])
}

func TestHumanFormatter_FormatProfileList(t *testing.T) {
	formatter := &clientcli.HumanFormatter{}
	profiles := []clientcli.Profile{
		{Name: "local", Endpoint: "http://localhost:5708", AccessKey: "AKIAIOSFODNN7EXAMPLE"},
		{Name: "production", Endpoint: "https://prod.example.com", AccessKey: "PRODACCESSKEY123"},
	}

	t.Run("without secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.FormatProfileList(&buf, profiles, "local", false)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "NAME")
		assert.Contains(t, output, "ENDPOINT")
		assert.Contains(t, output, "ACCESS KEY")
		assert.Contains(t, output, "local")
		assert.Contains(t, output, "production")
		assert.Contains(t, output, "*")                       // default marker
		assert.Contains(t, output, "AKIA...MPLE")             // masked
		assert.NotContains(t, output, "AKIAIOSFODNN7EXAMPLE") // full key not shown
	})

	t.Run("with secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.FormatProfileList(&buf, profiles, "local", true)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "AKIAIOSFODNN7EXAMPLE") // full key shown
	})
}

func TestHumanFormatter_FormatProfileShow(t *testing.T) {
	formatter := &clientcli.HumanFormatter{}
	profile := clientcli.Profile{
		Name:      "production",
		Endpoint:  "https://prod.example.com",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	t.Run("without secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.FormatProfileShow(&buf, profile, true, false)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Name:       production (default)")
		assert.Contains(t, output, "Endpoint:   https://prod.example.com")
		assert.Contains(t, output, "Access Key: AKIA...MPLE")
		assert.Contains(t, output, "Secret Key: wJal...EKEY")
		assert.NotContains(t, output, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	})

	t.Run("with secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.FormatProfileShow(&buf, profile, false, true)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Name:       production")
		assert.NotContains(t, output, "(default)")
		assert.Contains(t, output, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	})
}

func TestJSONFormatter_FormatProfileList(t *testing.T) {
	formatter := &clientcli.JSONFormatter{}
	profiles := []clientcli.Profile{
		{Name: "local", Endpoint: "http://localhost:5708", AccessKey: "AKIAIOSFODNN7EXAMPLE", SecretKey: "secretkey123"},
		{Name: "production", Endpoint: "https://prod.example.com", AccessKey: "PRODKEY", SecretKey: "prodsecret"},
	}

	t.Run("without secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.FormatProfileList(&buf, profiles, "local", false)
		require.NoError(t, err)

		var output map[string][]map[string]any
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)

		assert.Len(t, output["profiles"], 2)
		assert.Equal(t, "local", output["profiles"][0]["name"])
		assert.Equal(t, true, output["profiles"][0]["default"])
		assert.Equal(t, "AKIA...MPLE", output["profiles"][0]["access_key"])
		assert.Equal(t, "secr...y123", output["profiles"][0]["secret_key"])
	})

	t.Run("with secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.FormatProfileList(&buf, profiles, "local", true)
		require.NoError(t, err)

		var output map[string][]map[string]any
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)

		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", output["profiles"][0]["access_key"])
		assert.Equal(t, "secretkey123", output["profiles"][0]["secret_key"])
	})
}

func TestJSONFormatter_FormatProfileShow(t *testing.T) {
	formatter := &clientcli.JSONFormatter{}
	profile := clientcli.Profile{
		Name:      "production",
		Endpoint:  "https://prod.example.com",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "secretkey123456789",
	}

	t.Run("without secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.FormatProfileShow(&buf, profile, true, false)
		require.NoError(t, err)

		var output map[string]any
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)

		assert.Equal(t, "production", output["name"])
		assert.Equal(t, "https://prod.example.com", output["endpoint"])
		assert.Equal(t, true, output["default"])
		assert.Equal(t, "AKIA...MPLE", output["access_key"])
		assert.Equal(t, "secr...6789", output["secret_key"])
	})

	t.Run("with secrets", func(t *testing.T) {
		var buf bytes.Buffer
		err := formatter.FormatProfileShow(&buf, profile, false, true)
		require.NoError(t, err)

		var output map[string]any
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)

		assert.Equal(t, false, output["default"])
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", output["access_key"])
		assert.Equal(t, "secretkey123456789", output["secret_key"])
	})
}
