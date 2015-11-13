package main

import (
	"flag"
	"fmt"
	"github.com/centrifugal/centrifuge-go"
	"github.com/centrifugal/centrifugo/libcentrifugo"
	"github.com/centrifugal/centrifugo/libcentrifugo/auth"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var (
	numConnections = flag.Int("c", 1, "Number of connections")
	numChannels    = flag.Int("C", 1, "Number of channels per connection")
	host           = flag.String("h", "localhost:8000", "Centrifugo host:port")
	msgDelay       = flag.Int("d", 500, "Message delay (ms)")
	secret         = flag.String("s", "secret", "Secret key")
	namespace      = flag.String("n", "load", "Secret key")
	wsUrl          = ""
)

type Client struct {
	creds *centrifuge.Credentials
	cent  *centrifuge.Centrifuge
	wg    *sync.WaitGroup
	done  chan bool
	delay time.Duration
	subs  []*centrifuge.Sub
}

func NewClient(num int, wg *sync.WaitGroup, done chan bool) *Client {
	user := strconv.Itoa(num)
	timestamp := centrifuge.Timestamp()
	token := auth.GenerateClientToken(*secret, user, timestamp, "")

	client := &Client{wg: wg, done: done}

	creds := &centrifuge.Credentials{
		User:      user,
		Timestamp: timestamp,
		Info:      "",
		Token:     token,
	}

	events := &centrifuge.EventHandler{
		OnDisconnect: client.DisconnectHandler,
		OnPrivateSub: nil,
		OnRefresh:    nil,
	}

	cent := centrifuge.NewCentrifuge(wsUrl, creds, events, centrifuge.DefaultConfig)

	delay := time.Duration(*msgDelay) * time.Millisecond

	client.creds = creds
	client.cent = cent
	client.delay = delay

	return client
}

func (c *Client) DisconnectHandler(_ *centrifuge.Centrifuge) error {
	select {
	case <-c.done:
		return nil
	default:
		go c.prepare()
	}

	return nil
}

func (c *Client) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.cent.Close()

		c.prepare()
		c.perform()
	}()
}

func (c *Client) prepare() {
	c.connect()
	if c.cent.Connected() {
		c.subscribe()
	}
}

func (c *Client) connect() {
	err := c.cent.Connect()
	if err != nil {
		log.Println(c.creds.User, "Centrifuge connect: ", err)
	}
}

func (c *Client) subscribe() {
	c.subs = []*centrifuge.Sub{}

	events := &centrifuge.SubEventHandler{
		OnMessage: c.MessageHandler,
		OnJoin:    c.JoinHandler,
		OnLeave:   c.LeaveHandler,
	}

	for channel := 1; channel <= *numChannels; channel++ {
		sub, err := c.cent.Subscribe(fmt.Sprintf("%s:user%s-ch%v", *namespace, c.creds.User, channel), events)
		if err != nil {
			log.Println(c.creds.User, "Subscribe: ", err)
		}
		c.subs = append(c.subs, sub)
	}
}

func (c *Client) perform() {
	rand.Seed(time.Now().Unix())

	for {
		select {
		case <-c.done:
			return
		default:
			if c.cent.Connected() {
				err := c.subs[rand.Intn(len(c.subs))].Publish([]byte("{\"message\":\"hello\"}"))
				if err != nil {
					log.Println("Publish: ", err)
				}
			}
			time.Sleep(c.delay)
		}
	}
}

func (c *Client) MessageHandler(sub *centrifuge.Sub, msg libcentrifugo.Message) error {
	return nil
}

func (c *Client) JoinHandler(sub *centrifuge.Sub, msg libcentrifugo.ClientInfo) error {
	return nil
}

func (c *Client) LeaveHandler(sub *centrifuge.Sub, msg libcentrifugo.ClientInfo) error {
	return nil
}

func main() {
	flag.Parse()

	wsUrl = "ws://" + *host + "/connection/websocket"
	wg := &sync.WaitGroup{}
	done := make(chan bool)

	for conn := 1; conn <= *numConnections; conn++ {
		client := NewClient(conn, wg, done)
		client.Start()
		time.Sleep(1 * time.Millisecond)
	}

	s := make(chan os.Signal)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	<-s
	log.Println("Stopping")

	close(done)
	wg.Wait()
}
