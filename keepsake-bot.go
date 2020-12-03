package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"gopkg.in/irc.v3"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func MakeRequest(searchUrl, searchKey string) string {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify : true},
	}
	http := &http.Client{Transport: tr}
	resp, err := http.Get(searchUrl + url.QueryEscape(searchKey))
	if err != nil {
		log.Fatalln(err)
	}
    defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	return string(body)
}

var (
	host string
	ircServer = "irc.freenode.net:6667"
)

func LookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func init() {
	flag.StringVar(&host, "host", LookupEnvOrString("KEEPSAKE_HOST", host), "Keepsake HTTPS/HTTP Server (example: https://keepsake.example.com)")
	flag.StringVar(&ircServer, "irc-server", LookupEnvOrString("IRC_SERVER", ircServer), "IRC Server and port")
	flag.Parse()
}


func main() {
	var searchUrl = host + "/pages/search/raw/?s=1&q="
	var host = host + "/pages/view/"
	var search = host + "/pages/search/?q="
	conn, err := net.Dial("tcp", ircServer)
	if err != nil {
		log.Fatalln(err)
	}

	config := irc.ClientConfig{
		Nick: "keepsake",
		User: "keepsake",
		Name: "Keepsake Bot",
		Handler: irc.HandlerFunc(func(c *irc.Client, m *irc.Message) {
			if m.Command == "INVITE" {
				channel := len(m.Params)-1
				c.Write("JOIN " + m.Params[channel])
			} else if m.Command == "PRIVMSG" && c.FromChannel(m) {
				if strings.HasPrefix(m.Params[1], "!searchkey") {
					searchKey := strings.Split(m.Trailing(), " ")[1:]
					var title string
					resp := MakeRequest(searchUrl, strings.Join(searchKey, " "))

					matchedLine := regexp.MustCompile(`^>> Matched (Article|Title): .*$`)
					endpointLine := regexp.MustCompile(`^(For more information please curl endpoint at /pages/view/raw/|For more please visit /pages/view/)`)
					scanner := bufio.NewScanner(strings.NewReader(resp))
					count := 0
					reply := make(map[string]string)
					for scanner.Scan() {
						if matchedLine.MatchString(scanner.Text()) {
							count += 1
							title = strings.Join(strings.Split(scanner.Text(), ": ")[1:], "")
						}
						if endpointLine.MatchString(scanner.Text()) {
							ss := strings.Split(scanner.Text(), "/")
							n := ss[len(ss)-1]
							reply[title] = host + n
						}
					}
					if len(reply) == 0 {
						c.WriteMessage(&irc.Message{
							Command: "PRIVMSG",
							Params: []string{
								m.Params[0],
								strconv.Itoa(count) + " matches found.",
							},
						})
					} else if len(reply) == 1 {
						c.WriteMessage(&irc.Message{
							Command: "PRIVMSG",
							Params: []string{
								m.Params[0],
								strconv.Itoa(count) + " match found.",
							},
						})
						c.WriteMessage(&irc.Message{
							Command: "PRIVMSG",
							Params: []string{
								m.Params[0],
								title + " | " + reply[title],
							},
						})
					} else if len(reply) >= 2 {
						c.WriteMessage(&irc.Message{
							Command: "PRIVMSG",
							Params: []string{
								m.Params[0],
								strconv.Itoa(count) + " matches found.",
							},
						})
						x := 0
						for key, value := range reply {
							x += 1
							if x > 10 {
								break
							}
							c.WriteMessage(&irc.Message{
								Command: "PRIVMSG",
								Params: []string{
									m.Name,
									strconv.Itoa(x) + ". " + key + " | " + value,
								},
							})
						}
						c.WriteMessage(&irc.Message{
							Command: "PRIVMSG",
							Params: []string{
								m.Name,
								"Search results are capped at 10. For more results please see " + search + url.QueryEscape(strings.Join(searchKey, " ")),
							},
						})
					}
				}
			}
		}),
	}

	client := irc.NewClient(conn, config)
	err = client.Run()
	if err != nil {
		log.Fatalln(err)
	}
}
