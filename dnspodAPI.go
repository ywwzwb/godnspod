package main

import (
	"encoding/json"
	"fmt"
	"godnspod/util"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sirupsen/logrus"
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
func getDomainRecord(recordType RecordType, subdomain string, domain string, token string) (IPAddress string, recordid string, err error) {
	params := make(url.Values)
	params["login_token"] = []string{token}
	params["format"] = []string{"json"}
	params["lang"] = []string{"cn"}
	params["record_type"] = []string{string(recordType)}
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
	IPAddress = firstRecord["value"].(string)
	return
}
func createDomainRecord(recordType RecordType, subdomain string, domain string, token string, IPAddress string) (err error) {
	params := make(url.Values)
	params["login_token"] = []string{token}
	params["format"] = []string{"json"}
	params["lang"] = []string{"cn"}
	params["record_type"] = []string{string(recordType)}
	params["domain"] = []string{domain}
	params["sub_domain"] = []string{subdomain}
	params["value"] = []string{IPAddress}
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
func setDomainRecord(recordType RecordType, subdomain string, domain string, token string, IPAddress string, recordid string) (err error) {
	params := make(url.Values)
	params["login_token"] = []string{token}
	params["format"] = []string{"json"}
	params["lang"] = []string{"cn"}
	params["record_type"] = []string{string(recordType)}
	params["domain"] = []string{domain}
	params["sub_domain"] = []string{subdomain}
	params["value"] = []string{IPAddress}
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

func UpdateRecord(recordType RecordType, subdomain string, domain string, token string, IPAddress string) error {
	currentIP := net.IP(IPAddress)
	if !currentIP.IsGlobalUnicast() {
		return fmt.Errorf("ip address:%v is not a global unicast address", IPAddress)
	}
	recordIP, recordid, err := getDomainRecord(recordType, subdomain, domain, token)
	log := util.Logger.WithFields(logrus.Fields{
		"domain":      fmt.Sprintf("%v.%v", subdomain, domain),
		"record_type": recordType})
	if err != nil {
		if dnspodErr, ok := err.(dnspodError); ok && dnspodErr.code == dnspodErrorCodeRecordNotExist {
			// its ok if the domain is not exist, we can create it later
			log.Info("domain is not exist, create it later")
		} else {
			log.WithError(err).Error("get record failed")
			return err
		}
	}
	if recordIP == "" {
		log.Info("start create domain")
		if err := createDomainRecord(recordType, subdomain, domain, token, IPAddress); err != nil {
			log.WithError(err).Error("create domain failed")
			return err
		}
		log.WithField("ip", IPAddress).Info("create domain success")
		return nil
	}
	log.WithField("current record", recordIP).Debug("current record address")
	if currentIP.Equal(net.IP(recordIP)) {
		log.Debug("record is equal to current ip address")
		return nil
	}
	log.WithFields(logrus.Fields{
		"current record": recordIP,
		"new record":     IPAddress}).Debug("start set record address")
	if err = setDomainRecord(recordType, subdomain, domain, token, IPAddress, recordid); err != nil {
		log.WithError(err).Error("set domain name record failed")
		return err
	}
	log.WithField("record", IPAddress).Info("set domain record success")
	return nil
}
