package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

var num int = 8
var k int = 1
var basicIP string = "127.0.0.1:"
var IPcount int = 10000

type generateIP struct {
	ClientIPPath string `yaml:"ClientIPPath"`
	TpIPPath     string `yaml:"TpIPPath"`
	BCIPPath     string `yaml:"BCIPPath"`
	OrderIPPath  string `yaml:"OrderIPPath"`
}

func main() {
	gopath := os.Getenv("GOPATH")
	gen := &generateIP{}
	readBytes, err := ioutil.ReadFile(gopath + "/src/dumbo_fabric/config/node.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(readBytes, gen)
	if err != nil {
		panic(err)
	}
	//generate client IP
	ClientIPPath := fmt.Sprintf("%s%sip.txt", gopath, gen.ClientIPPath)
	f, err := os.OpenFile(ClientIPPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	for i := 0; i < num; i++ {
		ip := fmt.Sprintf("%s%d\n", basicIP, IPcount+1)
		IPcount++
		_, err2 := f.WriteString(ip)
		if err2 != nil {
			fmt.Printf("write err:\n%s", err2)
			return
		}
	}
	f.Close()
	//generate txpool IP
	TxpoolIPPath := fmt.Sprintf("%s%sip.txt", gopath, gen.TpIPPath)
	f, err = os.OpenFile(TxpoolIPPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	for i := 0; i < num; i++ {
		ip := fmt.Sprintf("%s%d\n", basicIP, IPcount+1)
		IPcount++
		_, err2 := f.WriteString(ip)
		if err2 != nil {
			fmt.Printf("write err:\n%s", err2)
			return
		}
	}
	f.Close()
	//generate broadcast middle IP
	for j := 1; j <= k; j++ {
		BroadcastmIPPath := fmt.Sprintf("%s%sipm%d.txt", gopath, gen.BCIPPath, j)
		f, err = os.OpenFile(BroadcastmIPPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}
		for i := 0; i < num; i++ {
			ip := fmt.Sprintf("%s%d\n", basicIP, IPcount+1)
			IPcount++
			_, err2 := f.WriteString(ip)
			if err2 != nil {
				fmt.Printf("write err:\n%s", err2)
				return
			}
		}
		f.Close()
	}
	//generate order middle IP
	OrdermIPPath := fmt.Sprintf("%s%sipm.txt", gopath, gen.OrderIPPath)
	f, err = os.OpenFile(OrdermIPPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	for i := 0; i < num; i++ {
		ip := fmt.Sprintf("%s%d\n", basicIP, IPcount+1)
		IPcount++
		_, err2 := f.WriteString(ip)
		if err2 != nil {
			fmt.Printf("write err:\n%s", err2)
			return
		}
	}
	f.Close()
	//generate broadcast IP

	for i := 1; i <= num; i++ {
		for j := 1; j <= k; j++ {
			BroadcastIPPath := fmt.Sprintf("%s%sip%d%d.txt", gopath, gen.BCIPPath, i, j)
			f, err := os.OpenFile(BroadcastIPPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				return
			}
			for m := 0; m < num; m++ {
				ip := fmt.Sprintf("%s%d\n", basicIP, IPcount+1)
				IPcount++
				_, err2 := f.WriteString(ip)
				if err2 != nil {
					fmt.Printf("write err:\n%s", err2)
					return
				}
			}
			f.Close()
		}
	}

	//generate order IP
	OrderIPPath := fmt.Sprintf("%s%sip.txt", gopath, gen.OrderIPPath)
	f, err = os.OpenFile(OrderIPPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	for i := 0; i < num; i++ {
		ip := fmt.Sprintf("%s%d\n", basicIP, IPcount+1)
		IPcount++
		_, err2 := f.WriteString(ip)
		if err2 != nil {
			fmt.Printf("write err:\n%s", err2)
			return
		}
	}
	f.Close()
}
