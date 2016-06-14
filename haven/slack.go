package haven

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"
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
		log.Printf("%v\n", err)
		return nil, err
	}

	if !ok.Ok {
		log.Println(string(body))
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

// HasUser tests a user exites in RelayGroup
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

//NewRelayGroup create RelayGroup
func NewRelayGroup(cfg map[string]bool, chans []Channel) RelayGroup {
	g := RelayGroup{}
	for _, channel := range chans {
		if _, ok := cfg[channel.Id]; !ok {
			continue
		}
		g[channel.Id] = channel
	}
	return g
}

// RelayGroups is slice of RelayGroup
type RelayGroups []RelayGroup

// NewRelayGroups create RelayGroups
func NewRelayGroups(cfg []map[string]bool, channels []Channel) RelayGroups {
	groups := make(RelayGroups, 0, len(cfg))
	for _, cfgGroup := range cfg {
		groups = append(groups, NewRelayGroup(cfgGroup, channels))
	}
	return groups
}

// ChannelCount count up channes in RelayGroups
func (g RelayGroups) ChannelCount() int {
	cc := 0
	for _, gr := range g {
		cc += len(gr)
	}
	return cc
}

// HasUser test RelayGroups having a user
func (g RelayGroups) HasUser(uID string) bool {
	for _, gr := range g {
		if gr.HasUser(uID) {
			return true
		}
	}
	return false
}

// DeterminRelayChannels determin channels which is relayed by receive cid
func (g RelayGroups) DeterminRelayChannels(cid string) []string {
	toRelay := make([]string, 0, g.ChannelCount())
	for _, gr := range g {
		// skip group don't have the cid
		if !gr.HasChannel(cid) {
			continue
		}
		for _, ch := range gr {
			if ch.Id == cid {
				continue
			}
			toRelay = append(toRelay, ch.Id)
		}
	}
	if len(toRelay) < 1 {
		return nil
	}
	return toRelay
}

// DeterminRelayChannelsByChannnels determin channels which is relayed by receive cids
func (g RelayGroups) DeterminRelayChannelsByChannnels(cids []string) []string {
	toRelay := map[string]bool{}
	for _, cid := range cids {
		if chs := g.DeterminRelayChannels(cid); chs != nil {
			for _, ch := range chs {
				toRelay[ch] = true
			}
		}
	}
	if len(toRelay) < 1 {
		return nil
	}

	if len(cids) == len(toRelay) {
		orig := make(map[string]bool, len(cids))
		for _, c := range cids {
			orig[c] = true
		}
		if reflect.DeepEqual(orig, toRelay) {
			return nil
		}
	}

	toRelayUniq := []string{}
	for c := range toRelay {
		toRelayUniq = append(toRelayUniq, c)
	}
	return toRelayUniq
}

// RelayBot relay multiple channels
// Relayable events are chat, file and shared message.
type RelayBot struct {
	url         string
	ws          *WsClient
	config      *Config
	relayGroups RelayGroups
	users       map[string]User
}

// NewRelayBot create RelayBot
func NewRelayBot(config *Config) *RelayBot {
	return &RelayBot{
		config: config,
		ws:     NewWsCleint(),
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
			log.Printf("%v\n", err)
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
		log.Printf("%v\n", err)
		return
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	var ok SlackOk
	if err := json.Unmarshal(body, &ok); err != nil {
		log.Printf("%v\n", err)
		return
	}

	if !ok.Ok {
		log.Printf("PostMessage error, %s\n", body)
	}
}

func (b *RelayBot) postMembersInfo(cID string) {
	userIds := []string{}
	for _, g := range b.relayGroups {
		if g.HasChannel(cID) {
			userIds = g.UserIDs()
			break
		}
	}

	if len(userIds) < 1 {
		return
	}

	var buf bytes.Buffer
	buf.WriteString("```")
	buf.WriteString("Haven members:\n")
	for _, uid := range userIds {
		user, ok := b.users[uid]
		if !ok {
			continue
		}

		buf.WriteString(fmt.Sprintf("Account:%s\t\t Name:%s\n", user.Name, user.Profile.FullName()))
	}
	buf.WriteString("```")

	pm := PostMessage{
		Token:     b.config.Token,
		Channel:   cID,
		Text:      buf.String(),
		LinkNames: 0,
		UserName:  "Bot",
	}
	b.PostMessage(pm)
}

func (b *RelayBot) handleSystemMessage(msg *Message) {
	if strings.Contains(strings.ToLower(msg.Text), "members") {
		b.postMembersInfo(msg.Channel)
	}
}

// Handle receive message
func (b *RelayBot) handleMessage(msg *Message) {
	if msg.ReplyTo != 0 {
		return
	}

	if msg.SubType == "bot_message" {
		return
	}

	if msg.SubType == "file_share" {
		return
	}

	if strings.HasPrefix(strings.ToLower(msg.Text), "haven") {
		b.handleSystemMessage(msg)
		return
	}

	relayTo := b.relayGroups.DeterminRelayChannels(msg.Channel)
	if relayTo == nil {
		return
	}

	sender, ok := b.users[msg.User]
	if !ok {
		log.Println("User outdated")
		log.Printf("%+v\n", msg)
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
	if !b.relayGroups.HasUser(ev.UserId) {
		return
	}

	file, err := b.fetchFileInfo(ev.FileId)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	if strings.HasPrefix(file.Name, "botupload-") {
		return
	}

	shared := append(file.Channels, file.Groups...)
	shared = append(shared, file.IMS...)

	relayTo := b.relayGroups.DeterminRelayChannelsByChannnels(shared)
	if relayTo == nil {
		return
	}

	if _, ok := b.users[file.User]; !ok {
		log.Println("User outdated")
		log.Printf("%+v\n", file)
		return
	}

	fileContent, err := b.downloadFile(file.UrlPrivate)
	if err != nil {
		log.Printf("%s\n", err)
		return
	}

	resp, err := b.UploadFile(relayTo, fileContent, file)
	if err != nil {
		log.Printf("%s\n", err)
		return
	}

	// json check
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("%s\n", err)
		return
	}

	ok := &SlackOk{}
	if err := json.Unmarshal(respBody, ok); err != nil {
		log.Printf("%s\n", err)
		return
	}

	if !ok.Ok {
		log.Println(string(respBody))
	}
}

// Handle receive event
func (b *RelayBot) handleEvent(ev *AnyEvent) {
	switch ev.Type {
	case "message":
		log.Println(string(ev.jsonMsg))
		var msgEv Message
		if err := json.Unmarshal(ev.jsonMsg, &msgEv); err != nil {
			log.Printf("%v\n", err)
			return
		}
		b.handleMessage(&msgEv)
	case "file_shared":
		log.Println(string(ev.jsonMsg))
		var fileEv FileShared
		if err := json.Unmarshal(ev.jsonMsg, &fileEv); err != nil {
			log.Printf("%v\n", err)
			return
		}
		b.handleFileShared(&fileEv)
	default:
	}
}

// Start relay bot
func (b *RelayBot) Start() {
	log.Println("relay bot start")
	b.Connect()
	for {
		select {
		case ev := <-b.ws.Receive:
			var e AnyEvent
			if err := json.Unmarshal(ev, &e); err != nil {
				log.Printf("%v\n", err)
				continue
			}
			e.jsonMsg = json.RawMessage(ev)
			b.handleEvent(&e)
		case err := <-b.ws.Disconnect:
			log.Printf("Disconnected. Cause %v\n", err)
			b.Connect()
		}
	}
}

// SetUsers set user list under bot controll
func (b *RelayBot) SetUsers(users []User) {
	b.users = make(map[string]User, len(users))
	for _, u := range users {
		b.users[u.Id] = u
	}
}

func (b *RelayBot) connect() error {
	log.Println("Call start api")
	res, err := StartAPI(b.config.Token)
	if err != nil {
		return err
	}
	b.url = res.Url
	all := append(res.Channels, res.Groups...)
	b.relayGroups = NewRelayGroups(b.config.RelayRooms, all)
	b.SetUsers(res.Users)

	log.Println("Connect ws")
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
		log.Printf("%v\n", err)
	} else {
		return
	}

	// retry loop
	t := time.NewTicker(ReconnectInterval)
	for range t.C {
		if err := b.connect(); err != nil {
			log.Printf("%v\n", err)
			continue
		}
		return
	}
}
