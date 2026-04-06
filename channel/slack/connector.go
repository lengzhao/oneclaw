package slack

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/lengzhao/oneclaw/channel"
	"github.com/lengzhao/oneclaw/mediastore"
	"github.com/lengzhao/oneclaw/channel/gate"
	"github.com/lengzhao/oneclaw/channel/params"
	"github.com/lengzhao/oneclaw/routing"
)

// RegistryName is YAML channels[].type for this connector.
const RegistryName = "slack"

const maxSlackDownloadBytes = 32 << 20

// Server implements channel.Connector using Slack Socket Mode.
type Server struct {
	botToken   string
	appToken   string
	allowFrom  []string
	groupTrig  gate.GroupTrigger
	api    *slack.Client
	socket *socketmode.Client
	cwd    string

	botUserID string
	teamID    string

	ctx    context.Context
	cancel context.CancelFunc

	pendingAcks sync.Map // chat composite key -> slackMessageRef

	workMu sync.Mutex // one turn at a time per connector instance (shared outCh)
}

type slackMessageRef struct {
	ChannelID string
	Timestamp string
}

type slackWork struct {
	ctx           context.Context
	sessionKey    string
	userID        string
	tenantID      string
	correlationID string
	text          string
	attachments   []routing.Attachment
	postTarget postTarget
	ackKey     string // key for pendingAcks (same as when stored)
}

type postTarget struct {
	channelID string
	threadTS  string
}

// New builds a Slack connector from ConnectorConfig.Params.
func New(cfg channel.ConnectorConfig) (channel.Connector, error) {
	p := cfg.Params
	bot := params.String(p, "bot_token")
	app := params.String(p, "app_token")
	if bot == "" || app == "" {
		return nil, fmt.Errorf("slack: bot_token and app_token are required")
	}
	cwd := ""
	if cfg.Engine != nil {
		cwd = cfg.Engine.CWD
	}
	gt := gate.GroupTrigger{
		MentionOnly: params.Bool(params.NestedMap(p, "group_trigger"), "mention_only"),
	}
	gt.Prefixes = params.StringSlice(params.NestedMap(p, "group_trigger"), "prefixes")
	apiClient := slack.New(bot, slack.OptionAppLevelToken(app))
	return &Server{
		botToken:  bot,
		appToken:  app,
		allowFrom: params.StringSlice(p, "allow_from"),
		groupTrig: gt,
		api:    apiClient,
		socket: socketmode.New(apiClient),
		cwd:    cwd,
	}, nil
}

func (s *Server) Name() string { return RegistryName }

// Run blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context, io channel.IO) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()

	authResp, err := s.api.AuthTest()
	if err != nil {
		return fmt.Errorf("slack auth test: %w", err)
	}
	s.botUserID = authResp.UserID
	s.teamID = authResp.TeamID
	slog.Info("slack.connected", "bot_user_id", s.botUserID, "team", authResp.Team)

	go func() {
		if err := s.socket.RunContext(s.ctx); err != nil && s.ctx.Err() == nil {
			slog.Error("slack.socket", "err", err)
		}
	}()

	go s.eventLoop(io)

	<-s.ctx.Done()
	return nil
}

func (s *Server) eventLoop(io channel.IO) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case ev, ok := <-s.socket.Events:
			if !ok {
				return
			}
			switch ev.Type {
			case socketmode.EventTypeEventsAPI:
				s.handleEventsAPI(ev, io)
			case socketmode.EventTypeSlashCommand:
				s.handleSlashCommand(ev, io)
			case socketmode.EventTypeInteractive:
				if ev.Request != nil {
					s.socket.Ack(*ev.Request)
				}
			}
		}
	}
}

func (s *Server) handleEventsAPI(ev socketmode.Event, io channel.IO) {
	if ev.Request != nil {
		s.socket.Ack(*ev.Request)
	}
	eventsAPIEvent, ok := ev.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}
	switch inner := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		s.handleMessageEvent(inner, io)
	case *slackevents.AppMentionEvent:
		s.handleAppMention(inner, io)
	}
}

func (s *Server) handleMessageEvent(ev *slackevents.MessageEvent, io channel.IO) {
	if ev.User == s.botUserID || ev.User == "" {
		return
	}
	if ev.BotID != "" {
		return
	}
	if ev.SubType != "" && ev.SubType != "file_share" {
		return
	}
	if !gate.IsAllowed(s.allowFrom, ev.User) {
		slog.Debug("slack.allowlist.reject", "user_id", ev.User)
		return
	}

	senderID := ev.User
	channelID := ev.Channel
	threadTS := ev.ThreadTimeStamp
	messageTS := ev.TimeStamp

	chatKey := channelID
	if threadTS != "" {
		chatKey = channelID + "/" + threadTS
	}

	s.pendingAcks.Store(chatKey, slackMessageRef{ChannelID: channelID, Timestamp: messageTS})

	content := ev.Text
	content = s.stripBotMention(content)

	if !strings.HasPrefix(channelID, "D") {
		ok, cleaned := gate.ShouldRespondInGroup(s.groupTrig, false, content)
		if !ok {
			return
		}
		content = cleaned
	}

	var atts []routing.Attachment
	if s.cwd != "" && ev.Message != nil && len(ev.Message.Files) > 0 {
		for _, f := range ev.Message.Files {
			if a := s.downloadSlackFile(f); a != nil {
				atts = append(atts, *a)
				content += fmt.Sprintf("\n[file: %s]", f.Name)
			}
		}
	}

	if strings.TrimSpace(content) == "" {
		return
	}

	s.enqueueTurn(io, slackWork{
		ctx:           s.ctx,
		sessionKey:    chatKey,
		userID:        senderID,
		tenantID:      s.teamID,
		correlationID: "slack-" + channelID + "-" + messageTS,
		text:          strings.TrimSpace(content),
		attachments:   atts,
		postTarget:    postTarget{channelID: channelID, threadTS: threadTS},
		ackKey:        chatKey,
	})
}

