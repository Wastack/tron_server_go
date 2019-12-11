// Package server implements a TCP server for playing Tron.
//
// The communication protocol is the following:
//
// Right after a successful connection to the server, the following message is
// triggered:
//	{ "type" : "connect", "color" : "#435654" }
// Color is the color of the car given to the player, and type helps the client
// interpret the message.
//
// After that, the server might be given a chat or a ready message:
//	{ "type" : "chat", "color" : "#453565", "message" : "my example message" }
//	{ "type" : "ready" }
// Ready indicates that the player is ready to move to the game phase.
// Chat messages are broadcasted to all players except the sender.
//
// If all the connections sent a ready message, the server notifies the clients:
//	{ "type" : "start_game", "colors" : ["#123456", "#325465"] }
// Colors contain the color of players in game. The clients should render the
// map, but the actual game should not start yet.
//
// One of the players should start the game with the message:
//	{"type" : "start"}
//
// Server starts ticking as a response. The message:
//	{"type" : "tick"}
// is periodically sent to every client. Ticking indicates the elapse of time
// and also keep the clients synchronized.
//
// End of game is not yet implemented. Clients handle all the game logic now.
package server

import (
	"bufio"
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tevino/abool"
	"github.com/tron_server/jsontypes"
	"net"
	"strings"
	"time"
)

type msgFormat struct {
	senderId int
	msg      string
}

type Server struct {
	players []*client
	conns   chan net.Conn
	msgs    chan msgFormat
	dconns  chan int // id

	free_colors    *list.List
	ids            int
	phase          int
	ticking        *abool.AtomicBool
	stopTick       chan bool
	stopListen     chan bool
	stopServer     chan bool
	serverListener net.Listener
}

type client struct {
	id    int
	conn  net.Conn
	color string
	ready bool
}

// Create initializes the server.
func Create() *Server {
	s := Server{
		players:     make([]*client, 0, 5),
		conns:       make(chan net.Conn),
		dconns:      make(chan int),
		msgs:        make(chan msgFormat),
		free_colors: list.New(),
		stopTick:    make(chan bool, 1),
		stopListen:  make(chan bool, 1),
		stopServer:  make(chan bool, 1),
		ticking:     abool.New(),
	}
	// TODO support more player
	colors := []string{"#ff0000", "#00ff00", "#0000ff"}
	for i := range colors {
		s.free_colors.PushBack(colors[i])
	}
	return &s
}

func (s *Server) subscribe(p *client) {
	s.players = append(s.players, p)
	p.id = s.ids
	s.ids++
	e := s.free_colors.Front()
	p.color = e.Value.(string)
	fmt.Printf("Client subscribed. Color: %s\n", p.color)
	s.free_colors.Remove(e)
}

func (s *Server) unsubscribe(p *client) {
	for i, player := range s.players {
		if p == player {
			fmt.Printf("Client with color: %s unsubscribed\n", p.color)
			// put back color
			s.free_colors.PushBack(p.color)
			// remove player
			s.players = append(s.players[:i], s.players[i+1:]...)
		}
	}
}

// Start starts the server. The server will listen on the port passed as an
// argument.
func (s *Server) Start(port string) {
	// start accepting connections. Connection objects will be pushed to
	// a channel.
	go s.hostServer(port)

	// All events are handled here in a centralized
	// "Broker" loop.
	for stop := false; !stop; {
		select {
		case conn := <-s.conns:
			go s.handleConnect(conn)
		case msg := <-s.msgs:
			s.handleMessage(msg)
		case dconn := <-s.dconns:
			s.handleDisconnect(dconn)
		case <-s.stopServer:
			stop = true
		}
	}
	fmt.Printf("Server shutdown\n")
}

func (s *Server) findById(id int) (*client, error) {
	for _, i := range s.players {
		if i.id == id {
			return i, nil
		}
	}
	return nil, errors.New("No player with id")
}

