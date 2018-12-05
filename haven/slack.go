package haven

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/k-saka/lvlogger"
)

const (
	// ReconnectInterval is try to connect slack rtm api interval
	ReconnectInterval = time.Second * 10
)

var logger *lvlogger.LvLogger

// SetLogger set logger used haven package used
func SetLogger(log *lvlogger.LvLogger) {
	logger = log
}

// relayGroup represents relaying channel group
type relayGroup map[string]channel

// hasChannel tests a channel exists in RelayGroup
func (g relayGroup) hasChannel(cID string) bool {
	_, ok := g[cID]
	return ok
}

// hasUser tests a user exists in RelayGroup
func (g relayGroup) hasUser(uID string) bool {
	for _, ch := range g {
		for _, m := range ch.Members {
			if m == uID {
				return true
			}
		}
	}
	return false
}

// userIDs return all user ids under group
func (g relayGroup) userIDs() []string {
	members := []string{}
	for _, ch := range g {
		members = append(members, ch.Members...)
	}
	return members
}

// channelCount count up channels in RelayGroups
func (g relayGroup) channelCount() int {
	return len(g)
}

// determineRelayChannels determine relay channels
func (g relayGroup) determineRelayChannels(cid string) []string {
	toRelay := []string{}

	if !g.hasChannel(cid) {
		return nil
	}

	for _, channel := range g {
		if channel.ID != cid {
			toRelay = append(toRelay, channel.ID)
		}
	}

	if len(toRelay) < 1 {
		return nil
	}

	return toRelay
}

// determineRelayChannelsMulti determine relay channels by channel ids
func (g relayGroup) determineRelayChannelsMulti(cids []string) []string {
	fromCids := map[string]struct{}{}
	for _, cid := range cids {
		fromCids[cid] = struct{}{}
	}

	toRelay := map[string]struct{}{}
	for _, cid := range cids {
		if toCids := g.determineRelayChannels(cid); toCids != nil {
			for _, toCid := range toCids {
				//// check already shared channel
				//if _, ok := fromCids[toCid]; ok {
				//	continue
				//}
				toRelay[toCid] = struct{}{}
			}
		}
	}

	// convert map to []string
	if len(toRelay) > 0 {
		toRelayCids := []string{}
		for cid := range toRelay {
			toRelayCids = append(toRelayCids, cid)
		}
		return toRelayCids
	}

	return nil
}

// newRelayGroup create RelayGroup from config
func newRelayGroup(config *Config, channels []channel) relayGroup {
	group := relayGroup{}
	for _, channel := range channels {
		if _, ok := config.RelayRooms[channel.ID]; ok {
			group[channel.ID] = channel
		}
	}
	return group
}

// RelayBot relay multiple channels
// Supported events are chat, file and shared message.
type RelayBot struct {
	url        string
	ws         *WsClient
	config     *Config
	messageLog *messageLog
	relayGroup relayGroup
	users      map[string]user
	hubUser    self
}

// NewRelayBot create RelayBot
func NewRelayBot(config *Config) *RelayBot {
	return &RelayBot{
		config:     config,
		ws:         NewWsClient(),
		messageLog: newMessageLog(100),
	}
}

func (b *RelayBot) postMembersInfo(cID string) {
	buf := bytes.Buffer{}
	tw := tabwriter.NewWriter(&buf, 0, 8, 0, '\t', 0)
	buf.WriteString("```")
	buf.WriteString("Haven members\n")
	for _, ch := range b.relayGroup {
		for _, uid := range ch.Members {
			user, ok := b.users[uid]
			if !ok {
				continue
			}
			fmt.Fprintf(tw, "Account:%s\tName:%s\n", user.Name, user.Profile.FullName())
		}
	}
	tw.Flush()
	buf.WriteString("```")

	pm := postMessageRequest{
		Channel:   cID,
		Text:      buf.String(),
		LinkNames: 0,
		UserName:  "Slack haven",
	}
	_, err := postMessage(b.config.Token, pm)
	if err != nil {
		logger.Warnf("%v", err)
	}
}

