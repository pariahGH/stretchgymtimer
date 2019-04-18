package main
import (
	"fmt"
	"os"
	"io"
	"time"
	"runtime"
	"github.com/go-gl/gl/v3.2-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/golang-ui/nuklear/nk"
	"github.com/xlab/closer"
	"github.com/youpy/go-wav"
	"github.com/gen2brain/malgo"
	)
	
const (
	winHeight = 300
	winWidth = 400
	maxVertexBuffer  = 512 * 1024
	maxElementBuffer = 128 * 1024
)

func init() {
	runtime.LockOSThread()
}

func main(){
	if err := glfw.Init(); err != nil {
		fmt.Println(err)
	}
	glfw.WindowHint(glfw.Resizable, glfw.False)
	win, err := glfw.CreateWindow(winWidth, winHeight, "StretchGymTimer", nil, nil)
	if err != nil {
		fmt.Println(err)
	}
	win.MakeContextCurrent()
	if err := gl.Init(); err != nil {
		closer.Fatalln("opengl: init failed:", err)
	}
	
	ctx := nk.NkPlatformInit(win, nk.PlatformInstallCallbacks)
	atlas := nk.NewFontAtlas()
	nk.NkFontStashBegin(&atlas)
	sansfont := nk.NkFontAtlasAddDefault(atlas, 14,nil)
	nk.NkFontStashEnd()
	if sansfont != nil {
		nk.NkStyleSetFont(ctx, sansfont.Handle())
	}
	exitC := make(chan struct{}, 1)
	doneC := make(chan struct{}, 1)
	closer.Bind(func() {
		close(exitC)
		<-doneC
	})
	currentTime := time.Now()
	duration, _ := time.ParseDuration("1h")
	currentTimePlusOne := currentTime.Add(duration)
	state := &State{
		Interval: 1,
		GymOn: true,
		StretchOn: true,
		GymTime: 19,
		GymAlert: false,
		TimerAlert: false,
		StartTime: currentTime,
		NextStretchTime: currentTimePlusOne,
		GymAknowledged: false,
		TimerAknowledged: false,
	}
	fpsTicker := time.NewTicker(time.Second / 30)
	for {
		select {
		case <-exitC:
			nk.NkPlatformShutdown()
			glfw.Terminate()
			fpsTicker.Stop()
			close(doneC)
			return
		case <-fpsTicker.C:
			if win.ShouldClose() {
				close(exitC)
				continue
			}
			glfw.PollEvents()
			gfxMain(win, ctx, state)
			checkTimers(state)
		}
	}
}

func checkTimers(state *State){
	interval := time.Hour * time.Duration(state.Interval)
	//if aligns are set, we align starttime to half hour or hour before continuing - else proceed normally
	/*if state.AlignTimerToHour {
		
	}
	if state.AlignTimerToHalfHour {
	}
	if state.FreefloatingTimer {
	
	}*/
	if time.Since(state.StartTime) >= interval && state.StretchOn{
		state.TimerAlert = true
		state.TimerAknowledged = false
		state.StartTime = time.Now()
		go playSound()
	}
	if int32(time.Now().Hour()) == state.GymTime && time.Now().Minute() == 0 && state.GymOn{
		state.GymAlert = true
		state.GymAknowledged = false
		go playSound()
	}
	//these need to update every frame in case of changes
	state.NextStretchTime = state.StartTime.Add(interval)
}

