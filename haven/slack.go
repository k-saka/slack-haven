package haven

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alexcesaro/log"
)

const (
	// ReconnectInterval is try to connect slack rtm api interval
	ReconnectInterval = time.Second * 10
)

var logger log.Logger

// SetLogger set logger used haven package used
func SetLogger(log log.Logger) {
	logger = log
}

// RelayGroup represents relaying channel group
type RelayGroup map[string]Channel

// HasChannel tests a channel exists in RelayGroup
func (g RelayGroup) HasChannel(cID string) bool {
	_, ok := g[cID]
	return ok
}

// HasUser tests a user exists in RelayGroup
func (g RelayGroup) HasUser(uID string) bool {
	for _, ch := range g {
		for _, m := range ch.Members {
			if m == uID {
				return true
			}
		}
	}
	return false
}

// UserIDs return all user ids under group
func (g RelayGroup) UserIDs() []string {
	members := []string{}
	for _, ch := range g {
		members = append(members, ch.Members...)
	}
	return members
}

// ChannelCount count up channels in RelayGroups
func (g RelayGroup) ChannelCount() int {
	return len(g)
}

// DetermineRelayChannels determine relay channels
func (g RelayGroup) DetermineRelayChannels(cid string) []string {
	toRelay := []string{}

	if !g.HasChannel(cid) {
		return nil
	}

	for _, channel := range g {
		if channel.Id != cid {
			toRelay = append(toRelay, channel.Id)
		}
	}

	if len(toRelay) < 1 {
		return nil
	}

	return toRelay
}

