package jsontypes

type ColorData struct {
    Type string `json:"type"`
    Color string `json:"color"`
}

type ChatData struct {
    Type string `json:"type"`
    Color string `json:"color"`
    Message string `json:"message"`
}

type EventData struct {
    CoordX int `json:"coord_x"`
    CoordY int `json:"coord_y"`
    Direction string `json:"direction"`
}

type StartGame struct {
    Type string `json:"type"`
    Colors []string `json:"colors"`
}

type GameData struct {
    Type string `json:"type"`
    Color string `json:"color"`
    Event EventData `json:"event"`
}
