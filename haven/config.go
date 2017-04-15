package haven

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/mitchellh/go-homedir"
)

// Config relay channels
type Config struct {
	RelayRooms map[string]struct{}
	Token      string
}

type configJSON struct {
	RelayRooms []string `json:"relay-rooms"`
	Token      string   `json:"token"`
}

// ConfigLoadFromFile read config file
func ConfigLoadFromFile(c *Config) error {
	home, err := homedir.Dir()

	if err != nil {
		return err
	}

	// Try read config file
	// TODO pass from command line argument
	configPath := path.Join(home, ".slack-haven")
	if _, err := os.Stat(configPath); err != nil {
		return err
	}

	buf, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	jsonConf := configJSON{}

	if err = json.Unmarshal(buf, &jsonConf); err != nil {
		return err
	}
	c.Token = jsonConf.Token
	c.RelayRooms = make(map[string]struct{}, len(jsonConf.RelayRooms))

	for _, r := range jsonConf.RelayRooms {
		c.RelayRooms[r] = struct{}{}
	}

	return nil
}
