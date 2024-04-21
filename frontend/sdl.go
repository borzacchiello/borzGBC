package frontend

import (
	"borzGBC/gbc"
	"fmt"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

var (
	SDL_WHITE = sdl.Color{R: 255, G: 255, B: 255, A: 0}
	SDL_BLACK = sdl.Color{R: 0, G: 0, B: 0, A: 0}
)

type SDLPlugin struct {
	window        *sdl.Window
	renderer      *sdl.Renderer
	surface       *sdl.Surface
	width, height int
	scale         int

	pushNotificationCounter int
	pushNotificationText    string
	font                    *ttf.Font
	charWidth, charHeight   int

	audioSpec      sdl.AudioSpec
	audioDevice    sdl.AudioDeviceID
	soundBuffer    []byte
	soundBufferIdx int

	fastMode int
	slowMode bool
}

func MakeSDLPlugin(scale int) (*SDLPlugin, error) {
	var err error

	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return nil, err
	}
	pl := &SDLPlugin{
		width:    160,
		height:   144,
		scale:    scale,
		fastMode: 0,
		slowMode: false,
	}

	pl.window, pl.renderer, err = sdl.CreateWindowAndRenderer(
		int32(pl.width*pl.scale), int32(pl.height*pl.scale), 0)
	if err != nil {
		return nil, err
	}
	pl.setTitle()

	pl.surface, err = sdl.CreateRGBSurface(
		0, int32(pl.width), int32(pl.height), 32, 0xFF000000, 0x00FF0000, 0x0000FF00, 0x000000FF)
	if err != nil {
		return nil, err
	}

	pl.renderer.SetDrawColor(0, 0, 0, 255)
	pl.renderer.Clear()

	// Audio
	want := sdl.AudioSpec{}
	want.Freq = 44100
	want.Format = sdl.AUDIO_S8
	want.Channels = 2
	want.Samples = 2048
	pl.audioDevice, err = sdl.OpenAudioDevice("", false, &want, &pl.audioSpec, 0)
	if err != nil {
		return nil, err
	}
	pl.soundBuffer = make([]byte, pl.audioSpec.Samples*2)
	sdl.PauseAudioDevice(pl.audioDevice, false)

	// Fonts
	err = ttf.Init()
	if err != nil {
		return nil, err
	}
	pl.font, err = ttf.OpenFont("resources/courier.ttf", 20)
	if err != nil {
		return nil, err
	}
	// deduce font size
	s, err := pl.font.RenderUTF8Solid("A", SDL_WHITE)
	if err != nil {
		return nil, err
	}
	pl.charHeight = int(s.H)
	pl.charWidth = int(s.W)
	s.Free()

	return pl, nil
}

func (pl *SDLPlugin) NotifyAudioSample(l, r int8) {
	// fmt.Printf("adding sample: %d, %d\n", l, r)
	if pl.soundBufferIdx >= len(pl.soundBuffer) {
		pl.soundBufferIdx = 0
		sdl.QueueAudio(pl.audioDevice, pl.soundBuffer[:])
	}
	pl.soundBuffer[pl.soundBufferIdx] = byte(l)
	pl.soundBuffer[pl.soundBufferIdx+1] = byte(r)
	pl.soundBufferIdx += 2
}

func (pl *SDLPlugin) DisplayNotification(text string) {
	pl.pushNotificationCounter = 120
	pl.pushNotificationText = text
	if len(pl.pushNotificationText)*pl.charWidth-20 > pl.width*pl.scale {
		// Trim the text to fit the screen
		newLen := (pl.width*pl.scale+20)/pl.charWidth - 1
		pl.pushNotificationText = pl.pushNotificationText[:newLen]
	}
}

func (pl *SDLPlugin) Destroy() {
	pl.renderer.Destroy()
	pl.window.Destroy()
	pl.surface.Free()
	pl.font.Close()
	sdl.CloseAudioDevice(pl.audioDevice)
	sdl.Quit()
}

