package mediaplugin

import (
	"borzGBC/gbc"
	"fmt"
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

type SDLPlugin struct {
	Window        *sdl.Window
	Renderer      *sdl.Renderer
	Surface       *sdl.Surface
	Width, Height int
	Scale         int

	AudioSpec      sdl.AudioSpec
	AudioDevice    sdl.AudioDeviceID
	SoundBuffer    []byte
	SoundBufferIdx int

	fastMode bool
	slowMode bool
}

func MakeSDLPlugin(scale int) (*SDLPlugin, error) {
	var err error

	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return nil, err
	}
	pl := &SDLPlugin{
		Width:    160,
		Height:   144,
		Scale:    scale,
		fastMode: false,
		slowMode: false,
	}

	pl.Window, pl.Renderer, err = sdl.CreateWindowAndRenderer(
		int32(pl.Width*pl.Scale), int32(pl.Height*pl.Scale), 0)
	if err != nil {
		return nil, err
	}
	pl.setTitle()

	pl.Surface, err = sdl.CreateRGBSurface(
		0, int32(pl.Width), int32(pl.Height), 32, 0xFF000000, 0x00FF0000, 0x0000FF00, 0x000000FF)
	if err != nil {
		return nil, err
	}

	pl.Renderer.SetDrawColor(0, 0, 0, 255)
	pl.Renderer.Clear()

	// Audio
	want := sdl.AudioSpec{}
	want.Freq = 44100
	want.Format = sdl.AUDIO_S8
	want.Channels = 2
	want.Samples = 2048
	pl.AudioDevice, err = sdl.OpenAudioDevice("", false, &want, &pl.AudioSpec, 0)
	if err != nil {
		return nil, err
	}
	pl.SoundBuffer = make([]byte, pl.AudioSpec.Samples*2)
	sdl.PauseAudioDevice(pl.AudioDevice, false)

	return pl, nil
}

func (pl *SDLPlugin) NotifySample(l, r int8) {
	// fmt.Printf("adding sample: %d, %d\n", l, r)
	if pl.SoundBufferIdx >= len(pl.SoundBuffer) {
		pl.SoundBufferIdx = 0
		sdl.QueueAudio(pl.AudioDevice, pl.SoundBuffer[:])
	}
	pl.SoundBuffer[pl.SoundBufferIdx] = byte(l)
	pl.SoundBuffer[pl.SoundBufferIdx+1] = byte(r)
	pl.SoundBufferIdx += 2
}

func (pl *SDLPlugin) Destroy() {
	pl.Renderer.Destroy()
	pl.Window.Destroy()
	sdl.CloseAudioDevice(pl.AudioDevice)
	sdl.Quit()
}

func (pl *SDLPlugin) SetPixel(x, y int, c uint32) {
	var r, g, b, a uint8
	r = uint8((c >> 24) & 0xFF)
	g = uint8((c >> 16) & 0xFF)
	b = uint8((c >> 8) & 0xFF)
	a = uint8(c & 0xFF)

	pixels := pl.Surface.Pixels()
	pixels[y*int(pl.Surface.Pitch)+x*int(pl.Surface.BytesPerPixel())+0] = a
	pixels[y*int(pl.Surface.Pitch)+x*int(pl.Surface.BytesPerPixel())+1] = b
	pixels[y*int(pl.Surface.Pitch)+x*int(pl.Surface.BytesPerPixel())+2] = g
	pixels[y*int(pl.Surface.Pitch)+x*int(pl.Surface.BytesPerPixel())+3] = r
}

func (pl *SDLPlugin) CommitScreen() {
	texture, err := pl.Renderer.CreateTextureFromSurface(pl.Surface)
	if err != nil {
		fmt.Println("Unable to create texture while rendering")
		return
	}
	defer texture.Destroy()

	rect := sdl.Rect{
		X: 0,
		Y: 0,
		W: int32(pl.Width * pl.Scale),
		H: int32(pl.Height * pl.Scale)}
	pl.Renderer.Copy(texture, nil, &rect)

	pl.Renderer.Present()

	pl.Renderer.SetDrawColor(0xff, 0xff, 0xff, 0xff)
	pl.Renderer.Clear()
}

func (pl *SDLPlugin) setTitle() {
	title := "BorzGBC"
	if pl.fastMode {
		title += " - FAST"
	} else if pl.slowMode {
		title += " - SLOW"
	}
	pl.Window.SetTitle(title)
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
				case sdl.K_f:
					if t.State == sdl.PRESSED {
						console.CPUFreq = gbc.GBCPU_FREQ
						if !pl.fastMode {
							console.CPUFreq = gbc.GBCPU_FREQ * 2
						}
						pl.slowMode = false
						pl.fastMode = !pl.fastMode
						pl.setTitle()
					}
				case sdl.K_g:
					if t.State == sdl.PRESSED {
						console.CPUFreq = gbc.GBCPU_FREQ
						if !pl.slowMode {
							console.CPUFreq = gbc.GBCPU_FREQ / 2
						}
						pl.fastMode = false
						pl.slowMode = !pl.slowMode
						pl.setTitle()
					}
				case sdl.K_m:
					if t.State == sdl.PRESSED {
						console.APU.ToggleAudio()
					}
				case sdl.K_PLUS:
					if t.State == sdl.PRESSED {
						console.APU.IncreaseAudio()
					}
				case sdl.K_MINUS:
					if t.State == sdl.PRESSED {
						console.APU.DecreaseAudio()
					}
				// Debug Flags
				case sdl.K_1:
					if t.State == sdl.PRESSED {
						console.APU.ToggleSoundChannel(1)
					}
				case sdl.K_2:
					if t.State == sdl.PRESSED {
						console.APU.ToggleSoundChannel(2)
					}
				case sdl.K_3:
					if t.State == sdl.PRESSED {
						console.APU.ToggleSoundChannel(3)
					}
				case sdl.K_4:
					if t.State == sdl.PRESSED {
						console.APU.ToggleSoundChannel(4)
					}
				case sdl.K_b:
					if t.State == sdl.PRESSED {
						bgmap := console.GetBackgroundMapStr()
						fmt.Println(bgmap)
					}

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
			for sdl.GetQueuedAudioSize(pl.AudioDevice) > uint32(pl.AudioSpec.Freq/2) {
				// Wait for audio buffer to process remaining data (if any)
				sdl.Delay(100)
			}
		} else {
			fmt.Println("Emulation is too slow")
		}
	}
	return nil
}
