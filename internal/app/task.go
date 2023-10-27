package app

import (
	"fmt"
	"github.com/qiuchao/proxypool/pkg/healthcheck"
	"github.com/qiuchao/proxypool/pkg/provider"
	"github.com/qiuchao/proxypool/pkg/proxy"
	"github.com/qiuchao/proxypoolCheck/config"
	"github.com/qiuchao/proxypoolCheck/internal/cache"
	"log"
	"time"
	"sync"
	"sync/atomic"
	"net"
	"net/http"
	"context"
	"io"
	"github.com/Dreamacro/clash/adapter"
	C "github.com/Dreamacro/clash/constant"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"github.com/oschwald/geoip2-golang"
	"os"
	"os/exec"
	"runtime"
)

var location, _ = time.LoadLocation("PRC")

// Get all usable proxies from proxypool server and set app vars
func InitApp() error{
	if cache.AllProxiesCount > 0 && IsSleepTime() {
		return nil
	}

	log.Printf("[Andy] Start running proxypool check...")
	// Get proxies from server
	proxies, err := getAllProxies()
	if err != nil {
		log.Println("Get proxies error: ", err)
		cache.LastCrawlTime = fmt.Sprint(time.Now().In(location).Format("2006-01-02 15:04:05"), err)
		return err
	}
	for _, p := range cache.GetProxies("proxies") {
		proxies = append(proxies, p)
	}
	log.Println("[Andy] Origin proxies:", len(proxies))
	proxies = proxies.Derive().Deduplication()
	allProxiesCount := len(proxies)
	log.Println("[Andy] Unique proxies:", len(proxies))

	// healthcheck settings
	healthcheck.DelayConn = config.Config.HealthCheckConnection
	healthcheck.DelayTimeout = time.Duration(config.Config.HealthCheckTimeout) * time.Second
	healthcheck.SpeedConn = config.Config.SpeedConnection
	healthcheck.SpeedTimeout = time.Duration(config.Config.SpeedTimeout) * time.Second

	testResults := make([]Result, 0, len(proxies))

	log.Printf("[Andy] Start filter proxies, count: %d", len(proxies))

	proxies = healthcheck.CleanBadProxiesWithGrpool(proxies)
	log.Println("[Andy] After healthcheck, usable proxy count: ", len(proxies))
	if config.Config.SpeedTest == true {
		proxies = healthcheck.SpeedTestAll(proxies)
		log.Println("[Andy] After speed test, usable proxy count: ", len(proxies))
		proxies, testResults = ThirdpartSpeedTest(proxies)
		log.Println("[Andy] After third part speed test, usable proxy count: ", len(proxies))
	}

	UpdateProxyBaseInfo(proxies, testResults)

	cache.AllProxiesCount = allProxiesCount
	cache.SSProxiesCount = proxies.TypeLen("ss")
	cache.SSRProxiesCount = proxies.TypeLen("ssr")
	cache.VmessProxiesCount = proxies.TypeLen("vmess")
	cache.TrojanProxiesCount = proxies.TypeLen("trojan")
	cache.UsableProxiesCount = len(proxies)
	cache.LastCrawlTime = fmt.Sprint(time.Now().In(location).Format("2006-01-02 15:04:05"))
	cache.SetProxies("proxies", proxies)

	cache.SetString("clashproxies", provider.Clash{
		provider.Base{
			Proxies: &proxies,
		},
	}.Provide())
	cache.SetString("surgeproxies", provider.Surge{
		provider.Base{
			Proxies: &proxies,
		},
	}.Provide())

	fmt.Println("Open", config.Config.Domain+":"+config.Config.Port, "to check.")

	ExecFinishCmd()

	return nil
}

func IsSleepTime() bool {
	sleepStart := config.Config.SleepStart
	sleepEnd := config.Config.SleepEnd
	if sleepStart != sleepEnd {
		currentTime := time.Now()
		hour := currentTime.Hour()
		if sleepStart < sleepEnd {
			if hour >= sleepStart && hour < sleepEnd {
				log.Printf("[Andy] Skip this execution, sleep time form %d to %d, now: %d", sleepStart, sleepEnd, hour)
				return true
			}
		} else {
			if (hour >= sleepStart && hour <= 23) || (hour >= 0 && hour < sleepEnd) {
				log.Printf("[Andy] Skip this execution, sleep time form %d to %d, now: %d", sleepStart, sleepEnd, hour)
				return true
			}
		}
	}
	return false
}

type CountryEmoji struct {
	Code  string `json:"code"`
	Emoji string `json:"emoji"`
}

