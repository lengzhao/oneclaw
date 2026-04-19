package session

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/mediastore"
)

func TestInboundUserChunksForAttachments_imageDisabledBySwitch(t *testing.T) {
	cwd := t.TempDir()
	img := imagePNG(t)
	rel, err := mediastore.StoreBytes(cwd, "snap.png", img, maxMultimodalBytes)
	if err != nil {
		t.Fatal(err)
	}
	chunks := InboundUserChunksForAttachments(cwd, []Attachment{
		{Name: "snap.png", MIME: "image/png", Path: rel},
	}, false, true)
	if len(chunks) != 1 || len(chunks[0].MediaParts) != 0 {
		t.Fatalf("expected text-only chunk when image disabled, got %#v", chunks)
	}
	if chunks[0].Text == "" || !strings.Contains(chunks[0].Text, "read_file") {
		t.Fatalf("expected read_file hint: %q", chunks[0].Text)
	}
}

func TestInboundUserChunksForAttachments_imagePNG(t *testing.T) {
	cwd := t.TempDir()
	img := imagePNG(t)
	rel, err := mediastore.StoreBytes(cwd, "snap.png", img, maxMultimodalBytes)
	if err != nil {
		t.Fatal(err)
	}
	chunks := InboundUserChunksForAttachments(cwd, []Attachment{
		{Name: "snap.png", MIME: "image/png", Path: rel},
	}, true, true)
	if len(chunks) != 1 || len(chunks[0].MediaParts) != 1 {
		t.Fatalf("expected 1 multimodal chunk, got %#v", chunks)
	}
	p := chunks[0].MediaParts[0]
	if p.OfImageURL == nil || p.OfImageURL.ImageURL.URL == "" {
		t.Fatalf("expected image_url part")
	}
	if !strings.HasPrefix(p.OfImageURL.ImageURL.URL, "data:image/png;base64,") {
		prefixLen := 40
		if len(p.OfImageURL.ImageURL.URL) < prefixLen {
			prefixLen = len(p.OfImageURL.ImageURL.URL)
		}
		t.Fatalf("unexpected url prefix: %q", p.OfImageURL.ImageURL.URL[:prefixLen])
	}
}

func TestInboundUserChunksForAttachments_textStillHint(t *testing.T) {
	cwd := t.TempDir()
	chunks := InboundUserChunksForAttachments(cwd, []Attachment{
		{Name: "n.txt", MIME: "text/plain", Text: "hello"},
	}, true, true)
	if len(chunks) != 1 || chunks[0].Text == "" || len(chunks[0].MediaParts) != 0 {
		t.Fatalf("expected text-only chunk, got %#v", chunks)
	}
	if !strings.Contains(chunks[0].Text, "hello") {
		t.Fatalf("missing inline body: %q", chunks[0].Text)
	}
}

func TestInboundUserChunksForAttachments_wavByExt(t *testing.T) {
	cwd := t.TempDir()
	day := filepath.Join(cwd, "media", "inbound", "2099-01-01")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	rel := "media/inbound/2099-01-01/beep.wav"
	if err := os.WriteFile(filepath.Join(cwd, filepath.FromSlash(rel)), []byte("RIFF....WAVE"), 0o644); err != nil {
		t.Fatal(err)
	}
	chunks := InboundUserChunksForAttachments(cwd, []Attachment{
		{Name: "beep.wav", MIME: "application/octet-stream", Path: rel},
	}, true, true)
	if len(chunks) != 1 || len(chunks[0].MediaParts) != 1 {
		t.Fatalf("expected 1 audio chunk, got %#v", chunks)
	}
	if chunks[0].MediaParts[0].OfInputAudio == nil || chunks[0].MediaParts[0].OfInputAudio.InputAudio.Format != "wav" {
		t.Fatalf("expected wav input_audio")
	}
}

func imagePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