func (pl *SDLPlugin) SetPixel(x, y int, c uint32) {
	var r, g, b, a uint8
	r = uint8((c >> 24) & 0xFF)
	g = uint8((c >> 16) & 0xFF)
	b = uint8((c >> 8) & 0xFF)
	a = uint8(c & 0xFF)

	pixels := pl.surface.Pixels()
	pixels[y*int(pl.surface.Pitch)+x*int(pl.surface.BytesPerPixel())+0] = a
	pixels[y*int(pl.surface.Pitch)+x*int(pl.surface.BytesPerPixel())+1] = b
	pixels[y*int(pl.surface.Pitch)+x*int(pl.surface.BytesPerPixel())+2] = g
	pixels[y*int(pl.surface.Pitch)+x*int(pl.surface.BytesPerPixel())+3] = r
}

func (pl *SDLPlugin) CommitScreen() {
	texture, err := pl.renderer.CreateTextureFromSurface(pl.surface)
	if err != nil {
		fmt.Println("Unable to create texture while rendering")
		return
	}
	defer texture.Destroy()

	rect := sdl.Rect{
		X: 0,
		Y: 0,
		W: int32(pl.width * pl.scale),
		H: int32(pl.height * pl.scale)}
	pl.renderer.Copy(texture, nil, &rect)

	if pl.pushNotificationCounter > 0 {
		pl.pushNotificationCounter -= 1

		pushSurface, err := pl.font.RenderUTF8Shaded(
			pl.pushNotificationText, SDL_WHITE, SDL_BLACK)
		if err != nil {
			fmt.Println("Unable to render push notification")
			return
		}
		defer pushSurface.Free()
		pushTexture, err := pl.renderer.CreateTextureFromSurface(pushSurface)
		if err != nil {
			fmt.Println("Unable to create texture while rendering (push notification)")
			return
		}
		defer texture.Destroy()
		pushRect := sdl.Rect{
			X: 10,
			Y: 10,
			W: int32(pl.charWidth * len(pl.pushNotificationText)),
			H: int32(pl.charHeight)}
		pl.renderer.Copy(pushTexture, nil, &pushRect)
	}

	pl.renderer.Present()

	pl.renderer.SetDrawColor(0xff, 0xff, 0xff, 0xff)
	pl.renderer.Clear()
}

func (pl *SDLPlugin) setTitle() {
	title := "BorzGBC"
	if pl.fastMode > 0 {
		title += fmt.Sprintf(" - fast x%d", pow2(pl.fastMode))
	} else if pl.slowMode {
		title += " - SLOW"
	}
	pl.window.SetTitle(title)
}

func pow2(n int) int {
	if n == 0 {
		return 1
	}
	result := 2
	for i := 2; i <= n; i++ {
		result *= 2
	}
	return result
}