func UpdateProxyBaseInfo(proxylist proxy.ProxyList, testResults []Result) {
	data, err := os.ReadFile("resource/Country-flag-emoji.json")
	if err != nil {
		log.Fatal(err)
		return
	}
	var countryEmojiList = make([]CountryEmoji, 0)
	err = json.Unmarshal(data, &countryEmojiList)
	if err != nil {
		log.Fatalln(err.Error())
		return
	}
	// download form --> https://github.com/P3TERX/GeoLite.mmdb/releases
	db, err := geoip2.Open("resource/GeoLite2-City.mmdb")
	if err != nil {
		log.Println("[Andy] Open GeoLite2-City.mmdb failure: %s", err)
		return
	}
	defer db.Close()

	emojiMap := make(map[string]string)
	for _, i := range countryEmojiList {
		emojiMap[i.Code] = i.Emoji
	}

	countMap := make(map[string]int)
	for _, p := range proxylist {
		ips, err := net.LookupIP(p.BaseInfo().Server)
		if err != nil {
			continue
		}
		ip := net.ParseIP(ips[0].String())

		record, err := db.City(ip)
		if err != nil {
			continue
		}
		
		country := "🏁ZZ"
		countryName := record.Country.Names["en"]
		city := record.City.Names["en"]
		countryIsoCode := record.Country.IsoCode
		emoji, found := emojiMap[countryIsoCode]
		if found {
			// country = fmt.Sprintf("%v%v", emoji, countryIsoCode)
			country = fmt.Sprintf("%v %v", emoji, record.Country.Names["zh-CN"])
		}

		originName := p.BaseInfo().Name
		p.SetIP(ip.String())
		p.SetCountry(country)
		p.SetName(countryIsoCode)
		p.AddToName(fmt.Sprintf("_%s", countryName))
		if len(record.Subdivisions) > 0 {
			p.AddToName(fmt.Sprintf("_%s", record.Subdivisions[0].Names["en"]))
		} else if city != "" {
			p.AddToName(fmt.Sprintf("_%s", city))
		}
		countMap[countryName]++
		p.AddToName(fmt.Sprintf("_%.02d", countMap[countryName]))
		for _, result := range testResults {
			if result.Name == originName {
				p.AddToName(fmt.Sprintf("|%s", strings.ReplaceAll(formatBandwidth(result.Bandwidth), "/s", "")))
				break
			}
		}
		// log.Printf("[Andy] Rename proxy: %s to: %s", originName, p.BaseInfo().Name)
	}
}

func ExecFinishCmd() {
	finishCmd := config.Config.FinishCmd
	if finishCmd != "" {
		if runtime.GOOS == "windows" {
			cmd := exec.Command("cmd", finishCmd)
			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("[Andy] Execute command(%s) error: %s", cmd, err)
			} else {					
				fmt.Printf("[Andy] Execute command(%s) finish. result:\n%s", cmd, string(output))
			}
		} else if runtime.GOOS == "linux" {
			// cmd := exec.Command(finishCmd)
			cmd := exec.Command("sh", finishCmd)
			output, err := cmd.Output()
			if err != nil {
				fmt.Printf("[Andy] Execute command(%s) error: %s", cmd, err)
			} else {					
				fmt.Printf("[Andy] Execute command(%s) finish. result:\n%s", cmd, string(output))
			}
		}
	}
}


// Third part speed test from: https://github.com/faceair/clash-speedtest

type Result struct {
	Name      string
	Bandwidth float64
	TTFB      time.Duration
}

type CProxy struct {
	C.Proxy
	SecretConfig any
	OriginProxy proxy.Proxy
}

var (
	red   = "\033[31m"
	green = "\033[32m"
)