func (b *RelayBot) postBotStatus(cID string) {
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	buf := bytes.Buffer{}
	tw := tabwriter.NewWriter(&buf, 0, 8, 0, '\t', 0)
	buf.WriteString("```\n")
	fmt.Fprintf(tw, "Haven status\n")
	fmt.Fprintf(tw, "Goroutine count\t%v\n", runtime.NumGoroutine())
	fmt.Fprintf(tw, "Total allock\t%v\n", mem.TotalAlloc)
	tw.Flush()
	buf.WriteString("```\n")
	pm := postMessageRequest{
		Channel:   cID,
		Text:      buf.String(),
		LinkNames: 0,
		UserName:  "Slack haven",
	}
	_, err := postMessage(b.config.Token, pm)
	if err != nil {
		logger.Warnf("%v", err)
	}
}

func (b *RelayBot) handleSystemMessage(msg *message) {
	text := strings.ToLower(msg.Text)
	if strings.Contains(text, "members") {
		b.postMembersInfo(msg.Channel)
		return
	}
	if strings.Contains(text, "status") {
		b.postBotStatus(msg.Channel)
		return
	}
}

func (b *RelayBot) relayMessage(originID string, pm postMessageRequest) {
	resp, err := postMessage(b.config.Token, pm)
	if err != nil {
		logger.Warnf("%v", err)
		return
	}
	if !resp.Ok {
		logger.Warnf("PostMessage error, %s", resp.Error)
		return
	}
	// message log
	b.messageLog.add(pm.Channel, resp.Ts, originID)
	logger.Debugf("relayed message %v", pm)
}

// Handle receive message
func (b *RelayBot) handleMessage(msg *message) {
	// for debugging
	if b.relayGroup.hasChannel(msg.Channel) {
		logger.Infof("under haven message: %#v", msg)
	}

	if msg.ReplyTo.String() != "" {
		return
	}

	if msg.SubType == "bot_message" {
		return
	}

	if msg.SubType == "file_share" && strings.Contains(msg.Text, "botupload-") {
		return
	}

	if strings.HasPrefix(strings.ToLower(msg.Text), "haven") {
		b.handleSystemMessage(msg)
		return
	}

	relayTo := b.relayGroup.determineRelayChannels(msg.Channel)
	if relayTo == nil {
		return
	}
	logger.Infof("to relay message %+v", *msg)

	sender, ok := b.users[msg.User]
	if !ok {
		logger.Warnf("User outdated. %+v", msg)
		return
	}
	// Add message log as origin
	b.messageLog.add(msg.Channel, msg.Ts, msg.Ts)

	uname := sender.Profile.RealName
	if uname == "" {
		uname = sender.Name
	}

	pm := postMessageRequest{
		Text:        msg.Text,
		UserName:    uname,
		UnfurlLinks: true,
		UnfurlMedia: true,
		AsUser:      false,
		IconURL:     sender.Profile.Image512,
		Attachments: msg.Attachments,
	}

	for _, channel := range relayTo {
		pm.Channel = channel
		go b.relayMessage(msg.Ts, pm)
	}
}

func (b *RelayBot) handleMessageChanged(ev *messageChanged) {
	// for debugging
	if b.relayGroup.hasChannel(ev.Message.Channel) {
		logger.Infof("under haven message changed: %#v", ev)
	}
}

// Handle file shared event
func (b *RelayBot) handleFileShared(ev *fileShared) {
	if !b.relayGroup.hasUser(ev.UserID) {
		return
	}

	file, err := fetchFileInfo(b.config.Token, ev.FileID)
	if err != nil {
		logger.Warnf("%v", err)
		return
	}

	if strings.HasPrefix(file.Name, "botupload-") {
		return
	}

	shared := append(file.Channels, file.Groups...)
	shared = append(shared, file.IMS...)

	relayTo := b.relayGroup.determineRelayChannelsMulti(shared)
	if relayTo == nil {
		return
	}

	logger.Infof("to handle file %v", *ev)

	if _, ok := b.users[file.User]; !ok {
		logger.Warnf("User outdated. %+v", file)
		return
	}

	fileContent, err := downloadFile(b.config.Token, file.URLPrivate)
	if err != nil {
		logger.Warnf("%s", err)
		return
	}

	err = uploadFile(b.config.Token, relayTo, fileContent, file)
	if err != nil {
		logger.Warnf("%s", err)
		return
	}
}

