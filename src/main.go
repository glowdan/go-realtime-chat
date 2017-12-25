package main

import (
	"flag"
	"log"
	"github.com/glowdan/mogo"
	"fmt"
	"runtime"
	"os"
)

func main() {
	flag.Parse()
	conf, err := mogo.ParseConf()

	if err != nil {
		log.Fatal(err)
	}
	localIp, err := mogo.GetLocalIp()
	if err != nil {
		log.Fatal(err)
	}
	// 注册到LOOKUP服务
	mogo.RegisterMoaService(&mogo.MoaRegConfig{
		ServiceUri: fmt.Sprintf("/service/prism/%s", conf.Namespace),
		HostPort:   fmt.Sprintf("%s:%d", localIp, conf.DebugPort),
		// redis是udp memcache是tcp
		Protocol: "redis",
	})
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.Fatal(mogo.NewServer(conf, os.Stdout).Listen(nil, nil, nil))
}
