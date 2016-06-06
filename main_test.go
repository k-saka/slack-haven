package main

import (
	"reflect"
	"testing"
)

func TestParseChannelsArg(t *testing.T) {
	input := "1,2:3,4,5"
	gs := parseChannelsArg(input)
	if !reflect.DeepEqual(gs, [][]string{{"1", "2"}, {"3", "4", "5"}}) {
		t.Errorf("Group parse failed. input: %s, parsed:%v\n", input, gs)
	}
}