func (s *Server) ticker() {
	fmt.Printf("Ticker started with %d players\n", len(s.players))
	defer func() {
		s.ticking.UnSet()
	}()
	for done := false; !done; {
		s.sendAllClients(`{"type" : "tick"}`, -1)
		select {
		case <-s.stopTick:
			fmt.Println("Ticking stopping")
			done = true
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	fmt.Println("Ticking stopped")
}

func (s *Server) handleMessage(mf msgFormat) {
	m := strings.TrimSpace(mf.msg)
	p, err := s.findById(mf.senderId)
	if err != nil {
		fmt.Println("Error: Player not found in list.")
	}

	switch s.phase {
	case 0: // lobby
		data := &jsontypes.ChatData{}

		if err := json.Unmarshal([]byte(m), data); err != nil {
			fmt.Printf("Error processing chat message: '%s': %s\n", m, err.Error())
		}
		switch data.Type {
		case "chat":
			s.sendAllClients(m, p.id) // broadcast chat message
		case "ready":
			p.ready = true
			// check on all ready
			if s.isAllReady() {
				sg := jsontypes.StartGame{Type: "start_game", Colors: make([]string, 0, 5)}
				for _, p := range s.players {
					sg.Colors = append(sg.Colors, p.color)
				}
				jsonByte, err := json.Marshal(sg)
				if err != nil {
					fmt.Printf("Fatal: could not produce start game json: %s\n", err.Error())
					return
				}
				s.sendAllClients(string(jsonByte), -1)
				s.phase = 1
			}
		default:
			fmt.Printf("Error: unknown message type in lobby phase\n")
		}
	case 1: // game
		// accept no more connections
		s.stopListen <- true
		s.serverListener.Close()

		data := &jsontypes.GameData{}

		if err := json.Unmarshal([]byte(m), data); err != nil {
			fmt.Printf("Error processing chat message: '%s'", m)
		}
		switch data.Type {
		case "start":
			// Start ticking
			if s.ticking.SetToIf(false, true) {
				go s.ticker()
			}
		case "player_event":
			// Player changing direction
			s.sendAllClients(m, p.id) // broadcast
		default:
			fmt.Printf("Error: unknown message type in game phase")
		}
	}

}

func (s *Server) handleDisconnect(id int) {
	p, err := s.findById(id)
	if err != nil {
		fmt.Printf("Error during disconnect")
	}
	s.unsubscribe(p)
	fmt.Printf("Client with id: %d disconnected\n", id)

	// shutdown server if no more player
	if len(s.players) < 1 {
		if s.ticking.IsSet() {
			s.stopTick <- true
		}
		s.shutdown()
	}
}

func (s *Server) shutdown() {
	fmt.Printf("Initiating shutdown\n")
	s.stopListen <- true // indicates normal close
	s.serverListener.Close()
	s.stopServer <- true
}

func (s *Server) sendAllClients(message string, except_id int) {
	message += "\n"
	for i := range s.players {
		if s.players[i].id == except_id {
			continue
		}
		s.players[i].conn.Write([]byte(message))
	}
}

func send(c net.Conn, msg string) {
	msg += "\n"
	c.Write([]byte(msg))
}

func (s *Server) isAllReady() bool {
	if len(s.players) < 2 {
		return false
	}
	for i := range s.players {
		if !s.players[i].ready {
			return false
		}
	}
	return true
}

func (s *Server) handleConnect(c net.Conn) {
	fmt.Printf("Serving %s\n", c.RemoteAddr().String())

	// subscribe new player
	p := client{conn: c}
	s.subscribe(&p)

	// send color to new connection
	m := fmt.Sprintf(`{ "type" : "connect", "color" : "%s" }`, p.color)
	send(c, m)

	m = fmt.Sprintf(`{ "type" : "chat", "color" : "%s", "message" : "%s has connected" }`, p.color, p.color)
	s.sendAllClients(m, p.id)

	// read for messages
	for {
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Printf("Error while reading from player: %s with Id %d: %s\n",
				p.color, p.id, err.Error())
			break
		}
		s.msgs <- msgFormat{p.id, netData}
	}
	s.dconns <- p.id
	fmt.Printf("Serving client with color: %s stopped\n", p.color)
}

// hostServer accepts connections on "port" and push connection objects into a channel.
func (s *Server) hostServer(port string) {
	fmt.Println("Start hosting server")
	PORT := ":" + port
	l, err := net.Listen("tcp4", PORT)
	s.serverListener = l
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer l.Close()

	for stop := false; !stop; {
		c, err := l.Accept()

		if err != nil {
			select {
			case <-s.stopListen:
				fmt.Println("Stop listening")
				stop = true
			default:
				fmt.Printf("Error while listening: %s\n", err.Error())
			}
		}
		if !stop {
			s.conns <- c
		}
	}
}
