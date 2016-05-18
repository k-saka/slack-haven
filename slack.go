package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	// Trying reconnect inverval
	ReconnectInterval = time.Second * 10
)

// RTM API parameter
type StartAPIParam struct {
	Token        string `joson:"token"`
	SimpleLatest bool
	NoUnread     bool
	MPIMAware    bool
}

// Convert to url.Values
func (p *StartAPIParam) URLValues() url.Values {
	val := url.Values{"token": {p.Token}}
	if p.SimpleLatest {
		val.Set("simple_latest", "true")
	}
	if p.NoUnread {
		val.Set("no_unreads", "true")
	}
	if p.MPIMAware {
		val.Set("mpim_aware", "true")
	}
	return val
}

// Call slack RTM start api
func StartAPI(token string) (resp *RTMStartResponse, err error) {
	data := StartAPIParam{
		Token:        token,
		SimpleLatest: true,
		NoUnread:     true,
	}
	res, err := http.PostForm(RTMStartURL, data.URLValues())
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

// Relay group
type RelayGroup []Channel

// Group has channel?
func (g RelayGroup) HasChannel(cID string) bool {
	for _, ch := range g {
		if ch.Id == cID {
			return true
		}
	}
	return false
}

// Relay group definition
type RelayGroups struct {
	groups []RelayGroup
}

// Create Relay group
func NewRelayGroups(cfg [][]string, chans []Channel) RelayGroups {
	groups := make([]RelayGroup, 0, len(cfg))
	for _, group := range cfg {
		relayGroup := make([]Channel, 0, len(group))
		for _, channelID := range group {
			for _, channel := range chans {
				if channel.Id != channelID {
					continue
				}
				relayGroup = append(relayGroup, channel)
				break
			}
		}
		groups = append(groups, relayGroup)
	}
	return RelayGroups{groups: groups}
}

// Channel count
func (g *RelayGroups) ChannelCount() int {
	cc := 0
	for _, group := range g.groups {
		cc += len(group)
	}
	return cc
}

// Determin to relay channels
func (g *RelayGroups) DeterminRelayChannels(cid string) []string {
	toRelay := make([]string, 0, g.ChannelCount())
	for _, group := range g.groups {
		if !group.HasChannel(cid) {
			continue
		}
		for _, ch := range group {
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

// Determin to relay channels
func (g *RelayGroups) DeterminRelayChannelsByChannnels(cids []string) []string {
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
	toRelayUniq := make([]string, 0, len(toRelay))
	for k, _ := range toRelay {
		toRelayUniq = append(toRelayUniq, k)
	}
	return toRelayUniq
}

// Relay bot
type RelayBot struct {
	url         string
	ws          *WsClient
	config      *Config
	relayGroups RelayGroups
	users       []User
}

// Create default RealayBot
func NewRelayBot(config *Config) *RelayBot {
	return &RelayBot{
		config: config,
		ws:     NewWsCleint(),
	}
}

// Convert to url.Values
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

	return val
}

// Post message
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
		log.Printf("PostMessage error, %v\n", body)
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

	relayTo := b.relayGroups.DeterminRelayChannels(msg.Channel)
	if relayTo == nil {
		return
	}

	sender := b.SearchUsers(msg.User)
	if sender == nil {
		log.Println("User outdated")
		log.Printf("%+v\n", msg)
		return
	}

	pm := PostMessage{
		Token:    b.config.Token,
		Text:     msg.Text,
		UserName: sender.Profile.RealName,
		AsUser:   false,
		IconUrl:  sender.Profile.Image512,
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

// Upload file to slack
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

// Handle file shared event
func (b *RelayBot) handleFileShared(ev *FileShared) {
	if strings.HasPrefix(ev.File.Name, "botupload-") {
		return
	}
	shared := append(ev.File.Channels, ev.File.Groups...)
	shared = append(shared, ev.File.IMS...)

	relayTo := b.relayGroups.DeterminRelayChannelsByChannnels(shared)
	if relayTo == nil {
		return
	}
	sender := b.SearchUsers(ev.File.User)
	if sender == nil {
		log.Println("User outdated")
		log.Printf("%+v\n", ev)
		return
	}

	fileContent, err := b.downloadFile(ev.File.UrlPrivate)
	if err != nil {
		log.Printf("%s\n", err)
		return
	}

	resp, err := b.UploadFile(relayTo, fileContent, &ev.File)
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

// Bot loop start
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

// for sort
type Users []User

// Set users
func (b *RelayBot) SetUsers(users []User) {
	sort.Sort(Users(users))
	b.users = users
}

// Search user list
func (b *RelayBot) SearchUsers(uid string) *User {
	i := sort.Search(len(b.users), func(i int) bool { return b.users[i].Id >= uid })
	if i < len(b.users) && b.users[i].Id == uid {
		return &b.users[i]
	}
	return nil
}

func (u Users) Len() int           { return len(u) }
func (u Users) Less(i, j int) bool { return u[i].Id < u[j].Id }
func (u Users) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }

var _ sort.Interface = Users(nil)

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

// Try Connect to slack websocket api until connection establish
func (b *RelayBot) Connect() {
	if err := b.connect(); err == nil {
		return
	} else {
		log.Printf("%v\n", err)
	}

	// retry loop
	t := time.NewTicker(ReconnectInterval)
	for _ = range t.C {
		if err := b.connect(); err != nil {
			log.Printf("%v\n", err)
			continue
		}
		return
	}
}