func gfxMain(win *glfw.Window, ctx *nk.Context, state *State){
	nk.NkPlatformNewFrame()

	bounds := nk.NkRect(0, 0, winWidth, winHeight)
	if nk.NkBegin(ctx, "", bounds, nk.WindowBorder) > 0 {
		nk.NkLayoutRowDynamic(ctx, 30, 1)
		nk.NkLabel(ctx, "Select intervals and turn timers on/off", nk.TextLeft)
	
		nk.NkLayoutRowDynamic(ctx, 30, 1)
		hour, minute, sec := state.NextStretchTime.Clock()
		nk.NkLabel(ctx, "Next stretch at: "+fmt.Sprintf("%d:%d:%d",hour, minute, sec), nk.TextLeft)
		
		nk.NkLayoutRowDynamic(ctx, 30, 2)
		nk.NkPropertyInt(ctx, "Stretch Interval:", 0, &state.Interval, 5, 1, 1)
		
		nk.NkLayoutRowDynamic(ctx, 30, 2)
		if nk.NkOptionLabel(ctx, "Timer On", flag(state.StretchOn == true)) > 0 {
			state.StretchOn = true
		}
		if nk.NkOptionLabel(ctx, "Timer Off", flag(state.StretchOn == false)) > 0 {
			state.StretchOn = false
		}
		//May or may not decide to add alignment options for timer...
		/*
		nk.NkLayoutRowDynamic(ctx, 30, 1)
		nk.NkLabel(ctx, "Align timer to hour, half hour, or freestanding", nk.TextLeft)
		*/
		nk.NkLayoutRowDynamic(ctx, 30, 2)
		nk.NkPropertyInt(ctx, "Gym Time (24hr):", 0, &state.GymTime, 23, 1, 1)
		
		nk.NkLayoutRowDynamic(ctx, 30, 2)
		if nk.NkOptionLabel(ctx, "Gym On", flag(state.GymOn == true)) > 0 {
			state.GymOn = true
		}
		if nk.NkOptionLabel(ctx, "Gym Off", flag(state.GymOn == false)) > 0 {
			state.GymOn = false
		}

	}
	
	nk.NkEnd(ctx)
	bg := make([]float32, 4)
	width, height := win.GetSize()
	gl.Viewport(0, 0, int32(width), int32(height))
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.ClearColor(bg[0], bg[1], bg[2], bg[3])
	nk.NkPlatformRender(nk.AntiAliasingOn, maxVertexBuffer, maxElementBuffer)
	win.SwapBuffers()
}

type State struct {
	Interval int32
	GymOn bool
	StretchOn bool
	GymTime int32
	GymAlert bool
	TimerAlert bool
	StartTime time.Time
	NextStretchTime time.Time
	GymAknowledged bool
	TimerAknowledged bool
}

func flag(v bool) int32 {
	if v {
		return 1
	}
	return 0
}

func playSound(){
	file, err := os.Open("alert.wav")
	stat, err := file.Stat()
	if err != nil {
		fmt.Println(err)
	}
	fileSize := stat.Size()
	if err != nil {
		fmt.Println(err)
	}

	defer file.Close()

	reader := wav.NewReader(file)
	f, err := reader.Format()
	if err != nil {
		fmt.Println(err)
	}
	channels := uint32(f.NumChannels)
	sampleRate := f.SampleRate
	playbackLength := fileSize/int64(f.ByteRate)
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		fmt.Printf("LOG <%v>\n", message)
	})
	if err != nil {
		fmt.Println(err)
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	deviceConfig := malgo.DefaultDeviceConfig()
	deviceConfig.Format = malgo.FormatS16
	deviceConfig.Channels = channels
	deviceConfig.SampleRate = sampleRate
	deviceConfig.Alsa.NoMMap = 1

	sampleSize := uint32(malgo.SampleSizeInBytes(deviceConfig.Format))
	// This is the function that's used for sending more data to the device for playback.
	onSendSamples := func(frameCount uint32, samples []byte) uint32 {
		n, _ := io.ReadFull(reader, samples)
		return uint32(n) / uint32(channels) / sampleSize
	}

	deviceCallbacks := malgo.DeviceCallbacks{
		Send: onSendSamples,
		Stop: func(){fmt.Println("done")},
	}
	device, err := malgo.InitDevice(ctx.Context, malgo.Playback, nil, deviceConfig, deviceCallbacks)
	if err != nil {
		fmt.Println(err)
	}
	defer device.Uninit()

	err = device.Start()
	if err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Second * time.Duration(playbackLength))
}