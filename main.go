package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/subosito/gotenv"
	"golang.org/x/net/html"
)

type routerStat struct {
	Port             string
	Status           string
	TxPackets        int64
	RxPackets        int64
	Collisions       int64
	TxBytesPerSecond int64
	RxBytesPerSecond int64
	Uptime           time.Duration
}

func (s routerStat) IsDown() bool {
	return strings.EqualFold(s.Status, "Link Down")
}

func (s routerStat) Classifiction() string {
	if strings.Contains(strings.ToUpper(s.Port), "WLAN") {
		return "Wireless"
	}

	return "Wired"
}

func (s routerStat) Availability() string {
	if strings.HasPrefix(strings.ToUpper(s.Port), "WAN") {
		return "External"
	}

	return "Internal"
}

func (s routerStat) Name() string {
	// 2.4G WLAN b/g/n
	return strings.Replace(strings.Replace(s.Port, "\\", "\\\\", -1), " ", "\\ ", -1)
}

func (s routerStat) String() string {
	if s.IsDown() {
		return fmt.Sprintf("Port: %s; Status: %s; TxPkts: %d; RxPkts: %d; Collisions: %d; TxB/s: %d; RxB/s: %d; Uptime: %v",
			s.Port,
			s.Status,
			0,
			0,
			0,
			0,
			0,
			s.Uptime)
	}

	return fmt.Sprintf("Port: %s; Status: %s; TxPkts: %d; RxPkts: %d; Collisions: %d; TxB/s: %d; RxB/s: %d; Uptime: %v",
		s.Port,
		s.Status,
		s.TxPackets,
		s.RxPackets,
		s.Collisions,
		s.TxBytesPerSecond,
		s.RxBytesPerSecond,
		s.Uptime)
}

func (s routerStat) LineFormat(address string) string {

	if s.IsDown() {
		return fmt.Sprintf("router,address=%s,name=%s,classification=%s,availability=%s tx_packets=%di,rx_packets=%di,collisions=%di,tx_bytes_sec=%di,rx_bytes_sec=%di,status=%q,down=%v",
			address,
			s.Name(),
			s.Classifiction(),
			s.Availability(),
			0,
			0,
			0,
			0,
			0,
			s.Status,
			s.IsDown())
	}

	return fmt.Sprintf("router,address=%s,name=%s,classification=%s,availability=%s tx_packets=%di,rx_packets=%di,collisions=%di,tx_bytes_sec=%di,rx_bytes_sec=%di,status=%q,down=%v",
		address,
		s.Name(),
		s.Classifiction(),
		s.Availability(),
		s.TxPackets,
		s.RxPackets,
		s.Collisions,
		s.TxBytesPerSecond,
		s.RxBytesPerSecond,
		s.Status,
		s.IsDown())
}

func init() {
	gotenv.Load()
	workingDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	gotenv.Load(filepath.Join(workingDir, ".env"))
}

func main() {
	user := os.Getenv("ROUTER_USER")
	pass := os.Getenv("ROUTER_PASS")
	address := os.Getenv("ROUTER_ADDR")

	isDebug := strings.Contains(strings.Join(os.Args, " "), "-debug")
	isHuman := strings.Contains(strings.Join(os.Args, " "), "-human")

	statsBody, err := getStats(address, user, pass)
	if err != nil {
		panic(err)
	}

	if isDebug {
		fmt.Fprintf(os.Stderr, statsBody+"\n")
	}

	stats, err := parseStats(statsBody)
	if err != nil {
		panic(err)
	}

	for _, stat := range stats {
		if isHuman {
			fmt.Fprintf(os.Stdout, "%s\n", stat.String())
		} else {
			fmt.Fprintf(os.Stdout, "%s\n", stat.LineFormat(address))
		}

	}
}

func parseStats(input string) ([]routerStat, error) {
	stats := []routerStat{}

	tokenizer := html.NewTokenizer(strings.NewReader(input))

	rowNumber := 0
	colNumber := 0
	dataValues := []string{}
	var previousStat *routerStat

	for {
		token := tokenizer.Next()

		switch {
		case token == html.ErrorToken:
			return stats, nil
		case token == html.StartTagToken:
			tag := tokenizer.Token()

			switch tag.Data {
			case "table":
				rowNumber = 0
			case "tr":
				rowNumber++
				colNumber = 0
				dataValues = []string{}
			case "td":
				colNumber++
			case "span":
				if rowNumber == 1 {
					continue
				}

				found := false

				for _, attr := range tag.Attr {
					if strings.EqualFold(attr.Key, "class") && (strings.EqualFold(attr.Val, "thead") || strings.EqualFold(attr.Val, "ttext")) {
						found = true
					}
				}

				if !found {
					continue
				}

				// get inner data
				token = tokenizer.Next()
				if token != html.TextToken {
					continue
				}

				dataValues = append(dataValues, string(tokenizer.Raw()))
			}

		case token == html.EndTagToken:
			tag := tokenizer.Token()
			if tag.Data == "tr" {
				stat := makeStat(dataValues, previousStat)
				if stat != nil {
					stats = append(stats, *stat)
					previousStat = stat
				}
				dataValues = []string{}
			}
		}
	}
}

func parseNumber(input string) int64 {
	res, _ := strconv.ParseInt(input, 10, 64)
	return res
}

func parseDuration(input string) time.Duration {
	res, _ := time.ParseDuration(input)
	return res
}

func makeStat(input []string, previous *routerStat) *routerStat {
	if len(input) == 0 {
		return nil
	}

	stat := &routerStat{}
	stat.Port = input[0]

	switch len(input) {
	case 3:
		stat.Port = input[0]
		stat.Status = input[1]
		stat.Uptime = parseDuration(input[2])
	case 8:
		stat.Port = input[0]
		stat.Status = input[1]
		stat.TxPackets = parseNumber(input[2])
		stat.RxPackets = parseNumber(input[3])
		stat.Collisions = parseNumber(input[4])
		stat.TxBytesPerSecond = parseNumber(input[5])
		stat.RxBytesPerSecond = parseNumber(input[6])
		stat.Uptime = parseDuration(input[7])
	}

	// LAN shares input/output stats
	if previous != nil && strings.HasPrefix(previous.Port, "LAN") && strings.HasPrefix(stat.Port, "LAN") {
		stat.TxPackets = previous.TxPackets
		stat.RxPackets = previous.RxPackets
		stat.Collisions = previous.Collisions
		stat.TxBytesPerSecond = previous.TxBytesPerSecond
		stat.RxBytesPerSecond = previous.RxBytesPerSecond
	}

	return stat
}

func getStats(address, user, pass string) (string, error) {
	retried := false

	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+address+"/RST_stattbl.htm", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.SetBasicAuth(user, pass)

RETRY:
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if retried {
			return "", fmt.Errorf("unexpected response code %s", resp.Status)
		}

		// because the netgear is flaky when your "session" ends, we may need to make two requests to trigger the auto correctly
		retried = true
		goto RETRY
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	return string(body), nil
}
