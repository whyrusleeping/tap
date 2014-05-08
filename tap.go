package main

import (
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/sdl_ttf"
	"net"
	"strings"
	"os"
	"os/exec"
	"path/filepath"
	//"reflect"
	"encoding/json"
	"io"
	"fmt"
	"time"
	"runtime"
)


type Message struct {
	Command string
}

type Prog struct {
	Name string
	Fullpath string
}

type Tap struct {
	Incoming chan *Message
	active bool
	programs []Prog
}

func NewTap() *Tap {
	t := new(Tap)
	t.Incoming = make(chan *Message)
	return t
}

func (t *Tap) handleConnection(r io.ReadCloser) {
	dec := json.NewDecoder(r)
	m := new(Message)

	err := dec.Decode(m)
	if err != nil {
		fmt.Println(err)
	}

	t.Incoming <- m
	r.Close()
}

func (t *Tap) StartSocket() {
	list,err := net.Listen("tcp", ":18838")
	if err != nil {
		panic(err)
	}

	for {
		con,err := list.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go t.handleConnection(con)
	}
}

func (t *Tap) SendMessage(mes string) {
	t.Incoming <- &Message{mes}
}

func (t *Tap) BuildProgramCache() {
	path := os.Getenv("PATH")
	dirs := strings.Split(path,":")
	t.programs = nil
	for _,p := range dirs {
		filepath.Walk(p, func(ppath string, info os.FileInfo, err error) error {
			prog := Prog{}
			prog.Name = filepath.Base(ppath)
			prog.Fullpath = ppath
			t.programs = append(t.programs, prog)
			return nil
		})
	}
}

func (t *Tap) Exec(e string) {
	full,err := exec.LookPath(e)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(full)
	cmd := exec.Command(full)
	cmd.Run()
}

func (t *Tap) StartInterface() {
	sdl.Init(sdl.INIT_EVERYTHING)
	ttf.Init()
	runtime.LockOSThread()
	win,rend := sdl.CreateWindowAndRenderer(300, 60, sdl.WINDOW_BORDERLESS | sdl.WINDOW_OPENGL)
	win.SetTitle("Tap")
	f,err := ttf.OpenFont("audiowide.ttf",22)
	if err != nil {
		fmt.Println(err)
	}
	l := NewLabel("Hello", sdl.Color{255,255,255,255}, f, sdl.Rect{0,0,100,30}, rend)
	tick := time.NewTicker(time.Millisecond * 50)
	t.active = true
	txt := ""
	for {
		select {
		case m := <-t.Incoming:
			switch m.Command {
			case "hide":
				tick.Stop()
				t.active = false
				win.Hide()
			case "show":
				tick = time.NewTicker(time.Millisecond * 50)
				t.active = true
				win.Show()

				txt = ""
				l.SetText(txt)
				rend.Clear()
				l.Draw()
				rend.Present()
			}
			l.SetText(m.Command)
		case <-tick.C:
			for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
				switch ev := ev.(type) {
				case *sdl.QuitEvent:
					return
				case *sdl.KeyDownEvent:
					//fmt.Println("key event.")
					fmt.Printf("[%[1]d]\n", ev.Keysym.Sym)
					if ev.Keysym.Sym == 13 || ev.Keysym.Sym == 27 {
						go t.SendMessage("hide")
						t.Exec(txt)
						txt = ""
					} else if ev.Keysym.Sym <= 'z' && ev.Keysym.Sym >= 'a' {
						txt += string(ev.Keysym.Sym)
					}
				default:
					//fmt.Println(reflect.TypeOf(ev))
				}
			}

			l.SetText(txt)
			rend.Clear()
			l.Draw()
			rend.Present()
		}
	}
}

func main() {
	runtime.GOMAXPROCS(2)
	//Check if already running
	con,err := net.Dial("tcp", ":18838")
	if err == nil {
		m := Message{"show"}
		enc := json.NewEncoder(con)
		err = enc.Encode(&m)
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	t := NewTap()
	t.BuildProgramCache()
	fmt.Println(t.programs)
	go t.StartSocket()
	t.StartInterface()
}
