package main

import (
	"fmt"
	"os"
)

func main() {
	for i := 1; i <= 4; i++ {
		for j := 1; j <= 4; j++ {
			gopath := os.Getenv("GOPATH")
			path := fmt.Sprintf(gopath+"/src/dumbo_fabric/config/IP/broadcast/ip%d%d.txt", i, j)

			f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777) //linux 路径
			/*f, err := os.OpenFile("D:/tmp/logs/test.log", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)*/ //windows路径
			if err != nil {
				fmt.Printf("open err%s", err)
				return
			}
			defer f.Close() //资源必须释放,函数刚要返回之前延迟执行

			for k := 1; k <= 4; k++ {
				ip := fmt.Sprintf("127.0.0.1:%d30%d%d\n", k, i, j)
				_, err2 := f.WriteString(ip)
				if err2 != nil {
					fmt.Printf("write err:\n%s", err2)
					return
				}
			}
		}
	}

}