var (
	emojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{2600}-\x{26FF}\x{1F1E0}-\x{1F1FF}]`)
	spaceRegex = regexp.MustCompile(`\s{2,}`)
)

func ThirdpartSpeedTest(proxylist proxy.ProxyList) (proxy.ProxyList, []Result) {
	log.Println("[Andy] Start third part speed test")
	allProxies := make(map[string]CProxy)
	for _, value := range proxylist {
		proxyStr := value.ToClash()
		re := regexp.MustCompile("- {")
		proxyStr = re.ReplaceAllString(proxyStr, "{")

		var proxyConfig map[string]interface{}
		err := json.Unmarshal([]byte(proxyStr), &proxyConfig)
		if err != nil {
			continue
		}

		p, err := adapter.ParseProxy(proxyConfig)
		if err != nil {
			fmt.Errorf("proxy %w", err)
			continue
		}

		if _, exist := allProxies[p.Name()]; exist {
			fmt.Errorf("proxy %s is the duplicate name", p.Name())
			continue
		}
		allProxies[p.Name()] = CProxy{Proxy: p, SecretConfig: proxyConfig, OriginProxy: value}
	}

	testResults := make([]Result, 0, len(allProxies))
	allProxyNames := make([]string, 0, len(allProxies))
	for name := range allProxies {
		allProxyNames = append(allProxyNames, name)
	}
	sort.Strings(allProxyNames)

	format := "%s%-42s\t%-12s\t%-12s\033[0m\n"
	fmt.Printf(format, "", "节点", "带宽", "延迟")
	for _, name := range allProxyNames {
		proxy := allProxies[name]
		switch proxy.Type() {
		case C.Shadowsocks, C.ShadowsocksR, C.Snell, C.Socks5, C.Http, C.Vmess, C.Trojan:
			result := TestProxyConcurrent(name, proxy, config.Config.SpeedDownloadSize, time.Duration(config.Config.SpeedTimeout) * time.Second, config.Config.SpeedConnection)
			result.Printf(format)
			testResults = append(testResults, *result)
		case C.Direct, C.Reject, C.Relay, C.Selector, C.Fallback, C.URLTest, C.LoadBalance:
			continue
		default:
			log.Fatalln("Unsupported proxy type: %s", proxy.Type())
		}
	}

	switch config.Config.SpeedSort {
		case 1:
			sort.Slice(testResults, func(i, j int) bool {
				return testResults[i].Bandwidth > testResults[j].Bandwidth
			})
			log.Println("[Andy] The results are sorted by bandwidth")
		case 2:
			sort.Slice(testResults, func(i, j int) bool {
				return testResults[i].TTFB < testResults[j].TTFB
			})
			log.Println("[Andy] The results are sorted by delay")
		default:
			log.Println("[Andy] The results are sorted by proxy name")
	}

	var filterProxylist proxy.ProxyList
	for _, result := range testResults {
		if result.Bandwidth < config.Config.SpeedMinBandwidth {
			continue
		}
		ttfb := float64(result.TTFB.Milliseconds())
		if config.Config.SpeedMaxTtfb > 0 && (ttfb <= 0 || ttfb > config.Config.SpeedMaxTtfb) {
			continue
		}
		if v, ok := allProxies[result.Name]; ok {
			filterProxylist = append(filterProxylist, v.OriginProxy)
		}
	}
	return filterProxylist, testResults
}

func (r *Result) Printf(format string) {
	color := ""
	if r.Bandwidth < 1024*1024 {
		color = red
	} else if r.Bandwidth > 1024*1024*10 {
		color = green
	}
	fmt.Printf(format, color, formatName(r.Name), formatBandwidth(r.Bandwidth), formatMilliseconds(r.TTFB))
}

func formatName(name string) string {
	noEmoji := emojiRegex.ReplaceAllString(name, "")
	mergedSpaces := spaceRegex.ReplaceAllString(noEmoji, " ")
	return strings.TrimSpace(mergedSpaces)
}

func formatBandwidth(v float64) string {
	if v <= 0 {
		return "N/A"
	}
	if v < 1024 {
		return fmt.Sprintf("%.02fB/s", v)
	}
	v /= 1024
	if v < 1024 {
		return fmt.Sprintf("%.02fKB/s", v)
	}
	v /= 1024
	if v < 1024 {
		return fmt.Sprintf("%.02fMB/s", v)
	}
	v /= 1024
	if v < 1024 {
		return fmt.Sprintf("%.02fGB/s", v)
	}
	v /= 1024
	return fmt.Sprintf("%.02fTB/s", v)
}

func formatMilliseconds(v time.Duration) string {
	if v <= 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.02fms", float64(v.Milliseconds()))
}

func TestProxyConcurrent(name string, p C.Proxy, downloadSize int, timeout time.Duration, concurrentCount int) *Result {
	if concurrentCount <= 0 {
		concurrentCount = 1
	}

	chunkSize := downloadSize / concurrentCount
	totalTTFB := int64(0)
	downloaded := int64(0)

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(i int) {
			result, w := TestProxy(name, p, chunkSize, timeout)
			if w != 0 {
				atomic.AddInt64(&downloaded, w)
				atomic.AddInt64(&totalTTFB, int64(result.TTFB))
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	downloadTime := time.Since(start)

	result := &Result{
		Name:      name,
		Bandwidth: float64(downloaded) / downloadTime.Seconds(),
		TTFB:      time.Duration(totalTTFB / int64(concurrentCount)),
	}

	return result
}

func TestProxy(name string, p C.Proxy, downloadSize int, timeout time.Duration) (*Result, int64) {
	client := http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				return p.DialContext(ctx, &C.Metadata{
					Host:    host,
					DstPort: port,
				})
			},
		},
	}

	start := time.Now()
	resp, err := client.Get(fmt.Sprintf(config.Config.SpeedServer, downloadSize))
	if err != nil {
		return &Result{name, -1, -1}, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode-http.StatusOK > 100 {
		return &Result{name, -1, -1}, 0
	}
	ttfb := time.Since(start)

	written, _ := io.Copy(io.Discard, resp.Body)
	if written == 0 {
		return &Result{name, -1, -1}, 0
	}
	downloadTime := time.Since(start) - ttfb
	bandwidth := float64(written) / downloadTime.Seconds()

	return &Result{name, bandwidth, ttfb}, written
}

