package main

// build for ea6500v2:
// GOARM=5 GOARCH=arm GOOS=linux CGO_ENABLED=0 go build -ldflags "-w -s"
import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// Config is reading from config.yaml
type Config struct {
	GetIPMethod     string        `yaml:"get_ip_method"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	Token           string        `yaml:"token"`
	Basedomain      string        `yaml:"basedomain"`
	Subdomain       string        `yaml:"subdomain"`
}

const (
	getIPMethodWanIP string = "wanip"
	getIPMethodCIP   string = "ip.cip.cc"
)

const (
	dnspodErrorCodeSuccess        int = 1
	dnspodErrorCodeRecordNotExist int = 10
)

type dnspodError struct {
	msg  string
	code int
}

func (e dnspodError) Error() string {
	return fmt.Sprintf("get domain error, code:%v, msg:%v", e.code, e.msg)
}

func getMyPubIPFromCIP() (string, error) {
	resp, err := http.Get("http://ip.cip.cc/")
	if err != nil {
		return "", err
	}
	ipbuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ipbuf)), nil
}
func getMyPubIP(method string) (ip string, err error) {
	switch method {
	case getIPMethodWanIP:
		cmd := exec.Command("nvram", "get", "wan0_ipaddr")
		outbuf, err := cmd.CombinedOutput()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(outbuf)), nil
	case getIPMethodCIP:
		return getMyPubIPFromCIP()
	default:
		return "", fmt.Errorf("unknown method:%v", method)
	}
}
func getDomainANameRecord(subdomain string, domain string, token string) (ip string, recordid string, err error) {
	params := make(url.Values)
	params["login_token"] = []string{token}
	params["format"] = []string{"json"}
	params["lang"] = []string{"cn"}
	params["record_type"] = []string{"A"}
	params["domain"] = []string{domain}
	params["sub_domain"] = []string{subdomain}
	resp, err := http.PostForm("https://dnsapi.cn/Record.List", params)
	if err != nil {
		return
	}
	var result map[string]interface{}
	respBuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if err = json.Unmarshal(respBuf, &result); err != nil {
		return
	}
	status := result["status"].(map[string]interface{})
	code, _ := strconv.Atoi(status["code"].(string))
	msg := status["message"].(string)
	if code != dnspodErrorCodeSuccess {
		err = dnspodError{code: code, msg: msg}
		return
	}
	records := result["records"].([]interface{})
	if len(records) == 0 {
		err = dnspodError{code: dnspodErrorCodeRecordNotExist, msg: "can not found record"}
		return
	}
	firstRecord := records[0].(map[string]interface{})
	recordid = firstRecord["id"].(string)
	ip = firstRecord["value"].(string)
	return
}
func createDomainANameRecord(subdomain string, domain string, token string, ip string) (err error) {
	params := make(url.Values)
	params["login_token"] = []string{token}
	params["format"] = []string{"json"}
	params["lang"] = []string{"cn"}
	params["record_type"] = []string{"A"}
	params["domain"] = []string{domain}
	params["sub_domain"] = []string{subdomain}
	params["value"] = []string{ip}
	params["record_line"] = []string{"默认"}
	resp, err := http.PostForm("https://dnsapi.cn/Record.Create", params)
	if err != nil {
		return
	}
	var result map[string]interface{}
	respBuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if err = json.Unmarshal(respBuf, &result); err != nil {
		return
	}
	status := result["status"].(map[string]interface{})
	code, _ := strconv.Atoi(status["code"].(string))
	msg := status["message"].(string)
	if code != dnspodErrorCodeSuccess {
		return dnspodError{code: code, msg: msg}
	}
	return nil
}
func setDomainANameRecord(subdomain string, domain string, token string, ip string, recordid string) (err error) {
	params := make(url.Values)
	params["login_token"] = []string{token}
	params["format"] = []string{"json"}
	params["lang"] = []string{"cn"}
	params["record_type"] = []string{"A"}
	params["domain"] = []string{domain}
	params["sub_domain"] = []string{subdomain}
	params["value"] = []string{ip}
	params["record_line"] = []string{"默认"}
	params["record_id"] = []string{recordid}
	resp, err := http.PostForm("https://dnsapi.cn/Record.Modify", params)
	if err != nil {
		return
	}
	var result map[string]interface{}
	respBuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if err = json.Unmarshal(respBuf, &result); err != nil {
		return
	}
	status := result["status"].(map[string]interface{})
	code, _ := strconv.Atoi(status["code"].(string))
	msg := status["message"].(string)
	if code != dnspodErrorCodeSuccess {
		return dnspodError{code: code, msg: msg}
	}
	return nil
}
func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "", "config file path")
	flag.Parse()
	if configPath == "" {
		fmt.Println("config file path empty")
		flag.Usage()
		os.Exit(1)
	}
	var conf Config
	fileBuf, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal("read config file failed:", err)
	}
	if err = yaml.Unmarshal(fileBuf, &conf); err != nil {
		log.Fatal("parse config file failed:", err)
	}
	var lastPubIP string = ""
	var anameIP string = ""
	var recordid string = ""
	for {
		ip, err := getMyPubIP(conf.GetIPMethod)
		if err != nil {
			log.Println("get my public ip error:", err)
			goto sleep
		}
		log.Println("my public ip:", ip)
		if lastPubIP == ip {
			log.Println("public ip has no change")
			goto sleep
		}
		anameIP, recordid, err = getDomainANameRecord(conf.Subdomain, conf.Basedomain, conf.Token)
		if err != nil {
			if dnspodErr, ok := err.(dnspodError); ok && dnspodErr.code == dnspodErrorCodeRecordNotExist {
				// its ok if the domain is not exist, we can create it later
				log.Printf("%v.%v is not exist yet, create it later", conf.Subdomain, conf.Basedomain)
			} else {
				log.Println("get A name record error:", err)
				goto sleep
			}
		}
		if anameIP == "" {
			log.Println("start create record")
			if err := createDomainANameRecord(conf.Subdomain, conf.Basedomain, conf.Token, ip); err != nil {
				log.Println("create domain name record failed: ", err)
				goto sleep
			}
			log.Println("create domain record success")
		} else {
			log.Println("A name record is:", anameIP)
			if anameIP == ip {
				log.Println("A name record is equal to current ip")
				goto save_last_ip
			}
			if err = setDomainANameRecord(conf.Subdomain, conf.Basedomain, conf.Token, ip, recordid); err != nil {
				log.Println("set domain name record failed: ", err)
				goto sleep
			}
			log.Println("set domain record success")
		}
	save_last_ip:
		lastPubIP = ip
	sleep:
		if conf.RefreshInterval > 1 {
			log.Printf("sleep %v for refresh\n", conf.RefreshInterval*time.Second)
			time.Sleep(conf.RefreshInterval * time.Second)
		} else {
			break
		}
	}
}
