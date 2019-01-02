package main

import (
    "fmt"
    "net/http"
    "os"
    "os/exec"

    "github.com/karrick/golf"
)

func Listen(port int, server string, ffmpegPath string) int {
    http.Handle("/", NewStreamHandler(server, ffmpegPath))


    if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
        fmt.Print("Error: Listen: ", err)
        return 1
    }

    return 0
}

func main() {
    ffmpegPath, err := exec.LookPath("ffmpeg")
    if err != nil {
        ffmpegPath = ""
    }

    optHelp := golf.BoolP('h', "help", false, "display this help and exit")
    optFfmpegPath := golf.StringP('f', "ffmpeg", ffmpegPath, "FFMPEG path")
    optServer := golf.StringP('s', "server", "", "Enigma2 streaming server")
    optPort := golf.IntP('p', "port", 1234, "Streaming port")

    golf.Parse()

    if *optHelp || *optServer == "" || *optFfmpegPath == "" {
        golf.Usage()

        os.Exit(0)
    }

    os.Exit(Listen(*optPort, *optServer, *optFfmpegPath))
}
