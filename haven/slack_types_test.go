package haven

import (
	"encoding/json"
	"testing"
)

var TestMessage = `
{
    "type": "message",
    "user": "A",
    "text": "B",
    "user_team": "C",
    "team": "D",
    "user_profile": {
        "avatar_hash": "E",
        "image_72": "F",
        "first_name": "G",
        "real_name": "H",
        "name": "I"
    },
    "attachments": [
        {
            "fallback": "J",
            "author_subname": "K",
            "ts": "L",
            "channel_id": "M",
            "channel_name": "N",
            "is_msg_unfurl": true,
            "text": "O",
            "author_icon": "P",
            "author_link": "Q",
            "mrkdwn_in": [
                "text"
            ],
            "color": "R",
            "from_url": "S",
            "is_share": true,
            "footer": "T"
        }
    ],
    "channel": "U",
    "ts": "V"
} 
`

func TestMessageUnmarshal(t *testing.T) {
	ev := &message{}
	if err := json.Unmarshal([]byte(TestMessage), ev); err != nil {
		t.Errorf("Message parsed error. %v\n", err)
	}
}
