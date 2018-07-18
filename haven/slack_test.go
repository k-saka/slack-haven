package haven

import (
	"reflect"
	"testing"
)

var ch1 = channel{ID: "1", Members: []string{"A", "B", "C"}}
var ch2 = channel{ID: "2", Members: []string{"A", "D", "E"}}

func TestRelayGroup(t *testing.T) {
	r := relayGroup{ch1.ID: ch1, ch2.ID: ch2}
	if !r.hasChannel("1") {
		t.Error("Channel 1 not found")
	}

	if r.hasChannel("3") {
		t.Error("Group don't have channel 3 but found it")
	}

	if !r.hasUser("A") {
		t.Error("User A not found")
	}

	if r.hasUser("X") {
		t.Error("Group don't have user X but found him")
	}
}

func TestRelayGroups(t *testing.T) {
	cfg := Config{RelayRooms: map[string]struct{}{"1": {}, "2": {}}}

	chans := []channel{ch1, ch2}
	group := newRelayGroup(&cfg, chans)

	if group.channelCount() != 2 {
		t.Errorf("Expected channel count is 2. Actual: %v", group.channelCount())
	}

	d := group.determineRelayChannels(ch1.ID)
	if !reflect.DeepEqual(d, []string{ch2.ID}) {
		t.Errorf("Expected channel id equals %v. Actual: %v", ch2.ID, d)
	}

	d = group.determineRelayChannels("xxx")
	if d != nil {
		t.Errorf("Expected channel id is nil. Actual: %v", d)
	}

	d = group.determineRelayChannelsMulti([]string{ch1.ID})
	if !reflect.DeepEqual(d, []string{ch2.ID}) {
		t.Errorf("Expected channel id equals [%v]. Actual: %v", ch2.ID, d)
	}

	d = group.determineRelayChannelsMulti([]string{"xxx"})
	if d != nil {
		t.Errorf("Expected channel id is nil. Actual: %v", d)
	}

	//d = group.DetermineRelayChannelsMulti([]string{ch1.Id, ch2.Id})
	//if d != nil {
	//	t.Errorf("Expected channel id is nil. Actual: %v", d)
	//}
}
