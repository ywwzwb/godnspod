package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

type GetIPMethodFunc func(GetIPMethod, bool) (string, error)

const (
	GetIPMethodDisable     string = "disable"
	GetIPMethodWanIP       string = "wanip"
	GetIPMethodLanIP       string = "lanip"
	GetIPMethodAPI         string = "network_api"
	GetIPMethodStatic      string = "static"
	GetIPMethodFixedSuffix string = "fix_suffix"
)

var GetIPMethodsFuncs = map[string]GetIPMethodFunc{}

func initIPMethodMap() {
	GetIPMethodsFuncs = map[string]GetIPMethodFunc{
		GetIPMethodWanIP:       getMyPubIPFromWanIP,
		GetIPMethodLanIP:       getMyPubIPFromLanIP,
		GetIPMethodAPI:         getMyPubIPFromNetworkAPI,
		GetIPMethodStatic:      getMyPubIPFromStaticIP,
		GetIPMethodFixedSuffix: getMyPubIPFromFixedSuffix,
	}
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
func getMyPubIPFromStaticIP(method GetIPMethod, isIPV6 bool) (string, error) {
	return method.Address, nil
}
func getMyPubIPFromFixedSuffix(method GetIPMethod, isIPV6 bool) (string, error) {
	if method.PrefixMethod == nil {
		return "", errors.New("prefix method nil")
	}
	if method.PrefixMethod.Method == GetIPMethodFixedSuffix {
		return "", errors.New("bad method, recursive call")
	}
	var prefixIPStr string
	if getIPFunc, ok := GetIPMethodsFuncs[method.PrefixMethod.Method]; !ok {
		return "", fmt.Errorf("unkonwn method:%v", method.PrefixMethod.Method)
	} else {
		var err error
		prefixIPStr, err = getIPFunc(*method.PrefixMethod, isIPV6)
		if err != nil {
			return "", fmt.Errorf("get prefix error:%v", err)
		}
	}
	prefixIP := net.ParseIP(prefixIPStr)
	bitlen := 8 * net.IPv4len
	if isIPV6 {
		bitlen = 8 * net.IPv6len
	}
	netmask := net.CIDRMask(method.PrefixLength, bitlen)
	if netmask == nil {
		return "", fmt.Errorf("create network mask failed")
	}
	maskedIP := prefixIP.Mask(netmask)
	suffixIP := net.ParseIP(method.Suffix)
	if !isIPV6 {
		suffixIP = suffixIP.To4()
	}
	var finalIP = net.IPv4zero.To4()
	if isIPV6 {
		finalIP = net.IPv6zero
	}
	len := net.IPv4len
	if isIPV6 {
		len = net.IPv6len
	}
	for i := 0; i < len; i++ {
		finalIP[i] = suffixIP[i] | maskedIP[i]
	}
	return finalIP.String(), nil
}