package server

import (
    "testing"
    "os"
    "net"
    "bufio"
    "time"
    "github.com/tron_server/jsontypes"
    "encoding/json"
    "regexp"
    "fmt"
    "strings"
)

const port = "8765"

func assertEqual(t *testing.T, a interface{}, b interface{}, message string) {
    if a == b {
	return
    }
    if len(message) == 0 {
	message = fmt.Sprintf("%v != %v", a, b)
    }
    t.Fatal(message)
}

// e.g #FF1276
func assertColorFormat(t *testing.T, color string) {
    var validColor = regexp.MustCompile(`^#([a-f]|[A-F]|[0-9]){6}$`)
    if !validColor.MatchString(color) {
	t.Fatal("Invalid color format")
    }
}


func TestMain(m *testing.M) {
    s := Create()
    go func() {
	os.Exit(m.Run())
    }()
    // server should shut down if client's disconnected successfully. No need to
    // shut down server manually.
    s.Start(port)
}

func sendMessage(t *testing.T, c net.Conn, message string) {
    c.Write([]byte(message + "\n"))
}

func assertReceive(t *testing.T, c net.Conn, message string) {
    resp, err := bufio.NewReader(c).ReadString('\n')
    if err != nil {
	t.Error("Cannot read message")
    }
    assertEqual(t, strings.TrimSpace(string(resp)), message, "")
}

func assertStartGameReceived(t *testing.T, c net.Conn, colors []string) {
    data, err := bufio.NewReader(c).ReadString('\n')
    if err != nil {
	t.Error("Reading from server failed.")
    }
    startData := &jsontypes.StartGame{}
    if err := json.Unmarshal([]byte(data), startData); err != nil {
	t.Error("Malformed color message.")
    }
    assertEqual(t, startData.Type, "start_game", "")

    colorsOk := make([]bool, len(colors))
    for i := range startData.Colors {
	for j:= range colorsOk {
	    if startData.Colors[i] == colors[j] {
		colorsOk[j] = true
	    }
	}
    }
    for i:= range colorsOk {
	if !colorsOk[i] {
	    t.Errorf("Color of player %d cannot be found in list", i)
	}
    }
}

func TestServerTwoPlayers(t *testing.T) {
    conn1, err := net.Dial("tcp", ":" + port)
    t.Logf("Player 1: connect..")
    if err != nil {
	t.Error("connection failed.")
    }
    defer conn1.Close()
    conn1.SetReadDeadline(time.Now().Add(5 * time.Second))
    t.Logf("Player 1: receiving color message..")
    data, err := bufio.NewReader(conn1).ReadString('\n')
    if err != nil {
	t.Error("Reading from server failed.")
    }

    t.Logf("Player 1: message received: '%s'", data)
    t.Logf("Player 1: unpacking color message..")
    conn1.SetReadDeadline(time.Now().Add(5 * time.Second))
    jsonData := &jsontypes.ColorData{}
    if err := json.Unmarshal([]byte(data), jsonData); err != nil {
	t.Error("Malformed color message.")
    }
    color1 := jsonData.Color
    assertEqual(t, jsonData.Type, "connect", "Malformed message type")
    t.Logf("Player 1: color: %s", color1)
    assertColorFormat(t, color1)

    t.Logf("Player 2: connect..")
    conn2, err := net.Dial("tcp", ":" + port)
    if err != nil {
	t.Error("connection failed.")
    }
    conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
    defer conn2.Close()

    // read color for player 2
    data, err = bufio.NewReader(conn2).ReadString('\n')
    if err != nil {
	t.Error("Player 2: Reading from server failed.")
    }
    t.Logf("Player 2: Message received: '%s'", data)

    // Ignore player connected message
    t.Logf("Player 1: Ignore connection received message")
    bufio.NewReader(conn1).ReadString('\n')

    t.Logf("Player 2: Unpacking color message..")
    conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
    jsonData = &jsontypes.ColorData{}
    if err := json.Unmarshal([]byte(data), jsonData); err != nil {
	t.Error("Player 2: Malformed color message.")
    }
    color2 := jsonData.Color
    t.Logf("Player 2: Color: '%s'", color2)
    assertEqual(t, jsonData.Type, "connect", "Malformed message type")
    assertColorFormat(t, color2)

    t.Logf("Player 1: Send chat message to Player 2..")
    message := fmt.Sprintf(`{"type": "chat", "color" : "%s", "message": "hello player 2"}`,
	jsonData.Color)
    sendMessage(t, conn1, message)
    assertReceive(t, conn2, message)

    t.Logf("Player 1: Send ready")
    sendMessage(t, conn1, `{"type":"ready"}`)

    t.Logf("Player 2: Send ready")
    sendMessage(t, conn2, `{"type":"ready"}`)

    colors := []string{color1, color2}
    t.Logf("Player 1: Receive start game..")
    assertStartGameReceived(t, conn1, colors)
    assertStartGameReceived(t, conn2, colors)

    // TODO "press enter" to initiate game start
    // TODO assert ticking
    // TODO assert direction change
}

