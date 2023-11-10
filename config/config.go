package config

import (
	"errors"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"os"
	"strings"
	"net/http"
	"time"
	"crypto/tls"
	"net/url"
	"log"
)

var configFilePath = "config.yaml"

// ConfigOptions is a struct that represents config files
type ConfigOptions struct {
	ServerUrl          []string `json:"server_url" yaml:"server_url"`
	ClashConfigUrl     []string `json:"clash_config_url" yaml:"clash_config_url"`
	Domain             string   `json:"domain" yaml:"domain"`
	Port               string   `json:"port" yaml:"port"`
	Request            string   `json:"request" yaml:"request"`
	CronInterval       uint64   `json:"cron_interval" yaml:"cron_interval"`
	ProxyUrl           string    `json:"proxy_url" yaml:"proxy_url"`
	MaxProxyCount      int      `json:"max_proxy_count" yaml:"max_proxy_count"`
	HealthCheckTimeout int      `json:"healthcheck_timeout" yaml:"healthcheck_timeout"`
	HealthCheckConnection int 	`json:"healthcheck_connection" yaml:"healthcheck_connection"`
	SpeedTest          bool     `json:"speedtest" yaml:"speedtest"`
	ThirdpartSpeedtest bool     `json:"thirdpart_speedtest" yaml:"thirdpart_speedtest"`
	SpeedConnection    int      `json:"speed_connection" yaml:"speed_connection"`
	SpeedTimeout       int      `json:"speed_timeout" yaml:"speed_timeout"`
	SpeedDownloadSize  int      `json:"speed_download_size" yaml:"speed_download_size"`
	SpeedSort          int      `json:"speed_sort" yaml:"speed_sort"`
	SpeedServer        string   `json:"speed_server" yaml:"speed_server"`
	SpeedMinBandwidth  float64  `json:"speed_min_bandwidth" yaml:"speed_min_bandwidth"`
	SpeedMaxTtfb       float64  `json:"speed_max_ttfb" yaml:"speed_max_ttfb"`
	SleepStart         int      `json:"sleep_start" yaml:"sleep_start"`
	SleepEnd           int      `json:"sleep_end" yaml:"sleep_end"`
	FinishCmd          string   `json:"finish_cmd" yaml:"finish_cmd"`
	ToBadProxyTimes    int      `json:"to_bad_proxy_times" yaml:"to_bad_proxy_times"`
	SkipBadProxyTimes  int      `json:"skip_bad_proxy_times" yaml:"skip_bad_proxy_times"`
}

var Config ConfigOptions

// Parse Config file
func Parse(path string) error {
	if path == "" {
		path = configFilePath
	} else {
		configFilePath = path
	}
	fileData, err := ReadFile(path)
	if err != nil {
		return err
	}
	Config = ConfigOptions{}
	err = yaml.Unmarshal(fileData, &Config)
	if err != nil {
		return err
	}
	// set default
	if Config.ServerUrl == nil{
		return errors.New("config error: no server url")
	}
	if Config.Domain == ""{
		Config.Domain = "127.0.0.1"
	}
	if Config.Port == ""{
		Config.Port = "80"
	}
	if Config.CronInterval == 0{
		Config.CronInterval = 60
	}
	if Config.MaxProxyCount == 0{
		Config.MaxProxyCount = 50
	}
	if Config.Request == ""{
		Config.Request = "http"
	}
	if Config.HealthCheckTimeout == 0{
		Config.HealthCheckTimeout = 5
	}
	if Config.HealthCheckConnection == 0{
		Config.HealthCheckConnection = 100
	}
	if Config.SpeedConnection == 0{
		Config.SpeedConnection = 15
	}
	if Config.SpeedTimeout == 0 {
		Config.SpeedTimeout = 10
	}
	if Config.SpeedDownloadSize == 0 {
		Config.SpeedDownloadSize = 104857600
	}
	if Config.SpeedSort == 0 {
		Config.SpeedSort = 0
	}
	if Config.SpeedServer == "" {
		Config.SpeedServer = "https://speed.cloudflare.com/__down?bytes=%d"
	}
	if Config.SpeedMinBandwidth == 0 {
		Config.SpeedMinBandwidth = 1024
	}
	if Config.SpeedMaxTtfb == 0 {
		Config.SpeedMaxTtfb = 4096
	}
	if Config.SleepStart == 0{
		Config.SleepStart = 0
	}
	if Config.SleepEnd == 0{
		Config.SleepEnd = 0
	}
	if Config.ToBadProxyTimes == 0{
		Config.ToBadProxyTimes = 3
	}
	if Config.SkipBadProxyTimes == 0{
		Config.SkipBadProxyTimes = 5
	}
	return nil
}


// 从本地文件或者http链接读取配置文件内容
func ReadFile(path string) ([]byte, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		var proxy func(*http.Request) (*url.URL, error)
		if Config.ProxyUrl != "" {
			proxyUrl, err := url.Parse(Config.ProxyUrl)
			if err != nil {
				log.Printf("[Andy] Proxy url(%s) error, %s", Config.ProxyUrl, err)
			} else {
				proxy = http.ProxyURL(proxyUrl)
			}
		}

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		if proxy != nil {
			tr.Proxy = proxy
		}

		client := &http.Client{
			Timeout: 30 * time.Second,
			Transport: tr,
		}

		resp, err := client.Get(path)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return body, nil
	} else {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, err
		}
		return ioutil.ReadFile(path)
	}
}
