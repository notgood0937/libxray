package libxray

//go mod init libxray
//go mod tidy
//go install golang.org/x/mobile/cmd/gomobile@latest
//gomobile init
//go get -d golang.org/x/mobile/cmd/gomobile
//gomobile bind -target ios
//gomobile bind -target macos

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/xtls/xray-core/common/cmdarg"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/core"
	_ "github.com/xtls/xray-core/main/distro/all"
)

var (
	coreServer *core.Instance
)

const (
	pingDelayTimeout int64 = 11000
	pingDelayError   int64 = 10000
)

func startXray(configFile string) (*core.Instance, error) {
	file := cmdarg.Arg{configFile}
	config, err := core.LoadConfig("json", file)
	if err != nil {
		return nil, err
	}

	server, err := core.New(config)
	if err != nil {
		return nil, err
	}

	return server, nil
}

func initEnv(datDir string, maxMemory string) {
	os.Setenv("xray.location.asset", datDir)
	if maxMemory != "" {
		os.Setenv("GOMEMLIMIT", maxMemory)
	}
}

func RunXray(datDir string, config string, maxMemory string) string {
	initEnv(datDir, maxMemory)
	coreServer, err := startXray(config)
	if err != nil {
		return err.Error()
	}

	if err := coreServer.Start(); err != nil {
		return err.Error()
	}

	runtime.GC()
	return ""
}

func StopXray() string {
	if coreServer != nil {
		err := coreServer.Close()
		coreServer = nil
		if err != nil {
			return err.Error()
		}
	}
	return ""
}

func XrayVersion() string {
	return core.Version()
}

func Ping(datDir string, config string, timeout int, url string) int64 {
	initEnv(datDir, "")
	server, err := startXray(config)
	if err != nil {
		return pingDelayError
	}

	if err := server.Start(); err != nil {
		return pingDelayError
	}
	defer server.Close()

	delay := measureDelay(server, time.Second*time.Duration(timeout), url)
	return delay
}

func measureDelay(inst *core.Instance, timeout time.Duration, url string) int64 {
	c, err := coreHTTPClient(inst, timeout)
	if err != nil {
		return pingDelayError
	}
	delaySum := int64(0)
	count := int64(0)
	times := 3
	isValid := false
	for i := 0; i < times; i++ {
		delay := coreHTTPRequest(c, url)
		if delay != pingDelayTimeout {
			delaySum += delay
			count += 1
			isValid = true
		}
	}
	if !isValid {
		return pingDelayTimeout
	}
	return delaySum / count
}

func coreHTTPClient(inst *core.Instance, timeout time.Duration) (*http.Client, error) {
	if inst == nil {
		return nil, errors.New("core instance nil")
	}

	tr := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dest, err := xnet.ParseDestination(fmt.Sprintf("%s:%s", network, addr))
			if err != nil {
				return nil, err
			}
			return core.Dial(ctx, inst, dest)
		},
	}

	c := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	return c, nil
}

func coreHTTPRequest(c *http.Client, url string) int64 {
	start := time.Now()
	req, _ := http.NewRequest("GET", url, nil)
	_, err := c.Do(req)
	if err != nil {
		return pingDelayTimeout
	}
	return time.Since(start).Milliseconds()
}

func CustomUUID(str string) string {
	id, err := uuid.ParseString(str)
	if err != nil {
		return err.Error()
	}
	return id.String()
}

// https://github.com/phayes/freeport/blob/master/freeport.go
// GetFreePort asks the kernel for free open ports that are ready to use.
func GetFreePorts(count int) string {
	var ports []int
	for i := 0; i < count; i++ {
		addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			return ""
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return ""
		}
		defer l.Close()
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return strings.Trim(strings.Join(strings.Fields(fmt.Sprint(ports)), ":"), "[]")
}
