package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/lengzhao/oneclaw/channel"
	"github.com/lengzhao/oneclaw/channel/gate"
	"github.com/lengzhao/oneclaw/channel/params"
	"github.com/lengzhao/oneclaw/mediastore"
	"github.com/lengzhao/oneclaw/routing"
)

const errCodeTenantTokenInvalid = 99991663

const RegistryName = "feishu"

const maxFeishuResourceBytes = 32 << 20

// Server implements channel.Connector via Feishu/Lark long connection.
type Server struct {
	appID      string
	appSecret  string
	verifyTok  string
	encryptKey string
	isLark     bool
	allowFrom  []string
	groupTrig  gate.GroupTrigger
	cwd        string
	client     *lark.Client
	ws         *larkws.Client
	tokenCache *tokenCache
	botOpenID  atomic.Value // string
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	wsCancel   context.CancelFunc
	workMu     sync.Mutex
	io         channel.IO // set in Run
}

type feishuWork struct {
	ctx           context.Context
	sessionKey    string
	userID        string
	tenantID      string
	correlationID string
	text          string
	attachments   []routing.Attachment
	chatID        string
}

// New builds a Feishu connector from YAML params.
func New(cfg channel.ConnectorConfig) (channel.Connector, error) {
	p := cfg.Params
	appID := params.String(p, "app_id")
	secret := params.String(p, "app_secret")
	if appID == "" || secret == "" {
		return nil, fmt.Errorf("feishu: app_id and app_secret are required")
	}
	cwd := ""
	if cfg.Engine != nil {
		cwd = cfg.Engine.CWD
	}
	gt := gate.GroupTrigger{
		MentionOnly: params.Bool(params.NestedMap(p, "group_trigger"), "mention_only"),
	}
	gt.Prefixes = params.StringSlice(params.NestedMap(p, "group_trigger"), "prefixes")
	tc := newTokenCache()
	opts := []lark.ClientOptionFunc{lark.WithTokenCache(tc)}
	if params.Bool(p, "is_lark") {
		opts = append(opts, lark.WithOpenBaseUrl(lark.LarkBaseUrl))
	}
	return &Server{
		appID:      appID,
		appSecret:  secret,
		verifyTok:  params.String(p, "verification_token"),
		encryptKey: params.String(p, "encrypt_key"),
		isLark:     params.Bool(p, "is_lark"),
		allowFrom:  params.StringSlice(p, "allow_from"),
		groupTrig:  gt,
		cwd:        cwd,
		tokenCache: tc,
		client:     lark.NewClient(appID, secret, opts...),
	}, nil
}

func (s *Server) Name() string { return RegistryName }

func (s *Server) Run(ctx context.Context, io channel.IO) error {
	if s.appID == "" || s.appSecret == "" {
		return fmt.Errorf("feishu: missing credentials")
	}
	s.io = io
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()

	if err := s.fetchBotOpenID(s.ctx); err != nil {
		slog.Warn("feishu.bot_open_id", "err", err)
	}

	dispatcher := larkdispatcher.NewEventDispatcher(s.verifyTok, s.encryptKey).
		OnP2MessageReceiveV1(s.handleMessageReceive)

	runCtx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.wsCancel = cancel
	domain := lark.FeishuBaseUrl
	if s.isLark {
		domain = lark.LarkBaseUrl
	}
	s.ws = larkws.NewClient(s.appID, s.appSecret, larkws.WithEventHandler(dispatcher), larkws.WithDomain(domain))
	ws := s.ws
	s.mu.Unlock()

	go func() {
		if err := ws.Start(runCtx); err != nil && runCtx.Err() == nil {
			slog.Error("feishu.ws", "err", err)
		}
	}()

	slog.Info("feishu.started", "lark", s.isLark)
	<-s.ctx.Done()

	s.mu.Lock()
	if s.wsCancel != nil {
		s.wsCancel()
		s.wsCancel = nil
	}
	s.ws = nil
	s.mu.Unlock()

	return nil
}

