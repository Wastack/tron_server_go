package main

import "github.com/tron_server/server"


func main() {
    s := server.Create()
    s.Start("8765")
}
