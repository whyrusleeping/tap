package main

import (
	"fmt"

	"github.com/veandco/go-sdl2/sdl"
	ttf "github.com/veandco/go-sdl2/sdl_ttf"
)

type Label struct {
	tex *sdl.Texture
	s *sdl.Surface
	text string
	color sdl.Color
	font *ttf.Font
	rend *sdl.Renderer

	loc sdl.Rect
}

func NewLabel(text string, color sdl.Color, font *ttf.Font, loc sdl.Rect, rend *sdl.Renderer) *Label {
	l := new(Label)
	l.color = color
	l.font = font
	l.loc = loc
	l.rend = rend
	l.SetText(text)
	return l
}

func (l *Label) SetText(text string) {
	l.text = text
	if l.s != nil {
		l.s.Free()
		l.s = nil
	}
	if l.tex != nil {
		l.tex.Destroy()
		l.tex = nil
	}
	sur := l.font.RenderText_Solid(text, l.color)
	if sur == nil {
		if len(text) > 0 {
			fmt.Println(ttf.GetError())
		}
		return
	}
	l.s = sur
	l.loc.W, l.loc.H = l.Size()
	l.tex = l.rend.CreateTextureFromSurface(sur)
	if l.tex == nil {
		fmt.Println("Texture was nil")
	}
}

func (l *Label) Draw() int {
	return l.rend.Copy(l.tex, nil, &l.loc)
}

func (l *Label) SetPosition(r sdl.Rect) {
	l.loc = r
}

func (l *Label) HandleEvent(e sdl.Event) bool {
	return false
}

func (l *Label) Size() (int32,int32) {
	return l.s.W, l.s.H
}

