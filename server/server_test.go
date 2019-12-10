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

func TestServerTwoPlayers(t *testing.T) {
    conn1, err := net.Dial("tcp", ":" + port)
    t.Logf("Trying to conn1ect to server..")
    if err != nil {
	t.Error("connection failed.")
    }
    defer conn1.Close()
    conn1.SetWriteDeadline(time.Now().Add(5 * time.Second))
    t.Logf("Receiving color message..")
    data, err := bufio.NewReader(conn1).ReadString('\n')
    if err != nil {
	t.Error("Reading from server failed.")
    }

    t.Logf("Message received: '%s'", data)
    t.Logf("Unpacking color message..")
    conn1.SetWriteDeadline(time.Now().Add(5 * time.Second))
    jsonData := &jsontypes.ColorData{}
    
    if err := json.Unmarshal([]byte(data), jsonData); err != nil {
	t.Error("Malformed color message.")
    }
    assertEqual(t, jsonData.Type, "connect", "Malformed message type")
    assertColorFormat(t, jsonData.Color)

    conn2, err := net.Dial("tcp", ":" + port)
    if err != nil {
	t.Error("connection failed.")
    }
    conn2.SetWriteDeadline(time.Now().Add(5 * time.Second))
    defer conn2.Close()
    // read color but ignore it for now
    bufio.NewReader(conn2).ReadString('\n')
    message := fmt.Sprintf(`{"type": "chat", "color" : "%s", "message": "hello player 2"}`,
	jsonData.Color)
    sendMessage(t, conn1, message)
    resp, err := bufio.NewReader(conn2).ReadString('\n')
    if err != nil {
	t.Error("Cannot read message")
    }
    assertEqual(t, strings.TrimSpace(string(resp)), message, "")
}

