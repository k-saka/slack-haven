package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/stdlog"
	"github.com/k-saka/slack-haven/haven"
)

var version string // version number or build hash

var logger log.Logger // global logger

// Parse channel command line argument
func parseChannelsArg(arg *string) map[string]struct{} {
	rooms := strings.Split(*arg, ",")
	roomConf := make(map[string]struct{}, len(rooms))
	for _, room := range rooms {
		roomConf[room] = struct{}{}
	}
	return roomConf
}

func configure(c *haven.Config) error {
	// Try reading config file
	if err := haven.ConfigLoadFromFile(c); err != nil {
		return err
	}

	// Overwrite config with command line options
	if *argToken != "" {
		c.Token = *argToken
	}

	if *argChannels != "" {
		c.RelayRooms = parseChannelsArg(argChannels)
	}

	// Validate options
	if c.Token == "" {
		return errors.New("Token is empty")
	}

	if len(c.RelayRooms) < 2 {
		return errors.New("Invalid room count")
	}

	return nil
}

func signalListener() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	s := <-sigChan
	logger.Warningf("Got signal %v", s)
}

var showVersion *bool
var argToken *string
var argChannels *string

func init() {
	showVersion = flag.Bool("version", false, "show version and exit")
	argToken = flag.String("token", "", "slack token")
	argChannels = flag.String("channel", "", "To relay channels definition, ex. id1,id2")
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	logger = stdlog.GetFromFlags()
	haven.SetLogger(logger)
	c := &haven.Config{}
	err := configure(c)
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}

	bot := haven.NewRelayBot(c)
	go bot.Start()
	signalListener()
}
