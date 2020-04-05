// Copyright 2015 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build darwin freebsd linux windows
// +build !js
// +build !android
// +build !ios

package glfw

import (
	"sync"
	"unicode"

	"github.com/hajimehoshi/ebiten/internal/driver"
	"github.com/hajimehoshi/ebiten/internal/glfw"
)

type gamePad struct {
	valid         bool
	guid          string
	name          string
	axisNum       int
	axes          [16]float64
	buttonNum     int
	buttonPressed [256]bool
}

type Input struct {
	keyPressed         map[glfw.Key]bool
	mouseButtonPressed map[glfw.MouseButton]bool
	onceCallback       sync.Once
	scrollX            float64
	scrollY            float64
	cursorX            int
	cursorY            int
	gamepads           [16]gamePad
	touches            map[int]pos // This is not updated until GLFW 3.3 is available (#417)
	runeBuffer         []rune
	ui                 *UserInterface
}

type pos struct {
	X int
	Y int
}

func (i *Input) CursorPosition() (x, y int) {
	i.ui.m.RLock()
	cx, cy := i.cursorX, i.cursorY
	i.ui.m.RUnlock()
	return i.ui.adjustPosition(cx, cy)
}

func (i *Input) GamepadIDs() []int {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if len(i.gamepads) == 0 {
		return nil
	}
	r := []int{}
	for id, g := range i.gamepads {
		if g.valid {
			r = append(r, id)
		}
	}
	return r
}

func (i *Input) GamepadSDLID(id int) string {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if len(i.gamepads) <= id {
		return ""
	}
	return i.gamepads[id].guid
}

func (i *Input) GamepadName(id int) string {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if len(i.gamepads) <= id {
		return ""
	}
	return i.gamepads[id].name
}

func (i *Input) GamepadAxisNum(id int) int {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if len(i.gamepads) <= id {
		return 0
	}
	return i.gamepads[id].axisNum
}

func (i *Input) GamepadAxis(id int, axis int) float64 {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if len(i.gamepads) <= id {
		return 0
	}
	return i.gamepads[id].axes[axis]
}

func (i *Input) GamepadButtonNum(id int) int {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if len(i.gamepads) <= id {
		return 0
	}
	return i.gamepads[id].buttonNum
}

func (i *Input) IsGamepadButtonPressed(id int, button driver.GamepadButton) bool {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if len(i.gamepads) <= id {
		return false
	}
	return i.gamepads[id].buttonPressed[button]
}

func (i *Input) TouchIDs() []int {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()

	if len(i.touches) == 0 {
		return nil
	}

	var ids []int
	for id := range i.touches {
		ids = append(ids, id)
	}
	return ids
}

func (i *Input) TouchPosition(id int) (x, y int) {
	i.ui.m.RLock()
	found := false
	var p pos
	for tid, pos := range i.touches {
		if id == tid {
			p = pos
			found = true
			break
		}
	}
	i.ui.m.RUnlock()

	if !found {
		return 0, 0
	}
	return i.ui.adjustPosition(p.X, p.Y)
}

func (i *Input) RuneBuffer() []rune {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	return i.runeBuffer
}

func (i *Input) ResetForFrame() {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	i.runeBuffer = i.runeBuffer[:0]
	i.scrollX, i.scrollY = 0, 0
}

func (i *Input) IsKeyPressed(key driver.Key) bool {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if i.keyPressed == nil {
		i.keyPressed = map[glfw.Key]bool{}
	}
	for gk, k := range glfwKeyCodeToKey {
		if k != key {
			continue
		}
		if i.keyPressed[gk] {
			return true
		}
	}
	return false
}

func (i *Input) IsMouseButtonPressed(button driver.MouseButton) bool {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	if i.mouseButtonPressed == nil {
		i.mouseButtonPressed = map[glfw.MouseButton]bool{}
	}
	for gb, b := range glfwMouseButtonToMouseButton {
		if b != button {
			continue
		}
		if i.mouseButtonPressed[gb] {
			return true
		}
	}
	return false
}

func (i *Input) Wheel() (xoff, yoff float64) {
	i.ui.m.RLock()
	defer i.ui.m.RUnlock()
	return i.scrollX, i.scrollY
}

var glfwMouseButtonToMouseButton = map[glfw.MouseButton]driver.MouseButton{
	glfw.MouseButtonLeft:   driver.MouseButtonLeft,
	glfw.MouseButtonRight:  driver.MouseButtonRight,
	glfw.MouseButtonMiddle: driver.MouseButtonMiddle,
}

func (i *Input) appendRuneBuffer(char rune) {
	if !unicode.IsPrint(char) {
		return
	}
	i.ui.m.Lock()
	i.runeBuffer = append(i.runeBuffer, char)
	i.ui.m.Unlock()
}

func (i *Input) setWheel(xoff, yoff float64) {
	i.ui.m.Lock()
	i.scrollX = xoff
	i.scrollY = yoff
	i.ui.m.Unlock()
}

func (i *Input) update(window *glfw.Window, scale float64) {
	i.ui.m.Lock()
	defer i.ui.m.Unlock()

	i.onceCallback.Do(func() {
		window.SetCharModsCallback(func(w *glfw.Window, char rune, mods glfw.ModifierKey) {
			i.appendRuneBuffer(char)
		})
		window.SetScrollCallback(func(w *glfw.Window, xoff float64, yoff float64) {
			i.setWheel(xoff, yoff)
		})
	})
	if i.keyPressed == nil {
		i.keyPressed = map[glfw.Key]bool{}
	}
	for gk := range glfwKeyCodeToKey {
		i.keyPressed[gk] = window.GetKey(gk) == glfw.Press
	}
	if i.mouseButtonPressed == nil {
		i.mouseButtonPressed = map[glfw.MouseButton]bool{}
	}
	for gb := range glfwMouseButtonToMouseButton {
		i.mouseButtonPressed[gb] = window.GetMouseButton(gb) == glfw.Press
	}
	x, y := window.GetCursorPos()
	i.cursorX = int(x / scale)
	i.cursorY = int(y / scale)
	for id := glfw.Joystick(0); id < glfw.Joystick(len(i.gamepads)); id++ {
		i.gamepads[id].valid = false
		if !glfw.JoystickPresent(id) {
			continue
		}
		i.gamepads[id].valid = true
		i.gamepads[id].guid = glfw.GetJoystickGUID(id)
		i.gamepads[id].name = glfw.GetJoystickName(id)

		axes32 := glfw.GetJoystickAxes(id)
		i.gamepads[id].axisNum = len(axes32)
		for a := 0; a < len(i.gamepads[id].axes); a++ {
			if len(axes32) <= a {
				i.gamepads[id].axes[a] = 0
				continue
			}
			i.gamepads[id].axes[a] = float64(axes32[a])
		}
		buttons := glfw.GetJoystickButtons(id)
		i.gamepads[id].buttonNum = len(buttons)
		for b := 0; b < len(i.gamepads[id].buttonPressed); b++ {
			if len(buttons) <= b {
				i.gamepads[id].buttonPressed[b] = false
				continue
			}
			i.gamepads[id].buttonPressed[b] = glfw.Action(buttons[b]) == glfw.Press
		}
	}
}
