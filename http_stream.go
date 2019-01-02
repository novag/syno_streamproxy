package main

import (
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "os/exec"
    "strings"
    "sync"
)

type StreamHandler struct {
    server      string
    ffmpegPath  string
}

func init() {
    log.SetFlags(log.Lshortfile)
}

func NewStreamHandler(server string, ffmpegPath string) *StreamHandler {
    return &StreamHandler{server, ffmpegPath}
}

func (h *StreamHandler) Transcode(w http.ResponseWriter, sref string, port int, wg *sync.WaitGroup) {
    args := []string{
        h.ffmpegPath,
        // 1s timeout
        "-timeout", "3000000",
        "-i", fmt.Sprintf("http://%s:8001%s", h.server, sref),

        // Intel Evansport hardware decoder/encoder
        "-prefer_smd",
        "-vcodec", "h264_smd",
        "-crf", "16",
        "-movflags", "+faststart",

        "-acodec", "copy",

        "-f", "mpegts",
        fmt.Sprintf("tcp://127.0.0.1:%d", port),
    }

    cmd := exec.Command("/bin/sudo", args...)

    log.Printf("%s %v", h.ffmpegPath, args)

    if err := cmd.Start(); err != nil {
        log.Print("Error: Transcode: ", err)
        http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
        wg.Done()
        return
    }

    exited := make(chan error)
    go func() { exited <- cmd.Wait() }()
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case err := <-exited:
        log.Print("Error: Transcode: ", err)
        http.Error(w, http.StatusText(http.StatusGatewayTimeout), http.StatusGatewayTimeout)
        wg.Done()
        return
    case <-done:
    }

    if err := cmd.Process.Kill(); err != nil {
        log.Print("Error: Transcode: ", err)
    }

    cmd.Wait()

    log.Print("Transcode finished.")
}

func (h *StreamHandler) ProxyTCP(w http.ResponseWriter, port chan int, wg *sync.WaitGroup) {
    listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:0"))
    if err != nil {
        log.Print("Error: ProxyTCP: ", err)
        http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
        wg.Done()
        return
    }

    port <- listener.Addr().(*net.TCPAddr).Port

    go func() {
        wg.Wait()
        listener.Close()
    }()

    conn, err := listener.Accept()
    if err != nil {
        log.Print("Error: ProxyTCP: ", err)
        return
    }

    go func() {
        wg.Wait()
        conn.Close()
    }()
    defer conn.Close()

    w.Header().Set("Connection", "Keep-Alive")
    w.Header().Set("Transfer-Encoding", "chunked")
    w.Header().Add("Content-Type", "video/mpeg")

    io.Copy(w, conn)
}

func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    log.Printf("Stream request: %v", r.URL.Path)
    sref := strings.Split(r.URL.Path, ".")[0]

    if sref == "" {
        http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
        return
    }

    w.Header().Add("Server", "syno_transcoder")

    var wg sync.WaitGroup
    wg.Add(1)

    notify := w.(http.CloseNotifier).CloseNotify()
    go func() {
        <-notify
        wg.Done()
        log.Print("Connection closed.")
    }()

    port := make(chan int)
    go h.ProxyTCP(w, port, &wg)
    go h.Transcode(w, sref, <-port, &wg)

    wg.Wait()
}