// DetermineRelayChannelsMulti determine relay channels by channel ids
func (g RelayGroup) DetermineRelayChannelsMulti(cids []string) []string {
	fromCids := map[string]struct{}{}
	for _, cid := range cids {
		fromCids[cid] = struct{}{}
	}

	toRelay := map[string]struct{}{}
	for _, cid := range cids {
		if toCids := g.DetermineRelayChannels(cid); toCids != nil {
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

// NewRelayGroup create RelayGroup from config
func NewRelayGroup(config *Config, channels []Channel) RelayGroup {
	group := RelayGroup{}
	for _, channel := range channels {
		if _, ok := config.RelayRooms[channel.Id]; ok {
			group[channel.Id] = channel
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
	relayGroup RelayGroup
	users      map[string]User
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

	pm := PostMessage{
		Channel:   cID,
		Text:      buf.String(),
		LinkNames: 0,
		UserName:  "Slack haven",
	}
	_, err := postMessage(b.config.Token, pm)
	if err != nil {
		logger.Warningf("%v", err)
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
	pm := PostMessage{
		Channel:   cID,
		Text:      buf.String(),
		LinkNames: 0,
		UserName:  "Slack haven",
	}
	_, err := postMessage(b.config.Token, pm)
	if err != nil {
		logger.Warningf("%v", err)
	}
}

func (b *RelayBot) handleSystemMessage(msg *Message) {
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

func (b *RelayBot) relayMessage(originID string, pm PostMessage) {
	resp, err := postMessage(b.config.Token, pm)
	if err != nil {
		logger.Warningf("%v", err)
		return
	}
	if !resp.Ok {
		logger.Warningf("PostMessage error, %s", resp.Error)
		return
	}
	// message log
	b.messageLog.add(pm.Channel, resp.Ts, originID)
	logger.Debugf("relayed message %v", pm)
}

// Handle receive message
func (b *RelayBot) handleMessage(msg *Message) {
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

	relayTo := b.relayGroup.DetermineRelayChannels(msg.Channel)
	if relayTo == nil {
		return
	}
	logger.Infof("to relay message %+v", *msg)

	sender, ok := b.users[msg.User]
	if !ok {
		logger.Warningf("User outdated. %+v", msg)
		return
	}
	// Add message log as origin
	b.messageLog.add(msg.Channel, msg.Ts, msg.Ts)

	uname := sender.Profile.RealName
	if uname == "" {
		uname = sender.Name
	}

	pm := PostMessage{
		Text:        msg.Text,
		UserName:    uname,
		UnfurlLinks: true,
		UnfurlMedia: true,
		AsUser:      false,
		IconUrl:     sender.Profile.Image512,
		Attachments: msg.Attachments,
	}

	for _, channel := range relayTo {
		pm.Channel = channel
		go b.relayMessage(msg.Ts, pm)
	}
}

// Handle file shared event
func (b *RelayBot) handleFileShared(ev *FileShared) {
	if !b.relayGroup.HasUser(ev.UserId) {
		return
	}

	file, err := fetchFileInfo(b.config.Token, ev.FileId)
	if err != nil {
		logger.Warningf("%v", err)
		return
	}

	if strings.HasPrefix(file.Name, "botupload-") {
		return
	}

	shared := append(file.Channels, file.Groups...)
	shared = append(shared, file.IMS...)

	relayTo := b.relayGroup.DetermineRelayChannelsMulti(shared)
	if relayTo == nil {
		return
	}

	logger.Infof("to handle file %v", *ev)

	if _, ok := b.users[file.User]; !ok {
		logger.Warningf("User outdated. %+v", file)
		return
	}

	fileContent, err := downloadFile(b.config.Token, file.UrlPrivate)
	if err != nil {
		logger.Warningf("%s", err)
		return
	}

	err = uploadFile(b.config.Token, relayTo, fileContent, file)
	if err != nil {
		logger.Warningf("%s", err)
		return
	}
}

func (b *RelayBot) handleReactionAdded(ev *ReactionAdded) {
	relayTo := b.relayGroup.DetermineRelayChannels(ev.Item.Channel)
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

	requestPayload := ReactionAddRequest{Name: ev.Reaction}
	for _, relayChannelID := range relayTo {
		if _, ok := messageMap[relayChannelID]; !ok {
			continue
		}
		requestPayload.Channel = relayChannelID
		requestPayload.Timestamp = messageMap[relayChannelID]
		_, err := addReaction(b.config.Token, requestPayload)
		if err != nil {
			logger.Warningf("cant send reaction: %v", err)
		}
	}
}

// Handle receive event
func (b *RelayBot) handleEvent(ev *AnyEvent) {
	switch ev.Type {
	case "message":
		logger.Debugf("message recieved %v", string(ev.jsonMsg))
		var msgEv Message
		if err := json.Unmarshal(ev.jsonMsg, &msgEv); err != nil {
			logger.Warningf("%v", err)
			return
		}
		b.handleMessage(&msgEv)
	case "file_shared":
		logger.Debugf("file recieved %v", string(ev.jsonMsg))
		var fileEv FileShared
		if err := json.Unmarshal(ev.jsonMsg, &fileEv); err != nil {
			logger.Warningf("%v", err)
			return
		}
		b.handleFileShared(&fileEv)
	case "reaction_added":
		logger.Debugf("reaction received %v", string(ev.jsonMsg))
		var reactionAddEv ReactionAdded
		if err := json.Unmarshal(ev.jsonMsg, &reactionAddEv); err != nil {
			logger.Warningf("%v", err)
			return
		}
		b.handleReactionAdded(&reactionAddEv)
	default:
		// logger.Debugf("unhandled event %v %v", ev.EventType, string(ev.jsonMsg))
	}
}

// SetUsers set user list under bot control
func (b *RelayBot) SetUsers(users []User) {
	b.users = make(map[string]User, len(users))
	for _, u := range users {
		b.users[u.Id] = u
	}
}

func (b *RelayBot) connect() error {
	logger.Info("Call start api")
	res, err := StartAPI(b.config.Token)
	if err != nil {
		return err
	}
	b.url = res.Url
	all := append(res.Channels, res.Groups...)
	b.relayGroup = NewRelayGroup(b.config, all)
	b.SetUsers(res.Users)

	logger.Info("Connect ws")
	err = b.ws.Connect(b.url)
	if err != nil {
		return err
	}
	return nil
}

// Connect to slack websocket.
// Try until connection establish
func (b *RelayBot) Connect() {
	if err := b.connect(); err != nil {
		logger.Warningf("%v", err)
	} else {
		return
	}

	// retry loop
	t := time.NewTicker(ReconnectInterval)
	defer t.Stop()
	for range t.C {
		if err := b.connect(); err != nil {
			logger.Warningf("%v", err)
			continue
		}
		return
	}
}

// Start relay bot
func (b *RelayBot) Start() {
	logger.Info("Relay bot start")
	b.Connect()
	defer b.ws.Close()

	for {
		select {
		case ev := <-b.ws.Receive:
			var e AnyEvent
			if err := json.Unmarshal(ev, &e); err != nil {
				logger.Warningf("%v", err)
				continue
			}
			e.jsonMsg = json.RawMessage(ev)
			b.handleEvent(&e)
		case err := <-b.ws.Disconnect:
			logger.Errorf("Disconnected. Cause %v", err)
			b.Connect()
		}
	}
}
