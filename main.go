package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"os/user"
	"path"
	"strings"
	"syscall"
)

// Bot config
type Config struct {
	RelayRooms [][]string `json:"relay-rooms"`
	Token      string     `json:"token"`
}

// Read config file
func ConfigLoadFromFile() (*Config, error) {
	usr, err := user.Current()
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	// Trying read config file
	configPath := path.Join(usr.HomeDir, ".slack-haven")
	if _, err := os.Stat(configPath); err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	if err = json.Unmarshal(buf, config); err != nil {
		return nil, err
	}
	return config, nil
}

func configure() *Config {
	// Trying read config file
	config, _ := ConfigLoadFromFile()
	token := flag.String("token", "", "slack token")
	channels := flag.String("channel", "", "channels, ex. id1,id2:id3,id4")
	flag.Parse()

	// Overwrite config with command line option
	if *token != "" {
		config.Token = *token
	}

	if *channels != "" {
		rawRelayRooms := strings.Split(*channels, ":")
		relayRooms := make([][]string, len(rawRelayRooms))
		for index, rooms := range rawRelayRooms {
			relayRooms[index] = strings.Split(rooms, ",")
		}
		config.RelayRooms = relayRooms
	}

	if config.Token == "" {
		log.Fatal("Token is empty")
	}

	for _, room := range config.RelayRooms {
		if len(room) < 2 {
			log.Fatal("Invalid room count")
		}
	}

	return config
}

func signalListenr() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	s := <-sigChan
	log.Printf("Got signal %v", s)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	config := configure()

	bot := NewRelayBot(config)
	go bot.Start()
	signalListenr()
}
