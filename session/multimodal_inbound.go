package session

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/loop"
)

// maxMultimodalBytes matches maxPersistAttachmentBytes (inbound store cap).
const maxMultimodalBytes = 4 << 20

// InboundUserChunksForAttachments builds user message chunks for the model: images and wav/mp3
// become native multimodal parts when allowed; other types keep read_file / inline text hints.
// allowImage / allowAudio correspond to YAML features.disable_multimodal_* inverted (see Engine).
func InboundUserChunksForAttachments(cwd string, atts []Attachment, allowImage, allowAudio bool) []loop.InboundUserChunk {
	if len(atts) == 0 {
		return nil
	}
	var chunks []loop.InboundUserChunk
	var pending []schema.MessageInputPart
	flush := func() {
		if len(pending) == 0 {
			return
		}
		chunks = append(chunks, loop.InboundUserChunk{MediaParts: pending})
		pending = nil
	}
	for _, a := range atts {
		parts, ok, err := tryMultimodalAttachment(cwd, a, allowImage, allowAudio)
		if err != nil {
			slog.Warn("session.multimodal.fallback", "name", a.Name, "err", err)
		}
		if ok && len(parts) > 0 {
			pending = append(pending, parts...)
			continue
		}
		flush()
		if s := formatInboundAttachmentUserText(a); strings.TrimSpace(s) != "" {
			chunks = append(chunks, loop.InboundUserChunk{Text: s})
		}
	}
	flush()
	return chunks
}

func tryMultimodalAttachment(cwd string, a Attachment, allowImage, allowAudio bool) ([]schema.MessageInputPart, bool, error) {
	rel := strings.TrimSpace(a.Path)
	if rel == "" {
		return nil, false, nil
	}
	abs := filepath.Join(cwd, filepath.FromSlash(rel))
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, false, err
	}
	if len(data) == 0 {
		return nil, false, fmt.Errorf("empty media file")
	}
	if len(data) > maxMultimodalBytes {
		return nil, false, fmt.Errorf("media exceeds %d bytes", maxMultimodalBytes)
	}
	mt := inferAttachmentMIME(a, rel)
	if isVisionImageMIME(mt) && !allowImage {
		return nil, false, nil
	}
	if isVisionImageMIME(mt) {
		dataURL := fmt.Sprintf("data:%s;base64,%s", visionDataMIME(mt), base64.StdEncoding.EncodeToString(data))
		url := dataURL
		return []schema.MessageInputPart{{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{URL: &url},
				Detail:            schema.ImageURLDetailAuto,
			},
		}}, true, nil
	}
	if isWAV(mt, rel) && !allowAudio {
		return nil, false, nil
	}
	if isWAV(mt, rel) {
		b64 := base64.StdEncoding.EncodeToString(data)
		mimeWav := "audio/wav"
		return []schema.MessageInputPart{{
			Type: schema.ChatMessagePartTypeAudioURL,
			Audio: &schema.MessageInputAudio{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: &b64,
					MIMEType:   mimeWav,
				},
			},
		}}, true, nil
	}
	if isMP3(mt, rel) && !allowAudio {
		return nil, false, nil
	}
	if isMP3(mt, rel) {
		b64 := base64.StdEncoding.EncodeToString(data)
		mimeMP3 := "audio/mpeg"
		return []schema.MessageInputPart{{
			Type: schema.ChatMessagePartTypeAudioURL,
			Audio: &schema.MessageInputAudio{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: &b64,
					MIMEType:   mimeMP3,
				},
			},
		}}, true, nil
	}
	return nil, false, nil
}

func inferAttachmentMIME(a Attachment, rel string) string {
	m := strings.TrimSpace(strings.ToLower(a.MIME))
	if m != "" && m != "application/octet-stream" {
		return m
	}
	ext := strings.ToLower(filepath.Ext(rel))
	if ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			return strings.ToLower(strings.TrimSpace(ct))
		}
	}
	return "application/octet-stream"
}

func isVisionImageMIME(mt string) bool {
	if !strings.HasPrefix(mt, "image/") {
		return false
	}
	// SVG / XML-heavy types: keep read_file path instead of image_url.
	switch mt {
	case "image/svg+xml", "image/svg", "image/heic", "image/heif":
		return false
	default:
		return true
	}
}

func visionDataMIME(mt string) string {
	switch mt {
	case "image/jpg":
		return "image/jpeg"
	default:
		return mt
	}
}

func isWAV(mt, rel string) bool {
	if strings.HasSuffix(strings.ToLower(rel), ".wav") {
		return true
	}
	switch mt {
	case "audio/wav", "audio/x-wav", "audio/wave", "audio/waveform":
		return true
	default:
		return false
	}
}

func isMP3(mt, rel string) bool {
	if strings.HasSuffix(strings.ToLower(rel), ".mp3") {
		return true
	}
	switch mt {
	case "audio/mpeg", "audio/mp3", "audio/x-mpeg":
		return true
	default:
		return false
	}
}
