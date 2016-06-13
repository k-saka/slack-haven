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
	RelayRooms []map[string]bool
	Token      string
}

type configJSON struct {
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

	jsonConf := configJSON{}

	if err = json.Unmarshal(buf, &jsonConf); err != nil {
		return err
	}
	c.Token = jsonConf.Token
	c.RelayRooms = make([]map[string]bool, len(jsonConf.RelayRooms))

	for i, r := range jsonConf.RelayRooms {
		c.RelayRooms[i] = map[string]bool{}
		for _, ch := range r {
			c.RelayRooms[i][ch] = true
		}
	}

	return nil
}
