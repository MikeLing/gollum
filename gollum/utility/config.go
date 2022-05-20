package utility

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

var configonce sync.Once

// Config store config in config.json
type Config struct {
	BindAddr            string   `json:"bindAddr"`
	BindPort            string   `json:"bindPort"`
	Logpath             string   `json:"logpath"`
	Nsnames             []string `json:"nsnames"`
	RequestTimeout      int      `json:"requestTimeout"`
	CacheDefaultTimeout int      `json:"cacheDefaultTimeout"`
	CleanInterval       int      `json:"cleanInterval"`
	ReportInterval      int      `json:"reportInterval"`
	WithHttpDNS         bool     `json:"withHttpDNS"`
	HTTPDnServer        string   `json:"hTTPDnServer"`
	GOOGLEDnServer      bool     `json:"googleDnServer"`
	GOOGLEDnSUrl        string   `json:"googleDnSUrl"`
	AliSecretKey        string   `json:"aliSecretKey"`
	BlackList           []string
	GoogleDNSIP         string
	HasGoogleDNSIP      bool
	Region              string `json:"region"`
}

// GollumConfig is the configuration file for gollum
var GollumConfig *Config

// LoadConfig will return a Config object
func LoadConfig() *Config {
	c := new(Config)
	jsonFile, err := os.Open("/etc/gollum.config.json")
	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()
	json.NewDecoder(jsonFile).Decode(c)

	file, err := os.Open("./blacklist")
	if err != nil {
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	c.BlackList = lines

	if c.Region == "na" {
		c.GoogleDNSIP = "8.8.8.8"
		c.HasGoogleDNSIP = true
		return c
	}
	c.GoogleDNSIP = ""
	c.HasGoogleDNSIP = false
	return c

}

// GetConfig will return a Config object
func GetConfig() *Config {
	configonce.Do(func() {
		GollumConfig = LoadConfig()
	})
	return GollumConfig
}
