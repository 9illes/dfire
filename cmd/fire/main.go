// +build example jsgo

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"github.com/hajimehoshi/ebiten/inpututil"
)

var (
	windowTitle = "Fire effect (ebiten) - v0.1.0"

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	// common

	screenWidth  int
	screenHeight int
	screenScale  float64

	frameCounter int

	frame        *image.RGBA
	colorPalette []color.RGBA
	pixelBuffer  Buffer

	// key binding
	keyBinding = "Key binding :\n\n" +
		"[space]\t\ttoggle effect\n" +
		"[p]\t\tSwitch between color palette\n" +
		"[Arr left]\tDecrease wind\n" +
		"[Arr right]\tIncrease wind\n" +
		"[D]\t\tWind direction\n" +
		"[Pg Up]\t\tIncrease background opacity\n" +
		"[Pg Down]\tDecrease background opacity\n" +
		"[Up]\t\tIncrease fire effect\n" +
		"[Down]\t\tDecrease fire effect\n" +
		"\n" +
		"enjoy !"

	// effect tuning
	screenCapture      = false
	showFps            = false
	effectEnable       bool
	paletteName        = "fire"
	oWindDirection     = 1
	oWindPower         int
	oLateralRnd        uint
	oEffectOpacity     uint8 = 0xFF
	oBackgroundOpacity uint8 = 0xFF
	oFireIntensity     int
)

func main() {

	flag.IntVar(&screenWidth, "w", 320, "Screen width in pixels")
	flag.IntVar(&screenHeight, "h", 240, "Screen height in pixels")
	flag.Float64Var(&screenScale, "s", 1, "Screen scale (max:4)")
	flag.UintVar(&oLateralRnd, "l", 1, "Screen height in pixels")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// ... rest of the program ...

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}

	p, err := NewPalette(colorPalettes[paletteName], oEffectOpacity, oBackgroundOpacity)
	check(err)
	colorPalette = p
	pixelBuffer = NewBuffer(screenWidth, screenHeight)
	pixelBuffer.SetLineColor(screenHeight-1, len(colorPalette)-int(oFireIntensity))

	frame = image.NewRGBA(image.Rect(0, 0, screenWidth, screenHeight))

	fmt.Println(keyBinding)

	ebiten.SetScreenTransparent(true)
	if err := ebiten.Run(effectLoop, screenWidth, screenHeight, screenScale, windowTitle); err != nil {
		log.Fatal(err)
	}

	fmt.Println("done.")
}

type rand struct {
	x, y, z, w uint32
}

func (r *rand) next() uint32 {
	// math/rand is too slow to keep 60 FPS on web browsers.
	// Use Xorshift instead: http://en.wikipedia.org/wiki/Xorshift
	t := r.x ^ (r.x << 11)
	r.x, r.y, r.z = r.y, r.z, r.w
	r.w = (r.w ^ (r.w >> 19)) ^ (t ^ (t >> 8))
	return r.w
}

var theRand = &rand{12345678, 4185243, 776511, 45411}

func effectLoop(screen *ebiten.Image) error {

	handleInputs()
	applyFuncToBufferedPixel(fireEffect)
	applyPalette(frame, pixelBuffer.Buffer, colorPalette)

	if ebiten.IsDrawingSkipped() {
		return nil
	}

	screen.ReplacePixels(frame.Pix)

	if screenCapture {
		saveFrame(png.Encode, frame, "./var/out", "png")
	}

	if showFps {
		ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %0.2f", ebiten.CurrentTPS()))
	}
	return nil
}

func applyFuncToBufferedPixel(effect func(int)) {
	for x := 0; x < screenWidth; x++ {
		for y := 1; y < screenHeight; y++ {
			p := y*screenWidth + x
			effect(p)
		}
	}
}

// INPUT | OPTIONS

func handleInputs() {
	inputToggleEffect()
	inputSwitchColorPalette()
	inputWindPower()
	inputBgOpacity()
	inputEffectPower()
	inputToggleFps()
	inputToggleScreenCapture()
}

