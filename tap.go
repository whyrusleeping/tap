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
	"sort"
)

type ProgList []Prog
func (p ProgList) Swap(i, j int) {
	p[i],p[j] = p[j],p[i]
}

func (p ProgList) Less(i,j int) bool {
	return p[i].Name < p[j].Name
}

func (p ProgList) Len() int {
	return len(p)
}

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
	programs ProgList
}

func NewTap() *Tap {
	t := new(Tap)
	t.Incoming = make(chan *Message)
	return t
}

//Find most likely program given input string
//TODO: use a fuzzy search
func (t *Tap) FindLikely(s string) string {
	i := len(t.programs) / 2
	beg := 0
	end := len(t.programs)
	for {
		p := t.programs[i]
		if len(s) <= len(p.Name) {
			if p.Name[:len(s)] == s {
				//Check for better fit.
				for j := i; j > 0; j-- {
					if t.programs[j].Name == s {
						return t.programs[j].Name
					}
					if p.Name[:len(s)] != s {
						break
					}
				}
				return p.Name
			}
		}
		if s < p.Name {
			end = i
			i = (i + beg) / 2
		} else {
			beg = i
			i = (i + end) / 2
		}
	}
	return ""
}

//Start listener for daemon messages
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


func (t *Tap) SendMessage(mes string) {
	t.Incoming <- &Message{mes}
}

//Find all programs in the users path
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
	sort.Sort(t.programs)
}

//Execute a program
func (t *Tap) Exec(e string) {
	full,err := exec.LookPath(e)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(full)
	cmd := exec.Command(full)
	cmd.Run()
}

//Create and run the graphical interface
func (t *Tap) StartInterface() {
	//Lock thread so SDL doesnt crash
	runtime.LockOSThread()

	//Initialize libraries
	sdl.Init(sdl.INIT_EVERYTHING)
	ttf.Init()
	defer sdl.Quit()
	defer ttf.Quit()

	//Create window
	win,rend := sdl.CreateWindowAndRenderer(300, 60, sdl.WINDOW_BORDERLESS | sdl.WINDOW_OPENGL)
	win.SetTitle("Tap")

	//Load font from GOPATH
	gopath := os.Getenv("GOPATH")
	f,err := ttf.OpenFont(gopath + "/src/github.com/whyrusleeping/tap/audiowide.ttf",22)
	if err != nil {
		fmt.Println(err)
	}

	//Create label objects
	typed := NewLabel("", sdl.Color{255,255,255,255}, f, sdl.Rect{0,0,100,30}, rend)
	ghost := NewLabel("", sdl.Color{128,128,128,255}, f, sdl.Rect{0,0,100,30}, rend)

	//update display 20 times per second
	tick := time.NewTicker(time.Millisecond * 50)

	t.active = true
	txt := ""
	sel := ""
	for {
		select {
		case m := <-t.Incoming:
			switch m.Command {
			case "hide":
				tick.Stop()
				t.active = false
				win.Hide()
			case "kill":
				fmt.Println("Received kill signal.")
				return //TODO: any cleanup?
			case "show":
				tick = time.NewTicker(time.Millisecond * 50)
				t.active = true
				win.Show()

				txt = ""
				typed.SetText(txt)
				rend.Clear()
				typed.Draw()
				rend.Present()
			}
			typed.SetText(m.Command)
		case <-tick.C:
			for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
				switch ev := ev.(type) {
				case *sdl.QuitEvent:
					return
				case *sdl.KeyDownEvent:
					if ev.Keysym.Sym == 13 {
						//On enter key, execute
						go t.SendMessage("hide")
						t.Exec(sel)
						sel = ""
						txt = ""
					} else if ev.Keysym.Sym == 27 {
						//escape key, just hide
						go t.SendMessage("hide")
						sel = ""
						txt = ""
					} else if ev.Keysym.Sym <= 'z' && ev.Keysym.Sym >= 'a' {
						//Letters...
						txt += string(ev.Keysym.Sym)
						sel = t.FindLikely(txt)
						ghost.SetText(sel)
					} else if ev.Keysym.Sym == 8 {
						//Backspace
						if len(txt) > 0 {
							txt = txt[:len(txt)-1]
						}

						if len(txt) > 0 {
							sel = t.FindLikely(txt)
							ghost.SetText(sel)
						} else {
							sel = t.FindLikely(txt)
							ghost.SetText(sel)
						}
					}
					typed.SetText(txt)
				default:
					//fmt.Println(reflect.TypeOf(ev))
				}
			}

			//Draw everything
			rend.Clear()
			ghost.Draw()
			typed.Draw()
			rend.Present()
		}
	}
}

func main() {
	runtime.GOMAXPROCS(2)

	//Check if already running
	con,err := net.Dial("tcp", ":18838")
	if err == nil {
		mesval := "show"
		if len(os.Args) > 1 {
			mesval = os.Args[1]
		}
		m := Message{mesval}
		enc := json.NewEncoder(con)
		err = enc.Encode(&m)
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	t := NewTap()
	t.BuildProgramCache()
	go t.StartSocket()
	t.StartInterface()
}
