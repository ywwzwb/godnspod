package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"strings"
)

type GetIPMethodFunc func(GetIPMethod, bool) (string, error)

const (
	GetIPMethodDisable     string = "disable"
	GetIPMethodNvram       string = "nvram"
	GetIPMethodLanIP       string = "lanip"
	GetIPMethodAPI         string = "network_api"
	GetIPMethodStatic      string = "static"
	GetIPMethodFixedSuffix string = "fix_suffix"
)

var GetIPMethodsFuncs = map[string]GetIPMethodFunc{}

func initIPMethodMap() {
	GetIPMethodsFuncs = map[string]GetIPMethodFunc{
		GetIPMethodNvram:       getMyPubIPFromNvram,
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
	iface, err := net.InterfaceByName(method.NetworkCardName)
	if err != nil {
		return "", err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	var finalIP net.IP = nil
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip.To4() != nil {
			// ipv4
			ip = ip.To4()
			if isIPV6 {
				continue
			}
		} else {
			if !isIPV6 {
				continue
			}
			// ipv6
		}
		if finalIP == nil {
			finalIP = ip
		}
		if !ip.IsPrivate() && ip.IsGlobalUnicast() {
			return ip.String(), nil
		}
	}
	if finalIP == nil {
		return "", errors.New("no ip")
	}
	return finalIP.String(), nil
}

func getMyPubIPFromNvram(method GetIPMethod, isIPV6 bool) (string, error) {
	// var keys = make([]string, 1)
	var keys []string
	if isIPV6 {
		keys = []string{"ipv6_rtr_addr", "lan_addr6"}
	} else {
		keys = []string{"wan0_ipaddr"}
	}
	for _, key := range keys {
		cmd := exec.Command("nvram", "get", key)
		outbuf, err := cmd.CombinedOutput()
		if err != nil {
			return "", err
		}
		ipRaw := strings.TrimSpace(string(outbuf))
		if len(ipRaw) == 0 {
			continue
		}
		// lan_addr6 取出来lan地址是带有掩码长度的，例如 a:b::c:d/60
		ip := strings.Split(ipRaw, "/")[0]
		return ip, nil
	}
	return "", fmt.Errorf("no ip")
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