func inputToggleEffect() {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		len(inpututil.JustPressedTouchIDs()) > 0 {

		effectEnable = !effectEnable
		bottomLinePaletteRef := 0
		if effectEnable {
			bottomLinePaletteRef = len(colorPalette) - 1
		}
		pixelBuffer.SetLineColor(screenHeight-1, bottomLinePaletteRef)
	}
}

func inputSwitchColorPalette() {
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		if paletteName == "fire" {
			paletteName = "blue"
		} else {
			paletteName = "fire"
		}
		colorPalette, _ = NewPalette(colorPalettes[paletteName], oEffectOpacity, oBackgroundOpacity)
	}
}

func inputBgOpacity() {
	minValue, maxValue := uint8(0x00), uint8(0xFF)
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		if maxValue > oBackgroundOpacity+0x01 {
			oBackgroundOpacity = oBackgroundOpacity + 0x5
		}
		colorPalette = updateEffectOpacity(colorPalette, oBackgroundOpacity)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
		if minValue < oBackgroundOpacity-0x01 {
			oBackgroundOpacity = oBackgroundOpacity - 0x5
		}
		colorPalette = updateEffectOpacity(colorPalette, oBackgroundOpacity)
	}
}

func inputToggleFps() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		showFps = !showFps
	}
}

func inputToggleScreenCapture() {
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		screenCapture = !screenCapture
	}
}

func inputEffectPower() {
	minValue, maxValue := 1, len(colorPalette)-2
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		if oFireIntensity <= maxValue {
			oFireIntensity = oFireIntensity + 1
			pixelBuffer.SetLineColor(screenHeight-1, len(colorPalette)-oFireIntensity)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		if oFireIntensity >= minValue {
			oFireIntensity = oFireIntensity - 1
			pixelBuffer.SetLineColor(screenHeight-1, len(colorPalette)-oFireIntensity)
		}
	}
}

func inputWindPower() {
	minValue, maxValue := 0, 6
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) && oWindPower > minValue {
		oWindPower = oWindPower - 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) && oWindPower < maxValue {
		oWindPower = oWindPower + 1
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyD) {
		oWindDirection = oWindDirection * -1
	}
}

// Palette

func applyPalette(frame *image.RGBA, buffer []int, colorPalette []color.RGBA) {
	for k, v := range buffer {

		// If number of color has changed
		if v >= len(colorPalette) {
			v = len(colorPalette) - 1
		}

		c := colorPalette[v]
		frame.Pix[4*k] = c.R
		frame.Pix[4*k+1] = c.G
		frame.Pix[4*k+2] = c.B
		frame.Pix[4*k+3] = c.A
	}
}

