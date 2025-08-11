package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

type Broadcaster struct {
	subs map[string]chan<- string
	sink chan string
}
func NewBroadcaster() Broadcaster {
	return Broadcaster{
		subs: make(map[string]chan<- string),
		sink: make(chan string, 100),
	}
}
func (br *Broadcaster) subscribe(remoteAddress string) (rx <-chan string, tx chan<- string) {
	if c, ok := br.subs[remoteAddress]; ok {
		close(c)
	}
	newChan := make(chan string, 10)
	br.subs[remoteAddress] = newChan
	return  newChan, br.sink
}
func (br *Broadcaster) unsubscribe(remoteAddress string)  {
	if c, ok := br.subs[remoteAddress]; ok {
		close(c)
		delete(br.subs, remoteAddress)
	}
}
func (br *Broadcaster) run() {
	for {
		if newMsg, ok := <- br.sink; ok {
			for _, ch := range br.subs {
				ch <- strings.Clone(newMsg)
			}
		} else {
			close(br.sink)
			break;
		}
	}
}

func printUsage() {
	fmt.Println(
		"crosstalk, a basic chat server\n\n",
		"\rUSAGE:\n\r",
		"\tcrosstalk [adress] [port]",
	)
}

func genTimestamp() string {
	now := time.Now();
	hour, min := now.Hour(), now.Minute();
	return fmt.Sprintf("\033[31;1m[%02d:%02d]\033[0m", hour, min);
}

func verifyMessage(msg *string) bool {
	return len(*msg) != 0;
}

func serveConnection(conn net.Conn, broadcaster *Broadcaster){
	defer conn.Close()
	connRW := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	defer conn.Close()
	buf := make([]byte, 1024);
	_, err := connRW.Write([]byte("Username: "));
	if err != nil {
		fmt.Printf("Error while connecting new user: %s", err);
		return;
	}
	connRW.Flush()
	n, err := connRW.Read(buf);
	if err != nil {
		fmt.Printf("Error while trying to accept username: %s", err);
		return;
	}
	username := string(buf[:n]);
	username = strings.TrimSpace(username)

	if len(username) < 5 {
		fmt.Fprintf(connRW, "\033[31;1m~username too short (need at least 5 characters)\033[0m\r\n");
		connRW.Flush()
		return;
	}

	n, err = fmt.Fprintf(connRW, "Welcome %s\r\n", username);
	if err != nil {
		fmt.Printf("Error while welcoming new user: %s", err);
		return;
	}
	r, g, b := rand.Intn(255), rand.Intn(255), rand.Intn(255)
	username = fmt.Sprintf("\033[1;38;2;%03d;%03d;%03dm%s\033[0m", r, g, b, username);
	connRW.Flush()
	connected := true
	newMsgChan, broadcast := broadcaster.subscribe(conn.RemoteAddr().String());
	broadcast <- fmt.Sprintf("%s %s joined in!\r\n", genTimestamp(), username);
	go func() {
		for {
			n, err := connRW.Read(buf);
			if err != nil {
				fmt.Println(err);
				connected = false;
				broadcaster.unsubscribe(conn.RemoteAddr().String())
				return;
			}
			mess := strings.TrimSpace(string(buf[:n]))
			if ok := verifyMessage(&mess); !ok {
				if _, err := connRW.WriteString("\033[31;1m~empty message\033[0m\r\n"); err != nil {
					fmt.Println(err);
					connected = false;
					broadcaster.unsubscribe(conn.RemoteAddr().String())
				}
				connRW.Flush()
			} else {
				broadcast <- fmt.Sprintf("%s %s: %s\033[0m\r\n", genTimestamp(), username, mess);
			}
		}
	}()
	for {
		newMsg, ok := <- newMsgChan
		if ok && connected {
			_, err := connRW.Write([]byte(newMsg))
			if err != nil {
				fmt.Printf("Error while sending broadcast message to user %s: %s\n", username, err);
				return;
			}
			if err := connRW.Flush(); err != nil {
				fmt.Println("Error while flushing");
				return;
			}
		} else if !connected {
			broadcast <- fmt.Sprintf("%s %s disconnected!\r\n", genTimestamp(), username);
			return;
		} else {
			return;
		}
	}
}

func main() {
	args := os.Args;
	if len(args) == 1 {
		printUsage();
		os.Exit(0);
	}
	if len(args) != 3 {
		fmt.Println("Error: too few arguments provided.");
		os.Exit(-1);
	}
	address, port := args[1], args[2];

	sock, err := net.Listen("tcp", net.JoinHostPort(address, port))
	if err != nil {
		fmt.Println(err);
		os.Exit(-1);
	}
	defer sock.Close();
	broadcaster := NewBroadcaster();
	go broadcaster.run();
	go echoMessages(&broadcaster, sock.Addr().String())
	for {
		conn, err := sock.Accept()
		if err != nil {
			fmt.Println(err);
			os.Exit(-1);
		}
		go serveConnection(conn, &broadcaster)
	}
}
func echoMessages(br *Broadcaster, addr string) {
	broadcast, _ := br.subscribe(addr);
	for{
		if newMsg, ok := <- broadcast; ok {
			fmt.Print(newMsg)
		} else {
			return;
		}
	}
}
