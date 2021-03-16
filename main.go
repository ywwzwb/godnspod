package main

// build for ea6500v2:
// GOARM=5 GOARCH=arm GOOS=linux CGO_ENABLED=0 go build -ldflags "-w -s"
import (
	"encoding/json"
	"flag"
	"fmt"
	"godnspod/util"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
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
	var logPath string
	var logLevelStr string
	flag.StringVar(&configPath, "config_path", "", "config file path")
	flag.StringVar(&logPath, "log_path", "", "log file path, leave empty to log to stdout")
	flag.StringVar(&logLevelStr, "log_level", "", "log level: trace, debug, info, warn, error, fatal, panic. default set to info")
	flag.Parse()
	// read from env
	configPathFromEnv, ok := os.LookupEnv("config_path")
	if ok && len(configPathFromEnv) > 0 {
		configPath = configPathFromEnv
	}
	logPathFromEnv, ok := os.LookupEnv("log_path")
	if ok && len(logPathFromEnv) > 0 {
		logPath = logPathFromEnv
	}
	logLevelStrFromEnv, ok := os.LookupEnv("log_level")
	if ok && len(logLevelStrFromEnv) > 0 {
		logLevelStr = logLevelStrFromEnv
	}
	if len(configPath) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	if len(logPath) == 0 {
		fmt.Println("log path not set!, disable log")
	}
	if len(logLevelStr) == 0 {
		fmt.Println("log level not set!, set to info")
		logLevelStr = "info"
	}
	logLevel, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		fmt.Println("parse log level faild, set to info")
		logLevel = logrus.InfoLevel
	}
	util.InitLoggerWith(logLevel, logPath, 3)
	util.Logger.Info("start running")
	var conf Config
	fileBuf, err := ioutil.ReadFile(configPath)
	if err != nil {
		util.Logger.WithError(err).Fatal("read config file failed")
	}
	if err = yaml.Unmarshal(fileBuf, &conf); err != nil {
		util.Logger.WithError(err).Fatal("parse config file failed")
	}
	var lastPubIP string = ""
	var anameIP string = ""
	var recordid string = ""
	for {
		ip, err := getMyPubIP(conf.GetIPMethod)
		if err != nil {
			util.Logger.WithError(err).Error("get my public ip error:", err)
			goto sleep
		}
		util.Logger.WithField("ip", ip).Debug("my public ip")
		if lastPubIP == ip {
			util.Logger.WithField("ip", ip).Debug("public ip has no change")
			goto sleep
		}
		anameIP, recordid, err = getDomainANameRecord(conf.Subdomain, conf.Basedomain, conf.Token)
		if err != nil {
			if dnspodErr, ok := err.(dnspodError); ok && dnspodErr.code == dnspodErrorCodeRecordNotExist {
				// its ok if the domain is not exist, we can create it later
				util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).Info("domain is not exist, create it later")
			} else {
				util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).WithError(err).Error("get A name record failed")
				goto sleep
			}
		}
		if anameIP == "" {
			util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).Info("start create domain")
			if err := createDomainANameRecord(conf.Subdomain, conf.Basedomain, conf.Token, ip); err != nil {
				util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).WithError(err).Error("create domain failed")
				goto sleep
			}
			util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).Info("create domain success")
		} else {
			util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).Debug("domain A record")
			if anameIP == ip {
				util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).Debug("A record is equal to current ip")
				goto save_last_ip
			}
			if err = setDomainANameRecord(conf.Subdomain, conf.Basedomain, conf.Token, ip, recordid); err != nil {
				util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).WithError(err).Error("set domain name record failed")
				goto sleep
			}
			util.Logger.WithField("domain", fmt.Sprintf("%v.%v", conf.Subdomain, conf.Basedomain)).WithField("record", ip).Info("set domain A record success")
		}
	save_last_ip:
		lastPubIP = ip
	sleep:
		if conf.RefreshInterval > 1 {
			util.Logger.Debug("sleep for refresh")
			time.Sleep(conf.RefreshInterval * time.Second)
		} else {
			break
		}
	}
}
