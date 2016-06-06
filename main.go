package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/k-saka/slack-haven/haven"
)

func parseChannelsArg(arg string) [][]string {
	groupsArg := strings.Split(arg, ":")
	groups := make([][]string, len(groupsArg))
	for i, rooms := range groupsArg {
		groups[i] = strings.Split(rooms, ",")
	}
	return groups
}

func configure(c *haven.Config) error {
	// Try reading config file
	_ = haven.ConfigLoadFromFile(c)

	// Overwrite config with command line options
	flag.StringVar(&c.Token, "token", c.Token, "slack token")
	channels := flag.String("channel", "", "To relay channels definition, ex. id1,id2:id3,id4")
	flag.Parse()

	if *channels != "" {
		c.RelayRooms = parseChannelsArg(*channels)
	}

	// Validate options
	if c.Token == "" {
		return errors.New("Token is empty")
	}

	for _, room := range c.RelayRooms {
		if len(room) < 2 {
			return errors.New("Invalid room count")
		}
	}
	return nil
}

func signalListener() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	s := <-sigChan
	log.Printf("Got signal %v", s)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	c := &haven.Config{}
	err := configure(c)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	bot := haven.NewRelayBot(c)
	go bot.Start()
	signalListener()
}
