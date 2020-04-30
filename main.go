package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

const (
	bash                   = "/bin/bash"
	scriptsName            = "install.sh"
	pingOkStr              = "cmd ping ok"
	sshPortOkStr           = "cmd telnet ssh 22 ok"
	networkOperation       = "check network"
	installConsulOperation = "install Consul"
)

type Worker interface {
	localPing()
	telnetSsh()
	remoteDo()
	workData() (ip string, msg chan []string)
}

type baseWork struct {
	ip     string
	msg    []string
	result chan []string
}

func (w *baseWork) localPing() {
	r := localPing(w.ip)
	if !r {
		fmt.Println(r, "cmd ping 不通 ", w.ip)
		w.msg = append(w.msg, fmt.Sprint("cmd ping 不通 ", w.ip))
	} else {
		w.msg = append(w.msg, fmt.Sprint(pingOkStr))
	}
}

func (w *baseWork) telnetSsh() {
	r2 := telnetSsh(w.ip)
	if !r2 {
		fmt.Println(r2, "cmd telnet 不通 ", w.ip)
		w.msg = append(w.msg, fmt.Sprint("cmd telnet ssh 22 不通 ", w.ip))
	} else {
		w.msg = append(w.msg, fmt.Sprint(sshPortOkStr))
	}
}
func (w *baseWork) workData() (ip string, msg chan []string) {
	return w.ip, w.result
}

type CheckNetwork struct {
	done func()
	*baseWork
}

func (net *CheckNetwork) remoteDo() {
	if net.msg[0] == pingOkStr && net.msg[1] == sshPortOkStr {
		cmd := exec.Command(bash, scriptsName, "check", net.ip)
		bytes, _ := cmd.CombinedOutput()
		net.msg = append(net.msg, string(bytes))
	} else {
		net.msg = append(net.msg, fmt.Sprint("remote host unreachable"))
	}

	net.result <- net.msg
	net.done()
}

type remoteInstallConsul struct {
	done func()
	*baseWork
}

func (c *remoteInstallConsul) remoteDo() {
	if c.msg[0] == pingOkStr && c.msg[1] == sshPortOkStr {
		cmd := exec.Command(bash, scriptsName, "install", c.ip)
		bytes, _ := cmd.CombinedOutput()
		c.msg = append(c.msg, string(bytes))
	} else {
		c.msg = append(c.msg, fmt.Sprint("remote host unreachable"))
	}

	c.result <- c.msg
	c.done()
}

func doWork(w Worker) {
	w.localPing()
	w.telnetSsh()
	w.remoteDo()
}

func matchIpRegex(ip string) bool {
	addr := strings.Trim(ip, " ")
	regStr := `^(([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.)(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){2}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`
	if match, _ := regexp.MatchString(regStr, addr); match {
		return true
	}
	return false
}

func checkIpRight(ips []string) (ret map[string]bool) {
	fmt.Println("middle check ip...", ips, len(ips))
	ret = make(map[string]bool)
	for _, addr := range ips {
		r := matchIpRegex(addr)
		ret[addr] = r
	}
	return
}

func telnetSsh(ip string) bool {
	cmd := exec.Command(bash, scriptsName, "ssh", ip)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("telnet ssh port error ", err)
	}

	resp := string(bytes)
	fmt.Println("telnet ssh 22", resp)

	if resp != "ssh port ok" {
		return false
	} else {
		return true
	}
}

func localPing(ip string) bool {
	cmd := exec.Command(bash, scriptsName, "local", ip)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("exec local ping error ", err)
	}
	pingCtx := string(bytes)
	pingCtx = strings.Replace(pingCtx, "\n", "", -1)
	fmt.Println("local ping", pingCtx)

	if pingCtx != "localPingOk" {
		return false
	} else {
		return true
	}
}

func createWorker(ip string, wg *sync.WaitGroup,
	OperationType string) Worker {
	w := &baseWork{
		ip:     ip,
		msg:    make([]string, 0),
		result: make(chan []string),
	}

	switch OperationType {
	case networkOperation:
		return &CheckNetwork{baseWork: w, done: wg.Done}
	case installConsulOperation:
		return &remoteInstallConsul{baseWork: w, done: wg.Done}
	default:
		panic(fmt.Sprint("Undefined type ", OperationType))
	}
}

func CreateWorkAndDo(ip []string, opType string) (workers []Worker,
	wag *sync.WaitGroup) {
	var wg sync.WaitGroup
	nums := len(ip)

	for _, addr := range ip {
		work := createWorker(addr, &wg, opType)
		workers = append(workers, work)
		go doWork(work)
	}
	wg.Add(nums)
	return workers, &wg
}

func Controller(ip []string, operation string) []gin.H {
	fmt.Println(operation, len(ip), ip)
	workers, wg := CreateWorkAndDo(ip, operation)
	result := make([]gin.H, 0, len(ip))

	for _, work := range workers {
		data := make(gin.H)
		ip, msg := work.workData()
		data[ip] = <-msg
		result = append(result, data)
	}
	wg.Wait()
	return result
}

func requestCheck(c *gin.Context) {
	ipsList := c.GetStringSlice("ip")
	ret := Controller(ipsList, networkOperation)
	c.JSON(200, gin.H{"message": "检查端口 ok", "data": ret})
}

func requestInstallConsul(c *gin.Context) {
	ipsList := c.GetStringSlice("ip")
	ret := Controller(ipsList, installConsulOperation)
	c.JSON(200, gin.H{"message": "安装consul ok", "data": ret})
}

func errorIpMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var ipsList []string
		err := c.BindJSON(&ipsList)
		if err != nil {
			c.AbortWithStatusJSON(200, gin.H{"message": "error request"})
			return
		}

		fmt.Println("middle", ipsList)
		if len(ipsList) == 0 {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"message": "输入ip 为空"})
			return
		}

		ret := checkIpRight(ipsList)
		errIp := make([]string, 0, len(ipsList))
		for k, v := range ret {
			if !v {
				errIp = append(errIp, k)
			}
		}
		if len(errIp) != 0 {
			c.AbortWithStatusJSON(http.StatusOK,
				gin.H{"message": "ip不合法", "data": errIp})
			return
		}

		c.Set("ip", ipsList)
	}
}

func main() {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "SUCCESS")
	})

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "ips.html", gin.H{
			"title": "安装 consul",
		})
	})

	r.POST("/check", errorIpMiddleware(), requestCheck)
	r.POST("/install", errorIpMiddleware(), requestInstallConsul)

	r.Run("0.0.0.0:7777")
}