func (b *RelayBot) handleReactionAdded(ev *reactionAdded) {
	// skip reaction posted by this bot
	if ev.User == b.hubUser.ID {
		return
	}

	relayTo := b.relayGroup.determineRelayChannels(ev.Item.Channel)
	if relayTo == nil {
		return
	}

	// supports only message
	if ev.Item.Type != "message" {
		return
	}

	messageMap := b.messageLog.getMessageMap(ev.Item.Channel, ev.Item.Ts)
	if messageMap == nil {
		return
	}

	requestPayload := reactionAddRequest{Name: ev.Reaction}
	for _, relayChannelID := range relayTo {
		if _, ok := messageMap[relayChannelID]; !ok {
			continue
		}
		requestPayload.Channel = relayChannelID
		requestPayload.Timestamp = messageMap[relayChannelID]
		_, err := addReaction(b.config.Token, requestPayload)
		if err != nil {
			logger.Warnf("cant send reaction: %v", err)
		}
	}
}

// Handle receive event
func (b *RelayBot) handleEvent(ev *anyEvent) {
	switch ev.Type {
	case "message":
		logger.Debugf("message recieved %v", string(ev.jsonMsg))
		// message changed event
		if ev.SubType == "message_changed" {
			var msgChangedEvent messageChanged
			if err := json.Unmarshal(ev.jsonMsg, &msgChangedEvent); err != nil {
				logger.Warnf("%v", err)
				return
			}
			b.handleMessageChanged(&msgChangedEvent)
			return
		}
		var msgEv message
		if err := json.Unmarshal(ev.jsonMsg, &msgEv); err != nil {
			logger.Warnf("%v", err)
			return
		}
		b.handleMessage(&msgEv)
	case "file_shared":
		logger.Debugf("file recieved %v", string(ev.jsonMsg))
		var fileEv fileShared
		if err := json.Unmarshal(ev.jsonMsg, &fileEv); err != nil {
			logger.Warnf("%v", err)
			return
		}
		b.handleFileShared(&fileEv)
	case "reaction_added":
		logger.Debugf("reaction received %v", string(ev.jsonMsg))
		var reactionAddEv reactionAdded
		if err := json.Unmarshal(ev.jsonMsg, &reactionAddEv); err != nil {
			logger.Warnf("%v", err)
			return
		}
		b.handleReactionAdded(&reactionAddEv)
	case "pong":
		logger.Debugf("pong received %v", string(ev.jsonMsg))
	default:
		// logger.Debugf("unhandled event %v %v", ev.EventType, string(ev.jsonMsg))
	}
}

// setUsers set user list under bot control
func (b *RelayBot) setUsers(users []user) {
	b.users = make(map[string]user, len(users))
	for _, u := range users {
		b.users[u.ID] = u
	}
}

func (b *RelayBot) _connect() error {
	logger.Info("Call start api")
	res, err := startAPI(b.config.Token)
	if err != nil {
		return err
	}
	b.url = res.URL
	all := append(res.Channels, res.Groups...)
	b.relayGroup = newRelayGroup(b.config, all)
	b.setUsers(res.Users)
	b.hubUser = res.Self
	logger.Info("Connect ws")
	err = b.ws.Connect(b.url)
	if err != nil {
		return err
	}
	return nil
}

// connect to slack websocket.
// Try until connection establish
func (b *RelayBot) connect() {
	if err := b._connect(); err != nil {
		logger.Warnf("%v", err)
	} else {
		return
	}

	// retry loop
	t := time.NewTicker(ReconnectInterval)
	defer t.Stop()
	for range t.C {
		if err := b._connect(); err != nil {
			logger.Warnf("%v", err)
			continue
		}
		return
	}
}

// Start relay bot
func (b *RelayBot) Start() {
	logger.Info("Relay bot start")
	b.connect()
	defer b.ws.Close()

	for {
		select {
		case ev := <-b.ws.Receive:
			var e anyEvent
			if err := json.Unmarshal(ev, &e); err != nil {
				logger.Warnf("%v", err)
				continue
			}
			e.jsonMsg = json.RawMessage(ev)
			b.handleEvent(&e)
		case err := <-b.ws.Disconnect:
			logger.Errorf("Disconnected. Cause %v", err)
			b.connect()
		}
	}
}
