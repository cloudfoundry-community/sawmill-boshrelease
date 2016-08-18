package sawmill

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/syslog"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testConfig struct {
	Servers       []string `json:"servers"`
	SyslogPort    int      `json:"syslog_port"`
	WebPort       int      `json:"web_port"`
	UserName      string   `json:"username"`
	Password      string   `json:"password"`
	SkipSSLVerify bool     `json:"skip_ssl_verify"`
}

func loadConfig() testConfig {
	path := os.Getenv("CONFIG_PATH")
	configFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	var cfg testConfig
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

var config = loadConfig()

var _ = Describe("Sawmill Log Streams", func() {
	var curls []*exec.Cmd
	loggers := map[string]*syslog.Writer{}

	BeforeEach(func() {
		var skipSSLVerify string
		if config.SkipSSLVerify {
			skipSSLVerify = "-k"
		}
		for _, server := range config.Servers {
			cmd := exec.Command("/bin/bash", "-c", fmt.Sprintf("curl %s -u %s:%s -s -v --no-buffer https://%s:%d > /var/vcap/data/tmp/sawmill-%s-out.log 2>&1", skipSSLVerify, config.UserName, config.Password, server, config.WebPort, server))
			err := cmd.Start()
			Expect(err).ShouldNot(HaveOccurred())
			curls = append(curls, cmd)

			// make sure curl is connected before moving on, so that we don't accidentally send
			// our log message before curl is fully connected - relies on the `-v` flag of curl
			Eventually(func() (string, error) {
				data, err := ioutil.ReadFile(fmt.Sprintf("/var/vcap/data/tmp/sawmill-%s-out.log", server))
				if err != nil {
					return "", err
				}
				return string(data), nil
			}, 10*time.Second, 100*time.Millisecond).Should(ContainSubstring("Accept: */*"))

			loggers[server], err = syslog.Dial("tcp", fmt.Sprintf("%s:%d", server, config.SyslogPort), syslog.LOG_INFO, fmt.Sprintf("SawmillSmokeTests-to-%s", server))
			Expect(err).ShouldNot(HaveOccurred())
		}
	})

	AfterEach(func() {
		for _, cmd := range curls {
			err := cmd.Process.Kill()
			Expect(err).ShouldNot(HaveOccurred())
		}
		for _, writer := range loggers {
			err := writer.Close()
			Expect(err).ShouldNot(HaveOccurred())
		}
		for _, server := range config.Servers {
			err := os.Remove(fmt.Sprintf("/var/vcap/data/tmp/sawmill-%s-out.log", server))
			Expect(err).ShouldNot(HaveOccurred())
		}
	})

	It("Streams log data inbound to any node out through every node", func() {
		for logDest, logger := range loggers {
			err := logger.Info(fmt.Sprintf("SawmillSmokeTests-to-%s", logDest))
			Expect(err).ShouldNot(HaveOccurred())
			for _, server := range config.Servers {
				slurpLog := func() (string, error) {
					data, err := ioutil.ReadFile(fmt.Sprintf("/var/vcap/data/tmp/sawmill-%s-out.log", server))
					if err != nil {
						return "", err
					}
					return string(data), nil
				}

				Eventually(slurpLog, time.Second*10, time.Millisecond*100).Should(ContainSubstring(fmt.Sprintf("SawmillSmokeTests-to-%s", logDest)))
			}
		}
	})
})
