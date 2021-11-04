package main

// build for ea6500v2:
// GOARM=5 GOARCH=arm GOOS=linux CGO_ENABLED=0 go build -ldflags "-w -s"
import (
	"errors"
	"flag"
	"fmt"
	"godnspod/util"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Config is reading from config.yaml
type Config struct {
	GetIPV4Methods  []GetIPMethod `yaml:"get_ipv4_method"`
	GetIPV6Methods  []GetIPMethod `yaml:"get_ipv6_method"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	Token           string        `yaml:"token"`
	Basedomain      string        `yaml:"basedomain"`
	Subdomain       string        `yaml:"subdomain"`
}
type GetIPMethod struct {
	Method          string `yaml:"method"`
	NetworkCardName string `yaml:"networkcard,omitempty"`
	Api             string `yaml:"api,omitempty"`
	Regex           string `yaml:"regex,omitempty"`
	CustomHead      string `yaml:"custom_head,omitempty"`
}

type RecordType string

const (
	RecordTypeAName    RecordType = "A"
	RecordTypeAAAAName RecordType = "AAAA"
)

func refresh(conf Config) {
	func() {
		// ipv4
		ipv4 := ""
	getipv4loop:
		for _, method := range conf.GetIPV4Methods {

			switch method.Method {
			case GetIPMethodDisable:
				util.Logger.Info("ipv4 is disabled")
				return
			default:
				log := util.Logger.WithField("method", method.Method)
				log.Debug("start get ipv4 address")
				if getIPFunc, ok := GetIPMethodsFuncs[method.Method]; ok {
					_ipv4, err := getIPFunc(method, false)
					if net.ParseIP(_ipv4) == nil {
						log.WithField("ip", _ipv4).Error("bad ip address")
						continue
					}
					if len(_ipv4) == 0 && err == nil {
						err = errors.New("empty ip address")
					}
					if err != nil {
						log.WithError(err).Error("get ipv4 address failed")
						continue
					}
					log.WithField("ip", _ipv4).Debug("get ipv4 address done")
					// if get ip address, then break out
					ipv4 = _ipv4
					break getipv4loop
				}
			}
			// set
		}
		if len(ipv4) == 0 {
			util.Logger.Error("no ipv4 address")
			return
		}
		if err := UpdateRecord(RecordTypeAName, conf.Subdomain, conf.Basedomain, conf.Token, ipv4); err != nil {
			util.Logger.WithError(err).Error("update a name failed")
		} else {
			util.Logger.Info("update a name succeed")
		}
	}()
	func() {
		// ipv6
		ipv6 := ""
	getipv6loop:
		for _, method := range conf.GetIPV6Methods {
			switch method.Method {
			case GetIPMethodDisable:
				util.Logger.Info("ipv6 is disabled")
				return
			default:
				log := util.Logger.WithField("method", method.Method)
				log.Debug("start get ipv6 address")
				if getIPFunc, ok := GetIPMethodsFuncs[method.Method]; ok {
					_ipv6, err := getIPFunc(method, true)
					if net.ParseIP(_ipv6) == nil {
						log.WithField("ip", _ipv6).Error("bad ip address")
						continue
					}
					if len(_ipv6) == 0 && err == nil {
						err = errors.New("empty ip address")
					}
					if err != nil {
						log.WithError(err).Error("get ipv6 address failed")
						continue
					}
					log.WithField("ip", _ipv6).Debug("get ipv6 address done")
					// if get ip address, then break out
					ipv6 = _ipv6
					break getipv6loop
				}
			}
			// set
		}
		if len(ipv6) == 0 {
			util.Logger.Error("no ipv6 address")
			return
		}
		if err := UpdateRecord(RecordTypeAAAAName, conf.Subdomain, conf.Basedomain, conf.Token, ipv6); err != nil {
			util.Logger.WithError(err).Error("update aaaa name failed")
		} else {
			util.Logger.Info("update aaaa name succeed")
		}
	}()
}
func main() {
	var configPath string
	var logPath string
	var logLevelStr string
	flag.StringVar(&configPath, "config_path", "", "config file path")
	flag.StringVar(&logPath, "log_path", "", "log file path, leave empty will log to stdout")
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
	for {
		refresh(conf)
		if conf.RefreshInterval > 1 {
			util.Logger.Debug("sleep for refresh")
			time.Sleep(conf.RefreshInterval * time.Second)
		} else {
			break
		}
	}
}