func (s *Server) handleAppMention(ev *slackevents.AppMentionEvent, io channel.IO) {
	if ev.User == s.botUserID {
		return
	}
	if !gate.IsAllowed(s.allowFrom, ev.User) {
		return
	}
	senderID := ev.User
	channelID := ev.Channel
	threadTS := ev.ThreadTimeStamp
	messageTS := ev.TimeStamp

	var chatKey string
	if threadTS != "" {
		chatKey = channelID + "/" + threadTS
	} else {
		chatKey = channelID + "/" + messageTS
	}

	s.pendingAcks.Store(chatKey, slackMessageRef{ChannelID: channelID, Timestamp: messageTS})

	content := strings.TrimSpace(s.stripBotMention(ev.Text))
	if content == "" {
		return
	}

	s.enqueueTurn(io, slackWork{
		ctx:           s.ctx,
		sessionKey:    chatKey,
		userID:        senderID,
		tenantID:      s.teamID,
		correlationID: "slack-mention-" + channelID + "-" + messageTS,
		text:          content,
		postTarget:    postTarget{channelID: channelID, threadTS: threadTS},
		ackKey:        chatKey,
	})
}

func (s *Server) handleSlashCommand(ev socketmode.Event, io channel.IO) {
	cmd, ok := ev.Data.(slack.SlashCommand)
	if !ok {
		return
	}
	if ev.Request != nil {
		s.socket.Ack(*ev.Request)
	}
	if !gate.IsAllowed(s.allowFrom, cmd.UserID) {
		return
	}
	text := strings.TrimSpace(cmd.Text)
	if text == "" {
		text = "help"
	}
	s.enqueueTurn(io, slackWork{
		ctx:           s.ctx,
		sessionKey:    cmd.ChannelID,
		userID:        cmd.UserID,
		tenantID:      s.teamID,
		correlationID: "slack-cmd-" + cmd.TriggerID,
		text:          text,
		postTarget:    postTarget{channelID: cmd.ChannelID, threadTS: ""},
		ackKey:        cmd.ChannelID,
	})
}

func (s *Server) enqueueTurn(io channel.IO, w slackWork) {
	go func() {
		s.workMu.Lock()
		defer s.workMu.Unlock()
		if err := s.runOneTurn(io, w); err != nil {
			slog.Error("slack.turn", "err", err)
		}
	}()
}

func (s *Server) runOneTurn(io channel.IO, w slackWork) error {
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
				s.postText(rec, w.postTarget, w.ackKey)
			case routing.KindTool:
			case routing.KindDone:
				okFlag, _ := rec.Data["ok"].(bool)
				if !okFlag {
					msg, _ := rec.Data["error"].(string)
					slog.Warn("slack.turn.done_fail", "error", msg)
				}
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

func (s *Server) postText(rec routing.Record, target postTarget, ackKey string) {
	content, _ := rec.Data["content"].(string)
	if strings.TrimSpace(content) == "" {
		return
	}
	opts := []slack.MsgOption{slack.MsgOptionText(content, false)}
	if target.threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(target.threadTS))
	}
	_, _, err := s.api.PostMessageContext(s.ctx, target.channelID, opts...)
	if err != nil {
		slog.Error("slack.post", "err", err)
		return
	}
	if ref, ok := s.pendingAcks.LoadAndDelete(ackKey); ok {
		msgRef := ref.(slackMessageRef)
		_ = s.api.AddReaction("white_check_mark", slack.ItemRef{
			Channel:   msgRef.ChannelID,
			Timestamp: msgRef.Timestamp,
		})
	}
}

func (s *Server) stripBotMention(text string) string {
	mention := fmt.Sprintf("<@%s>", s.botUserID)
	text = strings.ReplaceAll(text, mention, "")
	return strings.TrimSpace(text)
}

func (s *Server) downloadSlackFile(file slack.File) *routing.Attachment {
	if s.cwd == "" {
		return nil
	}
	url := file.URLPrivateDownload
	if url == "" {
		url = file.URLPrivate
	}
	if url == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+s.botToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("slack.download", "err", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	name := file.Name
	if strings.TrimSpace(name) == "" {
		name = "file"
	}
	rel, err := mediastore.StoreReader(s.cwd, name, resp.Body, maxSlackDownloadBytes)
	if err != nil {
		slog.Error("slack.mediastore", "err", err)
		return nil
	}
	mime := file.Mimetype
	if mime == "" {
		mime = "application/octet-stream"
	}
	return &routing.Attachment{Name: name, MIME: mime, Path: rel}
}
