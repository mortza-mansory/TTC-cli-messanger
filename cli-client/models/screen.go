package models

type Screen int

const (
	ScreenNone    Screen = -1 // sentinel: no active screen yet
	ScreenLoading Screen = iota
	ScreenLogin
	ScreenChat
)
