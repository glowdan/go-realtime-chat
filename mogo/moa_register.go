package mogo

import (
	"fmt"
	"errors"
	"net/http"
	"log"
	"io/ioutil"
	"os"
	"net"
	"net/url"
)

type MoaRegConfig struct {
	ServiceUri string            `json:"serviceUri"`
	HostPort   string            `json:"hostPort"`
	Protocol   string            `json:"protocol"`
	Config     map[string]string `json:"config"`
}

const MOA_REG_SERVER_URI = "http://moa005.m6:10021/register_service"

func buildUrl(regReq *MoaRegConfig) string {
	u, err := url.Parse(MOA_REG_SERVER_URI)
	if err != nil {
		log.Fatal(err)
	}
	q := u.Query()
	q.Set("service_uri", regReq.ServiceUri)
	q.Set("hostport", regReq.HostPort)
	q.Set("protocol", regReq.Protocol)
	q.Set("timeout", "1000")
	u.RawQuery = q.Encode()
	fmt.Println(u)
	return u.String()
}

//取消注册服务
func UnRegisterMoaService(regReq *MoaRegConfig) error {
	return nil
}

func RegisterMoaService(regReq *MoaRegConfig) error {
	aurl := buildUrl(regReq)
	fmt.Printf("Register moa service:%s\n", aurl)
	response, err := http.Get(aurl)
	if err != nil {
		log.Fatal(err)
	}
	msgbyte, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[debug] response:%s\n", string(msgbyte[:]))
	result := string(msgbyte[:])
	switch result {
	case "SUCCESS":
		return nil
	case "INVALID":
		fallthrough
	case "ISOLATED":
		fallthrough
	case "INTERNAL_ERROR":
		fallthrough
	default:
		return errors.New("Moa service register failed: " + result)
	}
}

func GetLocalIp() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		fmt.Printf("Oops: %v\n", err)
		return "", err
	}

	addrs, err := net.LookupHost(name)
	if err != nil {
		fmt.Printf("Oops: %v\n", err)
		return "", err
	}

	for _, localIp := range addrs {
		return localIp, nil
	}
	return "", errors.New("Can not get the localhost IP.")
}
