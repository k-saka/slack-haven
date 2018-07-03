package haven

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alexcesaro/log"
)

const (
	// ReconnectInterval is try to connect slack rtm api interval
	ReconnectInterval = time.Second * 10
)

// DefaultStartAPIParam is default slack rtm api parameter
var DefaultStartAPIParam = url.Values{
	"simple_latest": {"true"},
	"no_unreads":    {"true"},
}

var logger log.Logger

// SetLogger set logger used haven package used
func SetLogger(log log.Logger) {
	logger = log
}

// StartAPI call slack rtm.start api
func StartAPI(token string) (resp *RTMStartResponse, err error) {
	param := DefaultStartAPIParam
	param["token"] = []string{token}
	res, err := http.PostForm(RTMStartURL, param)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	var ok SlackOk
	if err := json.Unmarshal(body, &ok); err != nil {
		logger.Warningf("%v", err)
		return nil, err
	}

	if !ok.Ok {
		logger.Warningf(string(body))
		return nil, nil
	}

	var slackResponse RTMStartResponse
	if err = json.Unmarshal(body, &slackResponse); err != nil {
		return nil, err
	}

	// sort members
	for _, c := range slackResponse.Channels {
		sort.Strings(c.Members)
	}
	for _, c := range slackResponse.Groups {
		sort.Strings(c.Members)
	}

	return &slackResponse, nil
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

// DetermineRelayChannels determine relay channels by channel ids
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
	relayGroup RelayGroup
	users      map[string]User
}

// NewRelayBot create RelayBot
func NewRelayBot(config *Config) *RelayBot {
	return &RelayBot{
		config: config,
		ws:     NewWsClient(),
	}
}

// URLValues return url.Values
func (p PostMessage) URLValues() url.Values {
	val := url.Values{
		"token":      {p.Token},
		"channel":    {p.Channel},
		"text":       {p.Text},
		"link_names": {string(p.LinkNames)},
	}
	if p.UnfurlLinks {
		val.Set("unfurl_links", "true")
	}
	if p.UnfurlMedia {
		val.Set("unfurl_media", "true")
	}
	if p.UserName != "" {
		val.Set("username", p.UserName)
	}
	if p.AsUser {
		val.Set("as_user", "true")
	}
	if p.IconUrl != "" {
		val.Set("icon_url", p.IconUrl)
	}
	if p.IconEmoji != "" {
		val.Set("icon_emoji", p.IconEmoji)
	}
	if p.Attachments != nil && len(p.Attachments) > 0 {
		at, err := json.Marshal(p.Attachments)
		if err != nil {
			logger.Warningf("%v", err)
		} else {
			val.Set("attachments", string(at))
		}
	}
	return val
}

// PostMessage send a message to slack throw chat.postMessage API
func (b *RelayBot) PostMessage(pm PostMessage) {
	res, err := http.PostForm(PostMessageURL, pm.URLValues())
	if err != nil {
		logger.Warningf("%v", err)
		return
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	var ok SlackOk
	if err := json.Unmarshal(body, &ok); err != nil {
		logger.Warningf("%v", err)
		return
	}

	if !ok.Ok {
		logger.Warningf("PostMessage error, %s", body)
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
		Token:     b.config.Token,
		Channel:   cID,
		Text:      buf.String(),
		LinkNames: 0,
		UserName:  "Slack haven",
	}
	b.PostMessage(pm)
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
		Token:     b.config.Token,
		Channel:   cID,
		Text:      buf.String(),
		LinkNames: 0,
		UserName:  "Slack haven",
	}
	b.PostMessage(pm)
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

	uname := sender.Profile.RealName
	if uname == "" {
		uname = sender.Name
	}

	pm := PostMessage{
		Token:       b.config.Token,
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
		b.PostMessage(pm)
		logger.Infof("relayed message %v", pm)
	}
}

func (b *RelayBot) downloadFile(url string) (rc []byte, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+b.config.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return content, nil
}

// UploadFile send file to slack
func (b *RelayBot) UploadFile(channels []string, content []byte, file *File) (resp *http.Response, err error) {
	body := bytes.Buffer{}
	writer := multipart.NewWriter(&body)
	defer writer.Close()

	part, err := writer.CreateFormFile("file", file.Title)
	if err != nil {
		return nil, err
	}

	part.Write(content)
	_ = writer.WriteField("token", b.config.Token)
	_ = writer.WriteField("filetype", file.FileType)
	_ = writer.WriteField("filename", "botupload-"+file.Name)
	_ = writer.WriteField("channels", strings.Join(channels, ","))

	req, err := http.NewRequest("POST", UploadFileURL, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{}

	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (b *RelayBot) fetchFileInfo(id string) (f *File, err error) {
	v := url.Values{
		"token": {b.config.Token},
		"file":  {id},
	}
	res, err := http.PostForm(FileInfoURL, v)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	var ok SlackOk
	if err := json.Unmarshal(body, &ok); err != nil {
		return nil, err
	}
	if !ok.Ok {
		return nil, ok.NewError()
	}
	fileInfo := &FileInfo{}
	if err := json.Unmarshal(body, fileInfo); err != nil {
		return nil, err
	}

	return &fileInfo.File, nil
}

// Handle file shared event
func (b *RelayBot) handleFileShared(ev *FileShared) {
	if !b.relayGroup.HasUser(ev.UserId) {
		return
	}

	file, err := b.fetchFileInfo(ev.FileId)
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

	fileContent, err := b.downloadFile(file.UrlPrivate)
	if err != nil {
		logger.Warningf("%s", err)
		return
	}

	resp, err := b.UploadFile(relayTo, fileContent, file)
	if err != nil {
		logger.Warningf("%s", err)
		return
	}

	// json check
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Warningf("%s", err)
		return
	}

	ok := &SlackOk{}
	if err := json.Unmarshal(respBody, ok); err != nil {
		logger.Warningf("%s", err)
		return
	}

	if !ok.Ok {
		logger.Warningf(string(respBody))
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
	default:
	}
}

// Start relay bot
func (b *RelayBot) Start() {
	logger.Info("Relay bot start")
	b.Connect()
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
	for range t.C {
		if err := b.connect(); err != nil {
			logger.Warningf("%v", err)
			continue
		}
		return
	}
}
