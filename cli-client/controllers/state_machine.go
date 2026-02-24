package controllers

import "cli-client/models"

type StateMachine struct {
	current models.Screen
	onEnter map[models.Screen]func()
	onExit  map[models.Screen]func()
}

func NewStateMachine(initial models.Screen) *StateMachine {
	return &StateMachine{
		current: initial,
		onEnter: make(map[models.Screen]func()),
		onExit:  make(map[models.Screen]func()),
	}
}

func (sm *StateMachine) OnEnter(screen models.Screen, fn func()) {
	sm.onEnter[screen] = fn
}

func (sm *StateMachine) OnExit(screen models.Screen, fn func()) {
	sm.onExit[screen] = fn
}

func (sm *StateMachine) Transition(to models.Screen) {
	if sm.current == to {
		return
	}
	// Call OnExit for the current screen if registered
	if fn, ok := sm.onExit[sm.current]; ok {
		fn()
	}
	sm.current = to
	if fn, ok := sm.onEnter[to]; ok {
		fn()
	}
}

func (sm *StateMachine) Current() models.Screen {
	return sm.current
}
