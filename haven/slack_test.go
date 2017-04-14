package haven

import (
	"reflect"
	"testing"
)

var ch1 = Channel{Id: "1", Members: []string{"A", "B", "C"}}
var ch2 = Channel{Id: "2", Members: []string{"A", "D", "E"}}

func TestRelayGroup(t *testing.T) {
	r := RelayGroup{ch1.Id: ch1, ch2.Id: ch2}
	if !r.HasChannel("1") {
		t.Error("Channel 1 not found")
	}

	if r.HasChannel("3") {
		t.Error("Group don't have channel 3 but found it")
	}

	if !r.HasUser("A") {
		t.Error("User A not found")
	}

	if r.HasUser("X") {
		t.Error("Group don't have user X but found him")
	}
}

func TestRelayGroups(t *testing.T) {
	cfg := []map[string]bool{{"1": true, "2": true}}
	chans := []Channel{ch1, ch2}
	group := NewRelayGroups(cfg, chans)

	if group.ChannelCount() != 2 {
		t.Errorf("Expected channel count is 2. Actual: %v", group.ChannelCount())
	}

	d := group.DetermineRelayChannels(ch1.Id)
	if !reflect.DeepEqual(d, []string{ch2.Id}) {
		t.Errorf("Expected channel id equals %v. Actual: %v", ch2.Id, d)
	}

	d = group.DetermineRelayChannels("xxx")
	if d != nil {
		t.Errorf("Expected channel id is nil. Actual: %v", d)
	}

	d = group.DetermineRelayChannelsByChannels([]string{ch1.Id})
	if !reflect.DeepEqual(d, []string{ch2.Id}) {
		t.Errorf("Expected channel id equals [%v]. Actual: %v", ch2.Id, d)
	}

	d = group.DetermineRelayChannelsByChannels([]string{"xxx"})
	if d != nil {
		t.Errorf("Expected channel id is nil. Actual: %v", d)
	}

	d = group.DetermineRelayChannelsByChannels([]string{ch1.Id, ch2.Id})
	if d != nil {
		t.Errorf("Expected channel id is nil. Actual: %v", d)
	}
}
