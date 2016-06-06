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
	RelayRooms [][]string `json:"relay-rooms"`
	Token      string     `json:"token"`
}

// ConfigLoadFromFile read config file
func ConfigLoadFromFile(c *Config) error {
	home, err := homedir.Dir()
	if err != nil {
		return err
	}

	// Trying read config file
	configPath := path.Join(home, ".slack-haven")
	if _, err := os.Stat(configPath); err != nil {
		return err
	}

	buf, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(buf, c); err != nil {
		return err
	}

	return nil
}
