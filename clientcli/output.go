package clientcli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Formatter formats results for output.
type Formatter interface {
	FormatUpload(w io.Writer, results []UploadResult) error
	FormatDownload(w io.Writer, result *DownloadResult) error
	FormatDelete(w io.Writer, results []DeleteResult) error
	FormatList(w io.Writer, result *ListResult) error
	FormatError(w io.Writer, err error) error
}

// NewFormatter returns the appropriate formatter based on flags.
func NewFormatter(jsonOutput, quiet bool) Formatter {
	if jsonOutput {
		return &JSONFormatter{}
	}
	return &HumanFormatter{Quiet: quiet}
}

// HumanFormatter outputs human-readable text.
type HumanFormatter struct {
	Quiet bool
}

// FormatUpload formats upload results as human-readable text.
func (f *HumanFormatter) FormatUpload(w io.Writer, results []UploadResult) error {
	for i := range results {
		r := &results[i]
		if r.Err != nil {
			_, _ = fmt.Fprintf(w, "Error: %s - %v\n", r.LocalPath, r.Err)
			continue
		}
		if !f.Quiet {
			_, _ = fmt.Fprintf(w, "Uploaded: %s (%s)\n", r.RemotePath, formatSize(r.Size))
			_, _ = fmt.Fprintf(w, "  ETag: %s\n", r.ETag)
		}
	}
	return nil
}

// FormatDownload formats download result as human-readable text.
func (f *HumanFormatter) FormatDownload(w io.Writer, result *DownloadResult) error {
	if !f.Quiet {
		if result.LocalPath == "-" {
			_, _ = fmt.Fprintf(w, "Downloaded: %s (%s)\n", result.RemotePath, formatSize(result.Size))
		} else {
			_, _ = fmt.Fprintf(w, "Downloaded: %s -> %s (%s)\n", result.RemotePath, result.LocalPath, formatSize(result.Size))
		}
		_, _ = fmt.Fprintf(w, "  ETag: %s\n", result.ETag)
	}
	return nil
}

// FormatDelete formats delete results as human-readable text.
func (f *HumanFormatter) FormatDelete(w io.Writer, results []DeleteResult) error {
	for i := range results {
		r := &results[i]
		if r.Err != nil {
			_, _ = fmt.Fprintf(w, "Error: %s - %v\n", r.Path, r.Err)
			continue
		}
		if !f.Quiet {
			_, _ = fmt.Fprintf(w, "Deleted: %s\n", r.Path)
		}
	}
	return nil
}

// FormatList formats list results as human-readable text.
func (f *HumanFormatter) FormatList(w io.Writer, result *ListResult) error {
	if len(result.Items) == 0 {
		_, _ = fmt.Fprintln(w, "No objects found")
		return nil
	}

	// Calculate column widths
	maxPathLen := 4 // "PATH"
	for i := range result.Items {
		if len(result.Items[i].Path) > maxPathLen {
			maxPathLen = len(result.Items[i].Path)
		}
	}
	if maxPathLen > 60 {
		maxPathLen = 60
	}

	// Print header
	_, _ = fmt.Fprintf(w, "%-*s  %10s  %s\n", maxPathLen, "PATH", "SIZE", "UPDATED")
	_, _ = fmt.Fprintf(w, "%s  %s  %s\n", strings.Repeat("-", maxPathLen), strings.Repeat("-", 10), strings.Repeat("-", 19))

	// Print items
	for i := range result.Items {
		item := &result.Items[i]
		path := item.Path
		if len(path) > maxPathLen {
			path = path[:maxPathLen-3] + "..."
		}
		_, _ = fmt.Fprintf(w, "%-*s  %10s  %s\n",
			maxPathLen,
			path,
			formatSize(item.Size),
			item.UpdatedAt.Format("2006-01-02 15:04:05"),
		)
	}

	// Print summary
	_, _ = fmt.Fprintf(w, "\n%d object(s) (%s total)\n", len(result.Items), formatSize(result.TotalSize()))

	if result.NextCursor != "" {
		_, _ = fmt.Fprintf(w, "Next page: use --cursor %q\n", result.NextCursor)
	}

	return nil
}

// FormatError formats an error as human-readable text.
func (f *HumanFormatter) FormatError(w io.Writer, err error) error {
	_, _ = fmt.Fprintf(w, "Error: %v\n", err)
	return nil
}

// JSONFormatter outputs JSON.
type JSONFormatter struct{}

// FormatUpload formats upload results as JSON.
func (f *JSONFormatter) FormatUpload(w io.Writer, results []UploadResult) error {
	// Convert errors to strings for JSON output
	type jsonResult struct {
		LocalPath   string `json:"local_path"`
		RemotePath  string `json:"remote_path"`
		ID          string `json:"id,omitempty"`
		ContentType string `json:"content_type,omitempty"`
		ETag        string `json:"etag,omitempty"`
		Size        int64  `json:"size_bytes,omitempty"`
		CreatedAt   string `json:"created_at,omitempty"`
		UpdatedAt   string `json:"updated_at,omitempty"`
		Error       string `json:"error,omitempty"`
	}

	output := make([]jsonResult, len(results))
	for i := range results {
		r := &results[i]
		jr := jsonResult{
			LocalPath:  r.LocalPath,
			RemotePath: r.RemotePath,
		}
		if r.Err != nil {
			jr.Error = r.Err.Error()
		} else {
			jr.ID = r.ID.String()
			jr.ContentType = r.ContentType
			jr.ETag = r.ETag
			jr.Size = r.Size
			jr.CreatedAt = r.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			jr.UpdatedAt = r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		output[i] = jr
	}

	return writeJSON(w, output)
}

// FormatDownload formats download result as JSON.
func (f *JSONFormatter) FormatDownload(w io.Writer, result *DownloadResult) error {
	return writeJSON(w, result)
}

// FormatDelete formats delete results as JSON.
func (f *JSONFormatter) FormatDelete(w io.Writer, results []DeleteResult) error {
	// Convert errors to strings for JSON output
	type jsonResult struct {
		Path    string `json:"path"`
		Deleted bool   `json:"deleted"`
		Error   string `json:"error,omitempty"`
	}

	output := struct {
		Results []jsonResult `json:"results"`
	}{
		Results: make([]jsonResult, len(results)),
	}

	for i, r := range results {
		jr := jsonResult{
			Path:    r.Path,
			Deleted: r.Deleted,
		}
		if r.Err != nil {
			jr.Error = r.Err.Error()
		}
		output.Results[i] = jr
	}

	return writeJSON(w, output)
}

// FormatList formats list results as JSON.
func (f *JSONFormatter) FormatList(w io.Writer, result *ListResult) error {
	return writeJSON(w, result)
}

// FormatError formats an error as JSON.
func (f *JSONFormatter) FormatError(w io.Writer, err error) error {
	output := struct {
		Error string `json:"error"`
	}{
		Error: err.Error(),
	}
	return writeJSON(w, output)
}

// writeJSON writes a value as indented JSON.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// formatSize formats bytes as human-readable size.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
