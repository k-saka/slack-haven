package haven

import (
	"encoding/json"
	"errors"
)

type rtmStartRequest struct {
	SimpleLatest bool `json:"simple_latest"`
	NoUnreads    bool `json:"no_unreads"`
}

type rtmStartResponse struct {
	Ok       bool      `json:"ok"`
	URL      string    `json:"url"`
	Error    string    `json:"error"`
	Users    []user    `json:"users"`
	Channels []channel `json:"channels"`
	Mpims    []mpim    `json:"mpims"`
	Groups   []channel `json:"groups"`
}

type user struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Deleted           bool    `json:"deleted"`
	Color             string  `json:"color"`
	Profile           profile `json:"profile"`
	IsAdmin           bool    `json:"is_admin"`
	IsOwner           bool    `json:"is_owner"`
	IsPrimaryOwner    bool    `json:"is_primary_owner"`
	IsRestricted      bool    `json:"is_restricted"`
	IsUltraRestricted bool    `json:"is_ultra_restricted"`
	Has2fa            bool    `json:"has_2fa"`
	TwoFactorType     string  `json:"two_factor_type"`
	HasFile           bool    `json:"has_files"`
}

type profile struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	RealName  string `json:"real_name"`
	Email     string `json:"email"`
	Skype     string `json:"skype"`
	Phone     string `json:"phone"`
	Image24   string `json:"image_24"`
	Image32   string `json:"image_32"`
	Image48   string `json:"image_48"`
	Image72   string `json:"image_72"`
	Image192  string `json:"image_192"`
	Image512  string `json:"image_512"`
}

// FullName return realname or default name
func (p profile) FullName() string {
	if p.RealName != "" {
		return p.RealName
	}
	return "名無し@すらっくへいぶん"
}

type mpim struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	IsMpim             bool     `json:"is_mpim"`
	IsGroup            bool     `json:"is_group"`
	Created            int      `json:"created"`
	Creator            string   `json:"creator"`
	Members            []string `json:"members"`
	LastRead           string   `json:"last_read"`
	Latest             string   `json:"latest"`
	UnreadCount        int      `json:"unread_count"`
	UnreadCountDisplay int      `json:"unread_count_display"`
}

type channel struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	IsChannel  bool     `json:"is_channel"`
	IsGroup    bool     `json:"is_group"`
	IsMpim     bool     `json:"is_mpim"`
	Created    int      `json:"created"`
	Creator    string   `json:"creator"`
	IsArchived bool     `json:"is_archived"`
	IsGeneral  bool     `json:"is_general"`
	HasPins    bool     `json:"has_pins"`
	IsMember   bool     `json:"is_member"`
	LastRead   string   `json:"last_read"`
	Latest     string   `json:"latest"`
	Members    []string `json:"members"`
}

type eventType struct {
	Type string `json:"type"`
}

type anyEvent struct {
	eventType
	Event   interface{}
	jsonMsg json.RawMessage
}

type hello struct {
	eventType
}

type message struct {
	eventType
	ReplyTo     json.Number  `json:"reply_to,omitempty"`
	Channel     string       `json:"channel"`
	User        string       `json:"user"`
	Text        string       `json:"text"`
	Ts          string       `json:"ts"`
	Team        string       `json:"team"`
	SubType     string       `json:"subtype,omitempty"`
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	Fallback    string            `json:"fallback"`
	IsMsgUnfurl bool              `json:"is_msg_unfurl"`
	ChannelName string            `json:"channel_name"`
	IsShare     bool              `json:"is_share"`
	ChannelID   string            `json:"channel_id"`
	Color       string            `json:"color"`
	PreText     string            `json:"pretext"`
	AuthorName  string            `json:"author_name"`
	AuthorLink  string            `json:"author_link"`
	AuthorIcon  string            `json:"author_icon"`
	Title       string            `json:"title"`
	TitleLink   string            `json:"title_link"`
	Text        string            `json:"text"`
	Fields      []attachmentField `json:"fields"`
	ImageURL    string            `json:"image_url"`
	ThumbURL    string            `json:"thumb_url"`
	Footer      string            `json:"footer"`
	FooterIcon  string            `json:"footer_icon"`
	FromURL     string            `json:"from_url"`
	//	Ts          string            `json:"ts,omitempty"`
}

type attachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type postMessageRequest struct {
	Channel     string       `json:"channel"`
	Text        string       `json:"text"`
	LinkNames   int          `json:"link_names,omitempty"`
	UnfurlLinks bool         `json:"unfurl_links,omitempty"`
	UnfurlMedia bool         `json:"unfurl_media,omitempty"`
	UserName    string       `json:"username,omitempty"`
	AsUser      bool         `json:"as_user,omitempty"`
	IconURL     string       `json:"icon_url,omitempty"`
	IconEmoji   string       `json:"icon_emoji,omitempty"`
	Attachments []attachment `json:"attachments,omitempty"`
}

type postMessageResponse struct {
	Ok      bool   `json:"ok"`
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
	Error   string `json:"error"`
}

type slackOk struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

func (o slackOk) NewError() error {
	if o.Error != "" {
		return errors.New(o.Error)
	}
	return errors.New("slack response has something worng")
}

type slackFile struct {
	ID                 string   `json:"id"`
	Created            int      `json:"created"`
	TimeStamp          int      `json:"timestamp"`
	Name               string   `json:"name"`                 // "Pasted image at 2016_05_16 09_57 PM.png",
	Title              string   `json:"title"`                // "Pasted image at 2016-05-16, 9:57 PM",
	MimeType           string   `json:"mimetype"`             // "image/png",
	FileType           string   `json:"filetype"`             // "png"
	PrettyType         string   `json:"pretty_type"`          // "PNG"
	User               string   `json:"user"`                 // "U0EVC9JNB"
	Editable           bool     `json:"editable"`             // false
	Size               int      `json:"size"`                 // 79650
	Mode               string   `json:"mode"`                 // "hosted"
	IsExternal         bool     `json:"is_external"`          // false
	ExternalType       string   `json:"external_type"`        // ""
	IsPublic           bool     `json:"is_public"`            // false
	PublicURLShared    bool     `json:"public_url_shared"`    // false
	DisplayAsBot       bool     `json:"display_as_bot"`       // false
	UserName           string   `json:"username"`             // ""
	URLPrivate         string   `json:"url_private"`          // ""
	URLPrivateDownLoad string   `json:"url_private_download"` // ""
	Thumb64            string   `json:"thumb_64"`             // ""
	Thumb80            string   `json:"thumb_80"`             // ""
	Thumb360           string   `json:"thumb_360"`            // ""
	Thumb360w          int      `json:"thumb_360_w"`          // 324
	Thumb360h          int      `json:"thumb_360_h"`          // 204
	Thumb160           string   `json:"thumb_160"`            // ""
	ImageExifRotation  int      `json:"image_exif_rotation"`  // 1
	Originalw          int      `json:"original_w"`           // 324
	Originalh          int      `json:"original_h"`           // 204
	Permalink          string   `json:"permalink"`            // ""
	PermalinkPulic     string   `json:"permalink_public"`     // ""
	Channels           []string `json:"channels"`             // []
	Groups             []string `json:"groups"`               // [""]
	IMS                []string `json:"ims"`                  // []
	CommentCount       int      `json:"comments_count"`       // 0
}

type fileShared struct {
	eventType
	FileID  string `json:"file_id"`
	UserID  string `json:"user_id"`
	EventTs string `json:"event_ts"`
}

type fileInfo struct {
	File  slackFile `json:"file"`
	Ok    bool      `json:"ok"`
	Error string    `json:"error"`
}

type reactionAdded struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Reaction string `json:"reaction"`
	ItemUser string `json:"item_user"`
	Item     struct {
		Type        string `json:"type"`
		Channel     string `json:"channel"`
		Ts          string `json:"ts"`
		File        string `json:"file"`
		FileComment string `json:"file_comment"`
	} `json:"item"`
	EventTs string `json:"event_ts"`
}

type reactionAddRequest struct {
	Name        string `json:"name"`
	Channel     string `json:"channel"`
	File        string `json:"file,omitempty"`
	FileComment string `json:"file_comment,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

type ping struct {
	ID   uint   `json:"id"`
	Type string `json:"type"`
}