func (s *Server) handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	message := event.Event.Message
	sender := event.Event.Sender

	chatID := stringValue(message.ChatId)
	if chatID == "" {
		return nil
	}
	senderID := extractFeishuSenderID(sender)
	if senderID == "" {
		senderID = "unknown"
	}
	if !gate.IsAllowed(s.allowFrom, senderID) {
		return nil
	}

	messageType := stringValue(message.MessageType)
	messageID := stringValue(message.MessageId)
	rawContent := stringValue(message.Content)

	content := extractContent(messageType, rawContent)

	var atts []routing.Attachment
	if s.cwd != "" && messageID != "" {
		atts = s.downloadInboundMedia(s.ctx, chatID, messageID, messageType, rawContent)
	}

	if messageType == larkim.MsgTypeInteractive {
		_, externalURLs := extractCardImageKeys(rawContent)
		for _, u := range externalURLs {
			atts = append(atts, routing.Attachment{Name: "url", MIME: "text/uri-list", Text: u})
		}
	}

	content = appendMediaTags(content, messageType, atts)
	if strings.TrimSpace(content) == "" {
		content = "[empty message]"
	}

	chatType := stringValue(message.ChatType)
	tenantID := ""
	if sender != nil && sender.TenantKey != nil {
		tenantID = *sender.TenantKey
	}

	if chatType == "p2p" {
		// direct: group trigger N/A
	} else {
		isMentioned := s.isBotMentioned(message)
		if len(message.Mentions) > 0 {
			content = stripMentionPlaceholders(content, message.Mentions)
		}
		ok, cleaned := gate.ShouldRespondInGroup(s.groupTrig, isMentioned, content)
		if !ok {
			return nil
		}
		content = cleaned
	}

	slog.Info("feishu.inbound", "chat_id", chatID, "sender", senderID, "preview", truncateRunes(content, 80))

	w := feishuWork{
		ctx:           s.ctx,
		sessionKey:    chatID,
		userID:        senderID,
		tenantID:      tenantID,
		correlationID: "feishu-" + chatID + "-" + messageID,
		text:          content,
		attachments:   atts,
		chatID:        chatID,
	}
	go func() {
		s.workMu.Lock()
		defer s.workMu.Unlock()
		if err := s.runOneTurn(w); err != nil {
			slog.Error("feishu.turn", "err", err)
		}
	}()
	return nil
}

func (s *Server) runOneTurn(w feishuWork) error {
	io := s.io
	turnDone := make(chan error, 1)
	io.InboundChan <- channel.InboundTurn{
		Ctx:           w.ctx,
		Text:          w.text,
		Attachments:   w.attachments,
		SessionKey:    w.sessionKey,
		UserID:        w.userID,
		TenantID:      w.tenantID,
		CorrelationID: w.correlationID,
		Done:          turnDone,
	}

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case rec, ok := <-io.OutboundChan:
			if !ok {
				return nil
			}
			switch rec.Kind {
			case routing.KindText:
				if err := s.sendAssistantText(w.ctx, w.chatID, rec); err != nil {
					slog.Error("feishu.send", "err", err)
				}
			case routing.KindTool:
			case routing.KindDone:
				select {
				case err := <-turnDone:
					return err
				case <-s.ctx.Done():
					return s.ctx.Err()
				}
			}
		}
	}
}

func (s *Server) sendAssistantText(ctx context.Context, chatID string, rec routing.Record) error {
	text, _ := rec.Data["content"].(string)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	card, err := buildMarkdownCard(text)
	if err != nil {
		return s.sendText(ctx, chatID, text)
	}
	err = s.sendCard(ctx, chatID, card)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "11310") {
		return s.sendText(ctx, chatID, text)
	}
	return err
}

func (s *Server) fetchBotOpenID(ctx context.Context) error {
	resp, err := s.client.Do(ctx, &larkcore.ApiReq{
		HttpMethod:                http.MethodGet,
		ApiPath:                   "/open-apis/bot/v3/info",
		SupportedAccessTokenTypes: []larkcore.AccessTokenType{larkcore.AccessTokenTypeTenant},
	})
	if err != nil {
		return err
	}
	var result struct {
		Code int `json:"code"`
		Bot  struct {
			OpenID string `json:"open_id"`
		} `json:"bot"`
	}
	if err := json.Unmarshal(resp.RawBody, &result); err != nil {
		return err
	}
	if result.Code != 0 {
		s.invalidateTokenOnAuthError(result.Code)
		return fmt.Errorf("bot info code=%d", result.Code)
	}
	if result.Bot.OpenID == "" {
		return fmt.Errorf("empty open_id")
	}
	s.botOpenID.Store(result.Bot.OpenID)
	return nil
}

