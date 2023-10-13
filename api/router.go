package api

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"io/ioutil"

	"github.com/ssrlive/proxypool/pkg/provider"
	"github.com/ssrlive/proxypoolCheck/config"
	"github.com/ssrlive/proxypoolCheck/internal/app"
	appcache "github.com/ssrlive/proxypoolCheck/internal/cache"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/ssrlive/proxypool/pkg/tool"
	"github.com/ssrlive/proxypool/pkg/proxy"
)

const version = "v0.7.3"

var router *gin.Engine

var ssrObfsList = []string{
	"plain",
	"http_simple",
	"http_post",
	"random_head",
	"tls1.2_ticket_auth",
	"tls1.2_ticket_fastauth",
}

var ssrProtocolList = []string{
	"origin",
	"verify_deflate",
	"verify_sha1",
	"auth_sha1",
	"auth_sha1_v2",
	"auth_sha1_v4",
	"auth_aes128_md5",
	"auth_aes128_sha1",
	"auth_chain_a",
	"auth_chain_b",
}

var vmessCipherList = []string{
	"auto",
	"aes-128-gcm",
	"chacha20-poly1305",
	"none",
}

// 检查单个节点的加密方式、协议类型与混淆是否是Clash所支持的
func checkClashSupport(p proxy.Proxy) bool {
	switch p.TypeName() {
	case "ssr":
		ssr := p.(*proxy.ShadowsocksR)
		if tool.CheckInList(proxy.SSRCipherList, ssr.Cipher) &&
			tool.CheckInList(ssrProtocolList, ssr.Protocol) &&
			tool.CheckInList(ssrObfsList, ssr.Obfs) {
			return true
		}
	case "vmess":
		vmess := p.(*proxy.Vmess)
		if tool.CheckInList(vmessCipherList, vmess.Cipher) {
			return true
		}
	case "ss":
		ss := p.(*proxy.Shadowsocks)
		if tool.CheckInList(proxy.SSCipherList, ss.Cipher) {
			return true
		}
	case "trojan":
		return true
	default:
		return false
	}
	return false
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
		content, err := ioutil.ReadFile("resource/template/clash-config-country.yaml")
		if err != nil {
			log.Println("无法读取文件:", err)
		}

		nameList := []string{}
		var resultBuilder strings.Builder
		resultBuilder.WriteString("proxies:\n")
		countMap := make(map[string][]string)
		allProxies := appcache.GetProxies("proxies")
		for _, p := range allProxies {			
			if checkClashSupport(p) {
				country := p.BaseInfo().Country
				countMap[country] = append(countMap[country], p.BaseInfo().Name)
				nameList = append(nameList, p.BaseInfo().Name)
				resultBuilder.WriteString("    " + p.ToClash() + "\n")
			}
		}
		countryList := ""
		conntryProxies := ""
		for country, proxyName := range countMap {
			conntryProxyTemp := "    type: url-test\n    url: 'http://www.gstatic.com/generate_204'\n    interval: 3600"
			countryList += "      - " + country + "\n"
			conntryProxies += "  - name: " + country + "\n" + conntryProxyTemp + "\n    proxies: [" + strings.Join(proxyName, ", ") + "]\n"
		}
		if len(allProxies) == 0 { //如果没有proxy，添加无效的NULL节点，防止Clash对空节点的Provider报错
			resultBuilder.WriteString("- {\"name\":\"NULL\",\"server\":\"NULL\",\"port\":11708,\"type\":\"ssr\",\"country\":\"NULL\",\"password\":\"sEscPBiAD9K$\\u0026@79\",\"cipher\":\"aes-256-cfb\",\"protocol\":\"origin\",\"protocol_param\":\"NULL\",\"obfs\":\"http_simple\"}")
		}

		body := strings.Replace(string(content), "{{ proxies }}", resultBuilder.String(), -1)
		body = strings.Replace(body, "{{ ProxyNames }}", strings.Join(nameList, ", "), -1)
		body = strings.Replace(body, "{{ CountryList }}", countryList, -1)
		body = strings.Replace(body, "{{ ConntryProxies }}", conntryProxies, -1)

		c.String(200, body)
	})

	router.GET("/clash/config2", func(c *gin.Context) {
		content, err := ioutil.ReadFile("resource/template/clash-config-andy.yaml")
		if err != nil {
			log.Println("无法读取文件:", err)
		}

		nameList := []string{}
		var resultBuilder strings.Builder
		resultBuilder.WriteString("proxies:\n")
		allProxies := appcache.GetProxies("proxies")
		for _, p := range allProxies {			
			if checkClashSupport(p) {
				nameList = append(nameList, p.BaseInfo().Name)
				resultBuilder.WriteString("    " + p.ToClash() + "\n")
			}
		}
		if len(allProxies) == 0 {
			resultBuilder.WriteString("- {\"name\":\"NULL\",\"server\":\"NULL\",\"port\":11708,\"type\":\"ssr\",\"country\":\"NULL\",\"password\":\"sEscPBiAD9K$\\u0026@79\",\"cipher\":\"aes-256-cfb\",\"protocol\":\"origin\",\"protocol_param\":\"NULL\",\"obfs\":\"http_simple\"}")
		}

		body := strings.Replace(string(content), "{{ Proxies }}", resultBuilder.String(), -1)
		body = strings.Replace(body, "{{ ProxyNames }}", strings.Join(nameList, ", "), -1)

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
