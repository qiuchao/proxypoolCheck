package app

import (
	"encoding/json"
	"errors"
	"github.com/qiuchao/proxypool/pkg/proxy"
	"github.com/qiuchao/proxypoolCheck/config"
	"gopkg.in/yaml.v2"
	"log"
	"strings"
	"strconv"
)

func getAllProxies() (proxy.ProxyList, error) {
	var proxylist proxy.ProxyList
	var errs []error // collect errors
	log.Printf("[Andy] Get all proxies")
	
	for _, url := range config.Config.ClashConfigUrl {
		proxyList, err := getClashConfigProxies(url)

		if err != nil {
			log.Printf("Error when fetch %s: %s\n", url, err.Error())
			errs = append(errs, err)
			continue
		}
		for _, value := range proxyList {
			proxylist = append(proxylist, value)
		}
		log.Printf("[Andy] Get proxies from clash config, url: %s\tproxies: %d", url, len(proxyList))
	}

	for _, value := range config.Config.ServerUrl {
		url := formatURL(value)
		pjson, err := getProxies(url)

		if err != nil {
			log.Printf("Error when fetch %s: %s\n", url, err.Error())
			errs = append(errs, err)
			continue
		}

		var count = 0
		for i, p := range pjson {
			if i == 0 || len(p) < 2 {
				continue
			}
			p = p[2:] // remove "- "

			if pp, ok := convert2Proxy(p); ok {
				if i == 1 && pp.BaseInfo().Name == "NULL" {
					log.Println("no proxy on " + url)
					errs = append(errs, errors.New("no proxy on "+url))
					continue
				}
				// name := strings.Replace(pp.BaseInfo().Name, " |", "_", 1)
				// pp.SetName(name)
				proxylist = append(proxylist, pp)
				count = count + 1
			}
		}
		log.Printf("[Andy] Get proxies from proxypool, url: %s\tproxies: %d", url, count)
	}

	if proxylist == nil {
		if errs != nil {
			errInfo := "\n"
			for _, e := range errs {
				errInfo = errInfo + e.Error() + ";\n"
			}
			return nil, errors.New(errInfo)
		}
		return nil, errors.New("no proxy")
	}

	countMap := make(map[string]int)
	for _, p := range proxylist {
		name := strings.Replace(p.BaseInfo().Name, " |", "_", 1)
		c := countMap[name]
		countMap[name]++
		if c > 0 {
			if name == "" {
				name = "unknown"
			}
			newName := name + strconv.Itoa(c + 1)
			// log.Printf("[Andy] Change proxy name form %s to %s", name, newName)
			name = newName
		}
		p.SetName(name)
	}
	return proxylist, nil
}

func formatURL(value string) string {
	url := "http://127.0.0.1:8080"
	if value != "http://127.0.0.1:8080" {
		url = value
		if url[len(url)-1] == '/' {
			url = url[:len(url)-1]
		}
	}
	// urls := strings.Split(url, "/")
	// if urls[len(urls)-2] != "clash" {
	// 	url = url + "/clash/proxies"
	// }
	return url
}

type ClashConfig struct {
	Proxy []map[string]interface{} `yaml:"proxies"`
}

// get proxy strings from url
func getProxies(url string) ([]string, error) {
	fileData, err := config.ReadFile(url)
	if err != nil {
		return nil, err
	}
	proxyJson := strings.Split(string(fileData), "\n")

	if len(proxyJson) < 2 {
		return nil, errors.New("no proxy on " + url)
	}
	return proxyJson, nil
}

func getClashConfigProxies(path string) (proxy.ProxyList, error) {
	fileData, err := config.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cf ClashConfig
	err = yaml.Unmarshal(fileData, &cf)
	if err != nil {
		return nil, err
	} 

	proxyList := make(proxy.ProxyList, 0)
	for _, pjson := range cf.Proxy {
		p, err := parseProxyFromClashProxy(pjson)
		if err == nil && p != nil {
			proxyList = append(proxyList, p)
		}
	}

	return proxyList, nil
}

func parseProxyFromClashProxy(p map[string]interface{}) (_p proxy.Proxy, err error) {
	pjson, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	switch p["type"].(string) {
	case "ss":
		var _p proxy.Shadowsocks
		err := json.Unmarshal(pjson, &_p)
		if err != nil {
			return nil, err
		}
		return &_p, nil
	case "ssr":
		var _p proxy.ShadowsocksR
		err := json.Unmarshal(pjson, &_p)
		if err != nil {
			return nil, err
		}
		return &_p, nil
	case "vmess":
		var _p proxy.Vmess
		err := json.Unmarshal(pjson, &_p)
		if err != nil {
			return nil, err
		}
		return &_p, nil
	case "trojan":
		var _p proxy.Trojan
		err := json.Unmarshal(pjson, &_p)
		if err != nil {
			return nil, err
		}
		return &_p, nil
	}
	return nil, errors.New("clash json parse failed")
}

// Convert json string(clash format) to proxy
func convert2Proxy(pjson string) (proxy.Proxy, bool) {
	var f interface{}
	err := json.Unmarshal([]byte(pjson), &f)
	if err != nil {
		return nil, false
	}
	jsnMap := f.(interface{}).(map[string]interface{})

	switch jsnMap["type"].(string) {
	case "ss":
		var p proxy.Shadowsocks
		err := json.Unmarshal([]byte(pjson), &p)
		if err != nil {
			return nil, false
		}
		return &p, true
	case "ssr":
		var p proxy.ShadowsocksR
		err := json.Unmarshal([]byte(pjson), &p)
		if err != nil {
			return nil, false
		}
		return &p, true
	case "vmess":
		var p proxy.Vmess
		err := json.Unmarshal([]byte(pjson), &p)
		if err != nil {
			return nil, false
		}
		return &p, true
	case "trojan":
		var p proxy.Trojan
		err := json.Unmarshal([]byte(pjson), &p)
		if err != nil {
			return nil, false
		}
		return &p, true
	}
	return nil, false
}
