package main

import "github.com/tron_server/server"


func main() {
    s := server.CreateServer()
    s.Start("8765")
}
