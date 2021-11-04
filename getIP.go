package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

type GetIPMethodFunc func(GetIPMethod, bool) (string, error)

const (
	GetIPMethodDisable string = "disable"
	GetIPMethodWanIP   string = "wanip"
	GetIPMethodLanIP   string = "lanip"
	GetIPMethodAPI     string = "network_api"
)

var GetIPMethodsFuncs = map[string]GetIPMethodFunc{
	GetIPMethodWanIP: getMyPubIPFromWanIP,
	GetIPMethodLanIP: getMyPubIPFromLanIP,
	GetIPMethodAPI:   getMyPubIPFromNetworkAPI,
}

func getMyPubIPFromNetworkAPI(method GetIPMethod, isIPV6 bool) (string, error) {
	resp, err := http.Get(method.Api)
	if err != nil {
		return "", err
	}
	ipbuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	ipStr := strings.TrimSpace(string(ipbuf))
	return ipStr, nil
}
func getMyPubIPFromLanIP(method GetIPMethod, isIPV6 bool) (string, error) {
	if len(method.NetworkCardName) == 0 {
		return "", errors.New("lan ip must specific a network card name")
	}
	var cmdStr string
	if isIPV6 {
		if runtime.GOOS == "darwin" {
			cmdStr = fmt.Sprintf(`ifconfig %v | awk '/inet6 [^f].+(([0-9]+)|(secured)) ?$/ {print $2}'`, method.NetworkCardName)
		} else {
			cmdStr = fmt.Sprintf(`ifconfig %v | awk '/inet6 addr:.*Scope:Global/ { gsub(/\/.*$/, "",$3);print $3}'`, method.NetworkCardName)
		}
	} else {
		if runtime.GOOS == "darwin" {
			cmdStr = fmt.Sprintf(`ifconfig %v | awk '/inet / {print $2}'`, method.NetworkCardName)
		} else {
			cmdStr = fmt.Sprintf(`ifconfig %v | awk '/inet addr/ {gsub("addr:", "", $2); print $2}'`, method.NetworkCardName)
		}
	}
	cmd := exec.Command("sh", "-c", cmdStr)
	outbuf, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(outbuf)), nil
}
func getMyPubIPFromWanIP(method GetIPMethod, isIPV6 bool) (string, error) {
	if isIPV6 {
		return "", errors.New("wan ip not support ipv6")
	}
	cmd := exec.Command("nvram", "get", "wan0_ipaddr")
	outbuf, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(outbuf)), nil
}
