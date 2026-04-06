package statichttp

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/mediastore"
)

// Limits for multipart uploads (per file, raw bytes before UTF-8 / base64 handling).
const maxMultipartFormBytes = 32 << 20 // entire form
const maxUploadedFileBytes = 4 << 20   // single file part

// form field names
const (
	formFieldText   = "text"
	formFieldLocale = "locale"
	formFieldFiles  = "files"
	formFieldFile   = "file"
)

func parseChatMultipart(cwd string, r *http.Request) (chatRequest, error) {
	var zero chatRequest
	if strings.TrimSpace(cwd) == "" {
		return zero, fmt.Errorf("multipart upload requires engine working directory (cwd)")
	}
	if err := r.ParseMultipartForm(maxMultipartFormBytes); err != nil {
		return zero, fmt.Errorf("parse multipart: %w", err)
	}
	defer func() {
		if f := r.MultipartForm; f != nil {
			_ = f.RemoveAll()
		}
	}()

	out := chatRequest{
		Text:   strings.TrimSpace(r.FormValue(formFieldText)),
		Locale: strings.TrimSpace(r.FormValue(formFieldLocale)),
	}
	if r.MultipartForm == nil {
		return out, nil
	}
	for _, key := range []string{formFieldFiles, formFieldFile} {
		for _, fh := range r.MultipartForm.File[key] {
			a, err := fileHeaderToAttachment(cwd, fh)
			if err != nil {
				return zero, err
			}
			out.Attachments = append(out.Attachments, a)
		}
	}
	return out, nil
}

func fileHeaderToAttachment(cwd string, fh *multipart.FileHeader) (chatAttachment, error) {
	var zero chatAttachment
	if fh == nil {
		return zero, fmt.Errorf("empty file header")
	}
	f, err := fh.Open()
	if err != nil {
		return zero, fmt.Errorf("open part: %w", err)
	}
	defer f.Close()

	body, err := readMultipartPart(f, maxUploadedFileBytes)
	if err != nil {
		return zero, err
	}

	mime := strings.TrimSpace(fh.Header.Get("Content-Type"))
	if mime == "" {
		mime = http.DetectContentType(body)
	}
	name := sanitizeMultipartFilename(fh.Filename)
	rel, err := mediastore.StoreBytes(cwd, name, body, maxUploadedFileBytes)
	if err != nil {
		return zero, err
	}
	return chatAttachment{Name: name, MIME: mime, Path: rel}, nil
}

func readMultipartPart(f multipart.File, maxBytes int) ([]byte, error) {
	var buf []byte
	chunk := make([]byte, 32*1024)
	for {
		n, err := f.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if len(buf) > maxBytes {
				return nil, fmt.Errorf("file exceeds %d bytes", maxBytes)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if len(buf) == 0 {
		return nil, fmt.Errorf("empty file")
	}
	return buf, nil
}

func sanitizeMultipartFilename(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == ".." {
		return "upload"
	}
	return base
}
