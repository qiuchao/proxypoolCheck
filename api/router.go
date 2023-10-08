package api

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"io/ioutil"
	"regexp"

	"github.com/ssrlive/proxypool/pkg/provider"
	"github.com/ssrlive/proxypoolCheck/config"
	"github.com/ssrlive/proxypoolCheck/internal/app"
	appcache "github.com/ssrlive/proxypoolCheck/internal/cache"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
)

const version = "v0.7.3"

var router *gin.Engine

func GetAllProxiesText() (string, string) {
	text := appcache.GetString("clashproxies")
	if text == "" {
		proxies := appcache.GetProxies("proxies")
		clash := provider.Clash{
			Base: provider.Base{
				Proxies: &proxies,
			},
		}
		text = clash.Provide()
		appcache.SetString("clashproxies", text)
	}

	nameList := []string{}
	sp := strings.Split(string(text), "\n")
	for _, p := range sp {
		re := regexp.MustCompile(`"name":"([^"]+)"`)
		match := re.FindStringSubmatch(p)
		if len(match) > 1 {
			name := " '" + match[1] + "'"
			nameList = append(nameList, name)
		}
	}
	proxyNames := strings.Join(nameList, ",")

	re := regexp.MustCompile(`"(\w+)":`)
	text = re.ReplaceAllString(text, "$1: ")
	re = regexp.MustCompile(`(\{|\}|,)(\S)`)
	text = re.ReplaceAllString(text, "$1 $2")
	re = regexp.MustCompile("- {")
	text = re.ReplaceAllString(text, "    - {")

	return text, proxyNames
}

func GetCountryProxies(proxyCountry string, proxyNotCountry string, allProxiesNames string, countryName string, countryList string, conntryProxies string) (string, string) {
	proxies := appcache.GetProxies("proxies")
	clash := provider.Clash{
		provider.Base{
			Proxies:    &proxies,
			Types:      "",
			Country:    proxyCountry,
			NotCountry: proxyNotCountry,
			Speed:      "",
			Filter:     "",
		},
	}
	text := clash.Provide()

	nameList := []string{}
	sp := strings.Split(string(text), "\n")
	for _, p := range sp {
		re := regexp.MustCompile(`"name":"([^"]+)"`)
		match := re.FindStringSubmatch(p)
		if len(match) > 1 {
			name := " '" + match[1] + "'"
			match, _ := regexp.MatchString(name, allProxiesNames)
			if allProxiesNames == "" || match {
				nameList = append(nameList, name)
			}
		}
	}
	if len(nameList) > 0 {
		conntryProxyTemp := "    type: url-test\n    url: 'http://www.gstatic.com/generate_204'\n    interval: 3600"
		countryList += "      - " + countryName + "\n"
		conntryProxies += "  - name: " + countryName + "\n" + conntryProxyTemp + "\n    proxies: [" + strings.Join(nameList, ",") + "]\n"
	}

	return countryList, conntryProxies
}

