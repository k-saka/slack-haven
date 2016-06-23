package main

import (
	"errors"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/stdlog"
	"github.com/k-saka/slack-haven/haven"
)

var logger log.Logger

func parseChannelsArg(arg string) []map[string]bool {
	groupsArg := strings.Split(arg, ":")
	groups := make([]map[string]bool, len(groupsArg))
	for i, rooms := range groupsArg {
		groups[i] = map[string]bool{}
		for _, cID := range strings.Split(rooms, ",") {
			groups[i][cID] = true
		}
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
	logger.Warningf("Got signal %v", s)
}

func main() {
	logger = stdlog.GetFromFlags()
	haven.SetLogger(logger)
	c := &haven.Config{}
	err := configure(c)
	if err != nil {
		logger.Errorf("%v\n", err)
		os.Exit(1)
	}

	bot := haven.NewRelayBot(c)
	go bot.Start()
	signalListener()
}