func (pl *SDLPlugin) Run(console *gbc.Console) error {
	running := true
	for running {
		start := time.Now()

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				running = false
			case *sdl.KeyboardEvent:
				if t.Repeat != 0 {
					break
				}
				keyCode := t.Keysym.Sym
				switch keyCode {
				case sdl.K_q:
					running = false
				case sdl.K_F1, sdl.K_F2, sdl.K_F3, sdl.K_F4:
					if t.State == sdl.PRESSED {
						n := int(keyCode - sdl.K_F1 + 1)
						err := console.SaveState(n)
						if err != nil {
							pl.DisplayNotification("error while saving state")
						} else {
							pl.DisplayNotification("state saved")
						}
					}
				case sdl.K_F5, sdl.K_F6, sdl.K_F7, sdl.K_F8:
					n := int(keyCode - sdl.K_F5 + 1)
					if t.State == sdl.PRESSED {
						err := console.LoadState(n)
						if err != nil {
							pl.DisplayNotification("error while loading state")
						} else {
							pl.DisplayNotification("state loaded")
						}
					}
				case sdl.K_f:
					if t.State == sdl.PRESSED {
						console.CPUFreq = gbc.GBCPU_FREQ
						pl.fastMode = (pl.fastMode + 1) % 4
						console.CPUFreq = gbc.GBCPU_FREQ * pow2(pl.fastMode)
						pl.slowMode = false
						if pl.fastMode > 0 {
							pl.DisplayNotification(fmt.Sprintf("fast mode x%d", pow2(pl.fastMode)))
						} else {
							pl.DisplayNotification("normal mode")
						}
						pl.setTitle()
					}
				case sdl.K_g:
					if t.State == sdl.PRESSED {
						console.CPUFreq = gbc.GBCPU_FREQ
						if !pl.slowMode {
							console.CPUFreq = gbc.GBCPU_FREQ / 2
						}
						pl.fastMode = 0
						pl.slowMode = !pl.slowMode
						if pl.slowMode {
							pl.DisplayNotification("slow mode")
						} else {
							pl.DisplayNotification("normal mode")
						}
						pl.setTitle()
					}
				case sdl.K_m:
					if t.State == sdl.PRESSED {
						console.APU.ToggleAudio()
						if console.APU.IsMuted() {
							pl.DisplayNotification("muted")
						} else {
							pl.DisplayNotification("unmuted")
						}
					}
				case sdl.K_PLUS:
					if t.State == sdl.PRESSED {
						console.APU.IncreaseAudio()
						pl.DisplayNotification(console.APU.GetVolumeString())
					}
				case sdl.K_MINUS:
					if t.State == sdl.PRESSED {
						console.APU.DecreaseAudio()
						pl.DisplayNotification(console.APU.GetVolumeString())
					}
				// Debug Flags
				case sdl.K_1:
					if t.State == sdl.PRESSED {
						console.APU.ToggleSoundChannel(1)
						if console.APU.IsChMuted(1) {
							pl.DisplayNotification("ch1 muted")
						} else {
							pl.DisplayNotification("ch1 unmuted")
						}
					}
				case sdl.K_2:
					if t.State == sdl.PRESSED {
						console.APU.ToggleSoundChannel(2)
						if console.APU.IsChMuted(2) {
							pl.DisplayNotification("ch2 muted")
						} else {
							pl.DisplayNotification("ch2 unmuted")
						}
					}
				case sdl.K_3:
					if t.State == sdl.PRESSED {
						console.APU.ToggleSoundChannel(3)
						if console.APU.IsChMuted(3) {
							pl.DisplayNotification("ch3 muted")
						} else {
							pl.DisplayNotification("ch3 unmuted")
						}
					}
				case sdl.K_4:
					if t.State == sdl.PRESSED {
						console.APU.ToggleSoundChannel(4)
						if console.APU.IsChMuted(4) {
							pl.DisplayNotification("ch4 muted")
						} else {
							pl.DisplayNotification("ch4 unmuted")
						}
					}
				// case sdl.K_b:
				// 	if t.State == sdl.PRESSED {
				// 		bgmap := console.GetBackgroundMapStr()
				// 		fmt.Println(bgmap)
				// 	}

				// GB Keys
				case sdl.K_z:
					console.Input.BackState.A = t.State == sdl.PRESSED
				case sdl.K_x:
					console.Input.BackState.B = t.State == sdl.PRESSED
				case sdl.K_RETURN:
					console.Input.BackState.START = t.State == sdl.PRESSED
				case sdl.K_BACKSPACE:
					console.Input.BackState.SELECT = t.State == sdl.PRESSED
				case sdl.K_UP:
					console.Input.BackState.UP = t.State == sdl.PRESSED
				case sdl.K_DOWN:
					console.Input.BackState.DOWN = t.State == sdl.PRESSED
				case sdl.K_LEFT:
					console.Input.BackState.LEFT = t.State == sdl.PRESSED
				case sdl.K_RIGHT:
					console.Input.BackState.RIGHT = t.State == sdl.PRESSED
				}
			}
		}

		ticks := console.Step()
		elapsed := time.Since(start)
		if int(elapsed.Milliseconds()) < console.GetMs(ticks) {
			sdl.Delay(uint32(console.GetMs(ticks) - int(elapsed.Milliseconds())))
			for sdl.GetQueuedAudioSize(pl.audioDevice) > uint32(pl.audioSpec.Freq/5) {
				// Wait for audio buffer to process remaining data (if any)
				sdl.Delay(80)
			}
		} else {
			if console.Verbose {
				fmt.Println("Emulation is too slow")
			}
		}
	}

	console.Delete()
	return nil
}