func setupRouter() {
	gin.SetMode(gin.ReleaseMode)
	router = gin.New() // 没有任何中间件的路由
	store := persistence.NewInMemoryStore(time.Minute)
	router.Use(gin.Recovery(), cache.SiteCache(store, time.Minute))

	_ = RestoreAssets("", "assets/html")
	_ = RestoreAssets("", "assets/css")

	temp, err := loadHTMLTemplate()
	if err != nil {
		panic(err)
	}
	router.SetHTMLTemplate(temp)
	router.StaticFile("/css/index.css", "assets/css/index.css")
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "assets/html/index.html", gin.H{
			"domain":               config.Config.Domain,
			"request":              config.Config.Request,
			"port":                 config.Config.Port,
			"all_proxies_count":    appcache.AllProxiesCount,
			"ss_proxies_count":     appcache.SSProxiesCount,
			"ssr_proxies_count":    appcache.SSRProxiesCount,
			"vmess_proxies_count":  appcache.VmessProxiesCount,
			"trojan_proxies_count": appcache.TrojanProxiesCount,
			"useful_proxies_count": appcache.UsableProxiesCount,
			"last_crawl_time":      appcache.LastCrawlTime,
			"version":              version,
		})
	})
	router.GET("/clash", func(c *gin.Context) {
		c.HTML(http.StatusOK, "assets/html/clash.html", gin.H{
			"domain":  config.Config.Domain,
			"port":    config.Config.Port,
			"request": config.Config.Request,
		})
	})

	router.GET("/surge", func(c *gin.Context) {
		c.HTML(http.StatusOK, "assets/html/surge.html", gin.H{
			"domain":  config.Config.Domain,
			"request": config.Config.Request,
			"port":    config.Config.Port,
		})
	})

	router.GET("/clash/config1", func(c *gin.Context) {
		content, err := ioutil.ReadFile("template/clash-config1.yaml")
		if err != nil {
			log.Println("无法读取文件:", err)
		}

		proxyList, proxyNames := GetAllProxiesText()
		countryList := ""
		conntryProxies := ""
		countryList, conntryProxies = GetCountryProxies("AU", "", proxyNames, "🇦🇺 澳大利亚", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("CN,HK,TW", "", proxyNames, "🇨🇳 中国", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("US", "", proxyNames, "🇺🇸 美国", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("CA", "", proxyNames, "🇨🇦 加拿大", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("JP", "", proxyNames, "🇯🇵 日本", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("SG", "", proxyNames, "🇸🇬 新加坡", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("RU", "", proxyNames, "🇷🇺 俄罗斯", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("CH", "", proxyNames, "🇨🇭 瑞士", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("DE", "", proxyNames, "🇩🇪 德国", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("FR", "", proxyNames, "🇫🇷 法国", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("GB", "", proxyNames, "🇬🇧 英国", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("NL", "", proxyNames, "🇳🇱 荷兰", countryList, conntryProxies)
		countryList, conntryProxies = GetCountryProxies("", "CN,HK,TW,US,CA,JP,SG,AU,CH,DE,GB,NL,FR,RU", proxyNames, "其他国家", countryList, conntryProxies)

		body := strings.Replace(string(content), "{{ proxies }}", proxyList, -1)
		body = strings.Replace(body, "{{ ProxyNames }}", proxyNames, -1)
		body = strings.Replace(body, "{{ CountryList }}", countryList, -1)
		body = strings.Replace(body, "{{ ConntryProxies }}", conntryProxies, -1)

		c.String(200, body)
	})

	router.GET("/clash/config2", func(c *gin.Context) {
		content, err := ioutil.ReadFile("template/clash-config2.yaml")
		if err != nil {
			log.Println("无法读取文件:", err)
		}

		proxyList, proxyNames := GetAllProxiesText()

		body := strings.Replace(string(content), "{{ Proxies }}", proxyList, -1)
		body = strings.Replace(body, "{{ ProxyNames }}", proxyNames, -1)

		c.String(200, body)
	})

	router.GET("/clash/config", func(c *gin.Context) {
		c.HTML(http.StatusOK, "assets/html/clash-config.yaml", gin.H{
			"domain":  config.Config.Domain,
			"request": config.Config.Request,
			"port":    config.Config.Port,
		})
	})
	router.GET("/clash/localconfig", func(c *gin.Context) {
		c.HTML(http.StatusOK, "assets/html/clash-config-local.yaml", gin.H{
			"port": config.Config.Port,
		})
	})
	router.GET("/clash/proxies", func(c *gin.Context) {
		proxyTypes := c.DefaultQuery("type", "")
		proxyCountry := c.DefaultQuery("c", "")
		proxyNotCountry := c.DefaultQuery("nc", "")
		proxySpeed := c.DefaultQuery("speed", "")
		proxyFilter := c.DefaultQuery("filter", "")
		text := ""
		if proxyTypes == "" && proxyCountry == "" && proxyNotCountry == "" && proxySpeed == "" && proxyFilter == "" {
			text = appcache.GetString("clashproxies") // A string. To show speed in this if condition, this must be updated after speedtest
			if text == "" {
				proxies := appcache.GetProxies("proxies")
				clash := provider.Clash{
					Base: provider.Base{
						Proxies: &proxies,
					},
				}
				text = clash.Provide() // 根据Query筛选节点
				appcache.SetString("clashproxies", text)
			}
		} else if proxyTypes == "all" {
			proxies := appcache.GetProxies("allproxies")
			clash := provider.Clash{
				provider.Base{
					Proxies:    &proxies,
					Types:      proxyTypes,
					Country:    proxyCountry,
					NotCountry: proxyNotCountry,
					Speed:      proxySpeed,
					Filter:     proxyFilter,
				},
			}
			text = clash.Provide() // 根据Query筛选节点
		} else {
			proxies := appcache.GetProxies("proxies")
			clash := provider.Clash{
				provider.Base{
					Proxies:    &proxies,
					Types:      proxyTypes,
					Country:    proxyCountry,
					NotCountry: proxyNotCountry,
					Speed:      proxySpeed,
					Filter:     proxyFilter,
				},
			}
			text = clash.Provide() // 根据Query筛选节点
		}
		c.String(200, text)
	})
	router.GET("/surge/proxies", func(c *gin.Context) {
		proxyTypes := c.DefaultQuery("type", "")
		proxyCountry := c.DefaultQuery("c", "")
		proxyNotCountry := c.DefaultQuery("nc", "")
		proxySpeed := c.DefaultQuery("speed", "")
		proxyFilter := c.DefaultQuery("filter", "")
		text := ""
		if proxyTypes == "" && proxyCountry == "" && proxyNotCountry == "" && proxySpeed == "" {
			text = appcache.GetString("surgeproxies") // A string. To show speed in this if condition, this must be updated after speedtest
			if text == "" {
				proxies := appcache.GetProxies("proxies")
				surge := provider.Surge{
					Base: provider.Base{
						Proxies: &proxies,
					},
				}
				text = surge.Provide()
				appcache.SetString("surgeproxies", text)
			}
		} else if proxyTypes == "all" {
			proxies := appcache.GetProxies("allproxies")
			surge := provider.Surge{
				Base: provider.Base{
					Proxies:    &proxies,
					Types:      proxyTypes,
					Country:    proxyCountry,
					NotCountry: proxyNotCountry,
					Speed:      proxySpeed,
					Filter:     proxyFilter,
				},
			}
			text = surge.Provide()
		} else {
			proxies := appcache.GetProxies("proxies")
			surge := provider.Surge{
				Base: provider.Base{
					Proxies:    &proxies,
					Types:      proxyTypes,
					Country:    proxyCountry,
					NotCountry: proxyNotCountry,
					Filter:     proxyFilter,
				},
			}
			text = surge.Provide()
		}
		c.String(200, text)
	})
	router.GET("/forceupdate", func(c *gin.Context) {
		err := app.InitApp()
		if err != nil {
			c.String(http.StatusOK, err.Error())
		}
		c.String(http.StatusOK, "Updated")
	})
}

func Run() {
	setupRouter()
	servePort := config.Config.Port
	envp := os.Getenv("PORT") // envp for heroku. DO NOT SET ENV PORT IN PERSONAL SERVER UNLESS YOU KNOW WHAT YOU ARE DOING
	if envp != "" {
		servePort = envp
	}
	// Run on this server
	err := router.Run(":" + servePort)
	if err != nil {
		log.Fatalf("[router.go] Web server starting failed. Make sure your port %s has not been used. \n%s", servePort, err.Error())
	}
}

// 返回页面templates
func loadHTMLTemplate() (t *template.Template, err error) {
	t = template.New("")
	for _, fileName := range AssetNames() { //fileName带有路径前缀
		if strings.Contains(fileName, "css") {
			continue
		}
		data := MustAsset(fileName)                  //读取页面数据
		t, err = t.New(fileName).Parse(string(data)) //生成带路径名称的模板
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}
