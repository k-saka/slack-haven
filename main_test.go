package main

import (
	"reflect"
	"testing"
)

func TestParseChannelsArg(t *testing.T) {
	input := "1,2"
	gs := parseChannelsArg(&input)
	expected := map[string]struct{}{"1": {}, "2": {}}

	if !reflect.DeepEqual(gs, expected) {
		t.Errorf("Group parse failed. input: %s, parsed:%v\n", input, gs)
	}
}
