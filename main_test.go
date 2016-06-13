package main

import (
	"reflect"
	"testing"
)

func TestParseChannelsArg(t *testing.T) {
	input := "1,2:3,4,5"
	gs := parseChannelsArg(input)
	if !reflect.DeepEqual(gs, []map[string]bool{{"1": true, "2": true}, {"3": true, "4": true, "5": true}}) {
		t.Errorf("Group parse failed. input: %s, parsed:%v\n", input, gs)
	}
}