func (s *Server) isBotMentioned(message *larkim.EventMessage) bool {
	if message.Mentions == nil {
		return false
	}
	knownID, _ := s.botOpenID.Load().(string)
	if knownID == "" {
		return false
	}
	for _, m := range message.Mentions {
		if m.Id == nil || m.Id.OpenId == nil {
			continue
		}
		if *m.Id.OpenId == knownID {
			return true
		}
	}
	return false
}

func (s *Server) invalidateTokenOnAuthError(code int) {
	if code == errCodeTenantTokenInvalid {
		s.tokenCache.InvalidateAll()
		slog.Warn("feishu.token_invalidated")
	}
}

func (s *Server) sendCard(ctx context.Context, chatID, cardContent string) error {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeInteractive).
			Content(cardContent).
			Build()).
		Build()
	resp, err := s.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		s.invalidateTokenOnAuthError(resp.Code)
		return fmt.Errorf("feishu api %d: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (s *Server) sendText(ctx context.Context, chatID, text string) error {
	content, _ := json.Marshal(map[string]string{"text": text})
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeText).
			Content(string(content)).
			Build()).
		Build()
	resp, err := s.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu text api %d: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (s *Server) downloadInboundMedia(ctx context.Context, chatID, messageID, messageType, rawContent string) []routing.Attachment {
	if s.cwd == "" {
		return nil
	}
	var refs []routing.Attachment

	switch messageType {
	case larkim.MsgTypeImage:
		key := extractImageKey(rawContent)
		if key == "" {
			return nil
		}
		if a := s.downloadResource(ctx, messageID, key, "image", ".jpg"); a != nil {
			refs = append(refs, *a)
		}
	case larkim.MsgTypeInteractive:
		keys, _ := extractCardImageKeys(rawContent)
		for _, imageKey := range keys {
			if a := s.downloadResource(ctx, messageID, imageKey, "image", ".jpg"); a != nil {
				refs = append(refs, *a)
			}
		}
	case larkim.MsgTypeFile, larkim.MsgTypeAudio, larkim.MsgTypeMedia:
		key := extractFileKey(rawContent)
		if key == "" {
			return nil
		}
		var ext string
		switch messageType {
		case larkim.MsgTypeAudio:
			ext = ".ogg"
		case larkim.MsgTypeMedia:
			ext = ".mp4"
		}
		if a := s.downloadResource(ctx, messageID, key, "file", ext); a != nil {
			refs = append(refs, *a)
		}
	}
	return refs
}

func (s *Server) downloadResource(ctx context.Context, messageID, fileKey, resourceType, fallbackExt string) *routing.Attachment {
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(fileKey).
		Type(resourceType).
		Build()
	resp, err := s.client.Im.V1.MessageResource.Get(ctx, req)
	if err != nil {
		return nil
	}
	if !resp.Success() {
		s.invalidateTokenOnAuthError(resp.Code)
		return nil
	}
	if resp.File == nil {
		return nil
	}
	if closer, ok := resp.File.(io.Closer); ok {
		defer closer.Close()
	}
	filename := resp.FileName
	if filename == "" {
		filename = fileKey
	}
	if filepath.Ext(filename) == "" && fallbackExt != "" {
		filename += fallbackExt
	}
	rel, err := mediastore.StoreReader(s.cwd, filename, resp.File, maxFeishuResourceBytes)
	if err != nil {
		return nil
	}
	mime := "application/octet-stream"
	return &routing.Attachment{Name: filepath.Base(filename), MIME: mime, Path: rel}
}

