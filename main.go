package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-redis/redis"
)

type conf struct {
	Redis struct {
		Addr     string
		Password string
		DB       int
	}
	Callbacks []*struct {
		Sub  string
		Urls []string
	}
}

var callbackMap = make(map[string][]string)
var httpChMap = make(map[string]chan string)

func main() {
	confPath := getCurrentPath() + "conf.json"
	data, err := ioutil.ReadFile(confPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	conf := new(conf)
	err = json.Unmarshal(data, &conf)
	client := redis.NewClient(&redis.Options{
		Addr:        conf.Redis.Addr,
		DB:          conf.Redis.DB,
		Password:    conf.Redis.Password,
		ReadTimeout: time.Second * 5,
	})

	subs := make([]string, len(conf.Callbacks))
	for i, cb := range conf.Callbacks {
		callbackMap[cb.Sub] = cb.Urls
		subs[i] = cb.Sub
		httpChMap[cb.Sub] = make(chan string, 1024)
		go func(ch <-chan string) {
			for url := range ch {
				// fmt.Println(u)
				_, err = http.Get(url)
				if err != nil {
					fmt.Println(err)
					continue
				}
			}
		}(httpChMap[cb.Sub])
	}

	_, err = client.Ping().Result()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("连接", conf.Redis.Addr, "成功")
	fmt.Println("准备监听：", subs)
	sub := client.Subscribe(subs...)
	defer sub.Close()

	// go testPub(client, subs[0])

	for msg := range sub.Channel() {
		// fmt.Printf("%+v\n", *msg)
		if urls, ok := callbackMap[msg.Channel]; ok {
			for _, url := range urls {
				u := strings.Replace(url, "{{msg}}", msg.String(), -1)
				// fmt.Println(u)
				httpChMap[msg.Channel] <- u
			}
		}
	}
}

func getCurrentPath() string {
	s, err := exec.LookPath(os.Args[0])
	if err != nil {
		panic(err)
	}
	i := strings.LastIndex(s, "\\")
	path := string(s[0 : i+1])
	return path
}

func testPub(client *redis.Client, sub string) {
	go func() {
		for {
			time.Sleep(time.Second * 3)
			client.Publish(sub, "World")
			fmt.Println("Pub to", sub)
		}
	}()
}