func fireEffect(p int) {

	y := int(theRand.next() & 1)

	randWind := int(theRand.next() & 3)

	x := randWind + (oWindPower * oWindDirection)

	if x == 0 {
		x = 1
	}

	z := p - x + 1

	if z < screenWidth {
		z = screenWidth
	}

	if pixelBuffer.Buffer[p]-1 > 0 {
		pixelBuffer.Buffer[z-screenWidth] = pixelBuffer.Buffer[p] - y
	} else {
		pixelBuffer.Buffer[z-screenWidth] = pixelBuffer.Buffer[0]
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func saveFrame(enc func(io.Writer, image.Image) error, im image.Image, filename string, ext string) {
	frameCounter++
	f, err := os.Create(fmt.Sprintf("%s_%d.%s", filename, frameCounter, ext))
	check(err)
	defer f.Close()

	w := bufio.NewWriter(f)
	enc(w, im)
}

type Buffer struct {
	screenWidth  int
	screenHeight int
	Buffer       []int
}

// NewBuffer
func NewBuffer(screenWidth, screenHeight int) Buffer {
	return Buffer{screenWidth, screenHeight, make([]int, screenWidth*screenHeight, screenWidth*screenHeight)}
}

func (b *Buffer) SetLineColor(y, colorRef int) {
	lineStart := y * b.screenWidth
	for x := 0; x < b.screenWidth; x++ {
		b.Buffer[lineStart+x] = colorRef
	}
}

var (
	colorPalettes = map[string][]int{
		"fire": []int{
			0x00, 0x00, 0x00,
			0x1F, 0x07, 0x07,
			0x2F, 0x0F, 0x07,
			0x47, 0x0F, 0x07,
			0x57, 0x17, 0x07,
			0x67, 0x1F, 0x07,
			0x77, 0x1F, 0x07,
			0x8F, 0x27, 0x07,
			0x9F, 0x2F, 0x07,
			0xAF, 0x3F, 0x07,
			0xBF, 0x47, 0x07,
			0xC7, 0x47, 0x07,
			0xDF, 0x4F, 0x07,
			0xDF, 0x57, 0x07,
			0xDF, 0x57, 0x07,
			0xD7, 0x5F, 0x07,
			0xD7, 0x5F, 0x07,
			0xD7, 0x67, 0x0F,
			0xCF, 0x6F, 0x0F,
			0xCF, 0x77, 0x0F,
			0xCF, 0x7F, 0x0F,
			0xCF, 0x87, 0x17,
			0xC7, 0x87, 0x17,
			0xC7, 0x8F, 0x17,
			0xC7, 0x97, 0x1F,
			0xBF, 0x9F, 0x1F,
			0xBF, 0x9F, 0x1F,
			0xBF, 0xA7, 0x27,
			0xBF, 0xA7, 0x27,
			0xBF, 0xAF, 0x2F,
			0xB7, 0xAF, 0x2F,
			0xB7, 0xB7, 0x2F,
			0xB7, 0xB7, 0x37,
			0xCF, 0xCF, 0x6F,
			0xDF, 0xDF, 0x9F,
			0xEF, 0xEF, 0xC7,
			0xFF, 0xFF, 0xFF},
		"blue": []int{
			0x00, 0x00, 0x00,
			0x00, 0x1b, 0x33,
			0x00, 0x27, 0x52,
			0x00, 0x31, 0x68,
			0x00, 0x3a, 0x7d,
			0x00, 0x43, 0x92,
			0x00, 0x50, 0xaf,
			0x00, 0x5a, 0xc4,
			0x00, 0x65, 0xd8,
			0x00, 0x6f, 0xec,
			0x00, 0x73, 0xf6,
			0x00, 0x81, 0xff,
			0x00, 0x81, 0xff,
			0x00, 0x81, 0xff,
			0x00, 0x83, 0xff,
			0x00, 0x83, 0xff,
			0x3b, 0x80, 0xff,
			0x54, 0x82, 0xff,
			0x70, 0x80, 0xff,
			0x82, 0x81, 0xff,
			0x92, 0x83, 0xff,
			0x9f, 0x85, 0xff,
			0xa3, 0x81, 0xf9,
			0xbf, 0x86, 0xfb,
			0xd1, 0x83, 0xf5,
			0xdd, 0x87, 0xf6,
			0xdd, 0x87, 0xf6,
			0xe9, 0x8a, 0xf6,
			0xe9, 0x8a, 0xf6,
			0xe9, 0x8a, 0xf6,
			0xeb, 0x86, 0xee,
			0xf9, 0x88, 0xf2,
			0xf7, 0x8a, 0xef,
			0xff, 0xad, 0xf9,
			0xff, 0xc8, 0xfa,
			0xff, 0xe0, 0xff},
	}
)

// NewPalette create a list of Color based from a listing of color values
func NewPalette(rgbs []int, oEffectOpacity uint8, oBackgroundOpacity uint8) ([]color.RGBA, error) {
	if 0 != len(rgbs)%3 {
		return nil, errors.New("incorrect number of color values : must be a mulitple of 3")
	}

	l := len(rgbs) / 3
	p := make([]color.RGBA, l)

	for i := 0; i < l; i++ {
		p[i] = color.RGBA{uint8(rgbs[i*3+0]), uint8(rgbs[i*3+1]), uint8(rgbs[i*3+2]), oEffectOpacity}
	}

	p[0].A = oBackgroundOpacity
	return p, nil
}

func updateEffectOpacity(palette []color.RGBA, opacity uint8) []color.RGBA {
	for i := 1; i < len(palette); i++ {
		palette[0].A = opacity
	}

	return palette
}