// --- helpers from picoclaw/feishu/common.go ---

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func buildMarkdownCard(content string) (string, error) {
	card := map[string]any{
		"schema": "2.0",
		"body": map[string]any{
			"elements": []map[string]any{
				{"tag": "markdown", "content": content},
			},
		},
	}
	data, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func extractJSONStringField(content, field string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &m); err != nil {
		return ""
	}
	raw, ok := m[field]
	if !ok {
		return ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return ""
	}
	return str
}

func extractImageKey(content string) string { return extractJSONStringField(content, "image_key") }
func extractFileKey(content string) string  { return extractJSONStringField(content, "file_key") }
func extractFileName(content string) string { return extractJSONStringField(content, "file_name") }

func stripMentionPlaceholders(content string, mentions []*larkim.MentionEvent) string {
	if len(mentions) == 0 {
		return content
	}
	for _, m := range mentions {
		if m.Key != nil && *m.Key != "" {
			content = strings.ReplaceAll(content, *m.Key, "")
		}
	}
	content = strings.TrimSpace(content)
	return content
}

func extractCardImageKeys(rawContent string) (feishuKeys []string, externalURLs []string) {
	if rawContent == "" {
		return nil, nil
	}
	var card map[string]any
	if err := json.Unmarshal([]byte(rawContent), &card); err != nil {
		return nil, nil
	}
	extractImageKeysRecursive(card, &feishuKeys, &externalURLs)
	return feishuKeys, externalURLs
}

func isExternalURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func extractImageKeysRecursive(v any, feishuKeys, externalURLs *[]string) {
	switch val := v.(type) {
	case map[string]any:
		if tag, ok := val["tag"].(string); ok {
			switch tag {
			case "img":
				if imgKey, ok := val["img_key"].(string); ok && imgKey != "" {
					*feishuKeys = append(*feishuKeys, imgKey)
				}
				if src, ok := val["src"].(string); ok && src != "" {
					if isExternalURL(src) {
						*externalURLs = append(*externalURLs, src)
					} else {
						*feishuKeys = append(*feishuKeys, src)
					}
				}
			case "icon":
				if iconKey, ok := val["icon_key"].(string); ok && iconKey != "" {
					*feishuKeys = append(*feishuKeys, iconKey)
				}
			}
		}
		for _, child := range val {
			extractImageKeysRecursive(child, feishuKeys, externalURLs)
		}
	case []any:
		for _, item := range val {
			extractImageKeysRecursive(item, feishuKeys, externalURLs)
		}
	}
}

func extractContent(messageType, rawContent string) string {
	if rawContent == "" {
		return ""
	}
	switch messageType {
	case larkim.MsgTypeText:
		var textPayload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(rawContent), &textPayload); err == nil {
			return textPayload.Text
		}
		return rawContent
	case larkim.MsgTypePost, larkim.MsgTypeInteractive:
		return rawContent
	case larkim.MsgTypeImage:
		return ""
	case larkim.MsgTypeFile, larkim.MsgTypeAudio, larkim.MsgTypeMedia:
		name := extractFileName(rawContent)
		if name != "" {
			return name
		}
		return ""
	default:
		return rawContent
	}
}

func appendMediaTags(content, messageType string, media []routing.Attachment) string {
	if len(media) == 0 {
		return content
	}
	if messageType == larkim.MsgTypeInteractive {
		return content
	}
	var tag string
	switch messageType {
	case larkim.MsgTypeImage:
		tag = "[image: photo]"
	case larkim.MsgTypeAudio:
		tag = "[audio]"
	case larkim.MsgTypeMedia:
		tag = "[video]"
	case larkim.MsgTypeFile:
		tag = "[file]"
	default:
		tag = "[attachment]"
	}
	if content == "" {
		return tag
	}
	return content + " " + tag
}

func extractFeishuSenderID(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}
	if sender.SenderId.UserId != nil && *sender.SenderId.UserId != "" {
		return *sender.SenderId.UserId
	}
	if sender.SenderId.OpenId != nil && *sender.SenderId.OpenId != "" {
		return *sender.SenderId.OpenId
	}
	if sender.SenderId.UnionId != nil && *sender.SenderId.UnionId != "" {
		return *sender.SenderId.UnionId
	}
	return ""
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String() + "…"
}
