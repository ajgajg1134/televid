package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/philhofer/tcx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomonobold"
)

// 1 m/s = 2.2369 mph
const MPS_CONVERT = 2.2369

// 1 m/s = 3.6 kph
const KMPH_CONVERT = 3.6

var minLat, maxLat, minLong, maxLong = 90., -90., 90., -90.

func main() {
	// Uncomment to get cpu profiles for pprof
	//prof, err := os.Create("profile.pprof")
	// defer prof.Close()
	// if err != nil {
	// 	panic(err)
	// }
	// pprof.StartCPUProfile(prof)
	// defer pprof.StopCPUProfile()
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <TCX filename>")
		return
	}

	filename := os.Args[1]

	activities, err := tcx.ReadFile(filename)
	if err != nil {
		fmt.Println(err)
		return
	}

	//Skip empty start off course
	activities.Acts.Act[0].Laps[0].Trk.Pt = activities.Acts.Act[0].Laps[0].Trk.Pt[0:]

	//TODO: Fix extra distance from rolling past end

	coords := []coord{}
	fullDist := 0.0

	// TODO: Move the map builder to use the interpolated points for a cleaner looking map
	for _, activity := range activities.Acts.Act {
		for _, lap := range activity.Laps {
			for _, trackPoint := range lap.Trk.Pt {
				fullDist = math.Max(float64(fullDist), trackPoint.Dist)

				minLat = math.Min(minLat, trackPoint.Lat)
				minLong = math.Min(minLat, trackPoint.Long)

				maxLat = math.Max(maxLat, trackPoint.Lat)
				maxLong = math.Max(maxLong, trackPoint.Long)
				coords = append(coords, coord{
					Lat:  trackPoint.Lat,
					Long: trackPoint.Long,
				})
			}
		}
	}

	// Adjust full dist to remove any extra recording after the finish line
	fullDist -= 101

	//fmt.Printf("%10.10f lat %10f %10f long %10f\n", minLat, maxLat, minLong, maxLong)

	miniMap := drawMap(coords)

	fts := []frameTelemetry{}

	lapCounter := 1
	startLat := activities.Acts.Act[0].Laps[0].Trk.Pt[0].Lat
	startLong := activities.Acts.Act[0].Laps[0].Trk.Pt[0].Long
	distLatch := 0.0001 // How far away a point can be before it triggers a new lap
	lapLatch := true

	for _, activity := range activities.Acts.Act {
		for _, lap := range activity.Laps {
			limit := 200_000 // TO ADJUST LIMIT
			for i := range len(lap.Trk.Pt) - 1 {
				if i >= limit {
					break
				}
				pt := lap.Trk.Pt[i]
				ptNext := lap.Trk.Pt[i+1]

				timeDiff := ptNext.Time.Sub(pt.Time)
				if timeDiff > time.Second {
					// Verify we have a track point for every second otherwise the resulting video will be messed up
					// TODO: actually just handle this
					fmt.Printf("SAW BIG TIME %v at %d. The resulting video will be messed up", timeDiff, i)
				}

				// Lap detection
				distToLapStart := dist(startLat, startLong, pt.Lat, pt.Long)
				if distToLapStart < distLatch && !lapLatch {
					// new lap detected
					lapLatch = true
					fmt.Printf("New lap found at %d\n", i)
					lapCounter++
				} else if distToLapStart > distLatch && lapLatch {
					// far enough from lap start toggle the latch
					lapLatch = false
				}

				// Interpolate data between each trackpoint to make video smoother
				fts = append(fts, frameTelemetry{
					Power: pt.Power,
					Speed: pt.Speed,
					HR:    pt.HR,
					Lat:   pt.Lat,
					Long:  pt.Long,
					Dist:  pt.Dist,
					Lap:   lapCounter,
				})
				fts = append(fts, frameTelemetry{
					Power: infill(pt.Power, ptNext.Power, 0.8),
					Speed: infill(pt.Speed, ptNext.Speed, 0.8),
					HR:    infill(pt.HR, ptNext.HR, 0.8),
					Lat:   infill(pt.Lat, ptNext.Lat, 0.8),
					Long:  infill(pt.Long, ptNext.Long, 0.8),
					Dist:  infill(pt.Dist, ptNext.Dist, 0.8),
					Lap:   lapCounter,
				})
				fts = append(fts, frameTelemetry{
					Power: infill(pt.Power, ptNext.Power, 0.6),
					Speed: infill(pt.Speed, ptNext.Speed, 0.6),
					HR:    infill(pt.HR, ptNext.HR, 0.6),
					Lat:   infill(pt.Lat, ptNext.Lat, 0.6),
					Long:  infill(pt.Long, ptNext.Long, 0.6),
					Dist:  infill(pt.Dist, ptNext.Dist, 0.6),
					Lap:   lapCounter,
				})
				fts = append(fts, frameTelemetry{
					Power: infill(pt.Power, ptNext.Power, 0.4),
					Speed: infill(pt.Speed, ptNext.Speed, 0.4),
					HR:    infill(pt.HR, ptNext.HR, 0.4),
					Lat:   infill(pt.Lat, ptNext.Lat, 0.4),
					Long:  infill(pt.Long, ptNext.Long, 0.4),
					Dist:  infill(pt.Dist, ptNext.Dist, 0.4),
					Lap:   lapCounter,
				})
				fts = append(fts, frameTelemetry{
					Power: infill(pt.Power, ptNext.Power, 0.2),
					Speed: infill(pt.Speed, ptNext.Speed, 0.2),
					HR:    infill(pt.HR, ptNext.HR, 0.2),
					Lat:   infill(pt.Lat, ptNext.Lat, 0.2),
					Long:  infill(pt.Long, ptNext.Long, 0.2),
					Dist:  infill(pt.Dist, ptNext.Dist, 0.2),
					Lap:   lapCounter,
				})
			}
		}
	}

	numLaps := fts[len(fts)-1].Lap

	work := make(chan *frameWork)
	wg := sync.WaitGroup{}
	numWorkers := runtime.GOMAXPROCS(0)
	fmt.Printf("Starting %d workers\n", numWorkers)
	for range numWorkers {
		wg.Go(func() {
			var bigFace font.Face
			var smallFace font.Face
			fontFilePath := "./OpenSans-Medium.ttf"
			if fontBts, err := os.ReadFile(fontFilePath); err == nil {
				fmt.Printf("Found font file!\n")
				font, err := truetype.Parse(fontBts)
				if err != nil {
					log.Fatal(err)
				}
				bigFace = truetype.NewFace(font, &truetype.Options{Size: 65})
				smallFace = truetype.NewFace(font, &truetype.Options{Size: 45})
			} else {
				fmt.Printf("No custom font file, falling back to gomono!\n")
				// no custom font, fallback to gomono
				font, err := truetype.Parse(gomonobold.TTF)
				if err != nil {
					log.Fatal(err)
				}
				bigFace = truetype.NewFace(font, &truetype.Options{Size: 65})
				smallFace = truetype.NewFace(font, &truetype.Options{Size: 45})
			}
			for ft := range work {
				generateImage(ft.ft, ft.i, miniMap, bigFace, smallFace, fullDist, numLaps)
			}
		})
	}

	for i, ft := range fts {
		if i%100 == 0 {
			fmt.Printf("%d / %d %0.2f%%\n", i, len(fts), float64(i)/float64(len(fts))*100.0)
		}
		work <- &frameWork{ft: &ft, i: i}
	}
	close(work)
	wg.Wait()
	fmt.Println("Done.")
}

func infill(a, b float64, ratA float64) float64 {
	return (a * ratA) + (b * (1 - ratA))
}

type frameWork struct {
	ft *frameTelemetry
	i  int
}

type frameTelemetry struct {
	Power float64
	Speed float64
	HR    float64
	Lat   float64
	Long  float64
	Dist  float64
	Lap   int
}

func generateImage(frameTelemetry *frameTelemetry, i int, staticImg image.Image, bigFace, smallFace font.Face, fullDist float64, numLaps int) {
	ggCtx := gg.NewContextForImage(staticImg)
	// ggCtx.SetRGBA(0, 0, 0, 0)
	// ggCtx.Clear()
	ggCtx.SetRGBA(0, 0, 0, 1)
	ggCtx.SetFontFace(bigFace)
	drawStringDropShadow(ggCtx, fmt.Sprintf("%3.0f mph", frameTelemetry.Speed*MPS_CONVERT), 20, 95)
	drawStringDropShadow(ggCtx, fmt.Sprintf("%3.0f m", math.Min(fullDist-frameTelemetry.Dist, 0)), 1550, 95)

	ggCtx.SetFontFace(smallFace)
	drawStringDropShadow(ggCtx, fmt.Sprintf("%3.0f", frameTelemetry.Speed*KMPH_CONVERT), 50, 160)
	drawStringDropShadow(ggCtx, "kph", 170, 160)

	drawStringDropShadow(ggCtx, fmt.Sprintf("lap %d / %d", frameTelemetry.Lap, numLaps), 1590, 160)

	drawStringDropShadow(ggCtx, fmt.Sprintf("%04.0f w", frameTelemetry.Power), 55, 1050)
	drawWattBox(ggCtx, frameTelemetry.Power)

	drawStringDropShadow(ggCtx, fmt.Sprintf("%3.f bpm", frameTelemetry.HR), 395, 1050)
	drawHRBox(ggCtx, frameTelemetry.HR)

	ggCtx.SetRGBA(0.2, 0.2, 0.7, 1) // GPS point color
	gx, gy := scaleGPS(frameTelemetry.Lat, frameTelemetry.Long)
	ggCtx.DrawCircle(gy, gx, 8)
	ggCtx.Fill()

	ggCtx.SavePNG(fmt.Sprintf("imgs/%04d.png", i))
}

// GradientStop configures a color stop for the gradient
type GradientStop struct {
	Offset float64
	Color  color.RGBA
}

func drawBox(ggCtx *gg.Context, value float64, x, y, height, width float64, strokeColor color.RGBA, maxVal, displayMax float64, stops []GradientStop) {
	grad := gg.NewLinearGradient(x, y, x+width, y+height)

	for _, stop := range stops {
		grad.AddColorStop(stop.Offset, stop.Color)
	}

	ggCtx.SetLineWidth(4)
	ggCtx.SetLineCapRound()
	// Convert 0-255 to 0-1 for SetRGB
	ggCtx.SetRGB(float64(strokeColor.R)/255.0, float64(strokeColor.G)/255.0, float64(strokeColor.B)/255.0)
	ggCtx.DrawRectangle(x, y, width, height)
	ggCtx.Stroke()

	//Scale width. We want 30 -> 300
	calcWidth := value / maxVal * displayMax
	calcWidth = math.Min(width, calcWidth+30)

	ggCtx.SetFillStyle(grad)
	ggCtx.DrawRectangle(x, y, calcWidth, height)

	ggCtx.Fill()
}

func drawWattBox(ggCtx *gg.Context, watts float64) {
	height, width, x, y := 50., 270., 55., 950.
	strokeColor := color.RGBA{R: 26, G: 26, B: 26, A: 255} // Approximation from 0.1, 0.1, 0.15 * 255. Actual values: 25, 25, 38
	// Correct stroke color based on original: 0.1, 0.1, 0.15 -> 25.5, 25.5, 38.25
	strokeColor = color.RGBA{R: 26, G: 26, B: 38, A: 255}

	stops := []GradientStop{
		{0, color.RGBA{R: 26, G: 42, B: 252, A: 255}},
		{0.5, color.RGBA{R: 178, G: 31, B: 31, A: 255}},
		{1, color.RGBA{R: 253, G: 187, B: 45, A: 255}},
	}

	drawBox(ggCtx, watts, x, y, height, width, strokeColor, 450.0, 250.0, stops)
}

func drawHRBox(ggCtx *gg.Context, bpm float64) {
	height, width, x, y := 50., 270., 395., 950.
	strokeColor := color.RGBA{R: 38, G: 20, B: 20, A: 255} // Approximation from 0.15, 0.08, 0.08 * 255
	// Correct stroke color based on original: 0.15, 0.08, 0.08 -> 38.25, 20.4, 20.4
	strokeColor = color.RGBA{R: 38, G: 20, B: 20, A: 255}

	stops := []GradientStop{
		{0, color.RGBA{R: 194, G: 21, B: 0, A: 255}},
		{1, color.RGBA{R: 255, G: 197, B: 0, A: 255}},
	}

	drawBox(ggCtx, bpm, x, y, height, width, strokeColor, 200.0, 180.0, stops)
}

func drawStringDropShadow(ggCtx *gg.Context, s string, x, y float64) {
	shadowDepth := 4.
	// Shadow color
	ggCtx.SetRGBA(0, 0, 0, 1)
	ggCtx.DrawString(s, x+shadowDepth, y+shadowDepth)
	//Text color
	ggCtx.SetRGBA(0.9, 0.9, 0.9, 1)
	ggCtx.DrawString(s, x, y)
}

func scaleGPS(lat float64, long float64) (float64, float64) {
	// technically this should probably use minLat and minLong and the scale factor is 1 / (max - min)
	normalizeFactor := (1. / (maxLat - minLat))
	scaleFactor := 200. // How tall in pixels the minimap is
	lat = lat - minLat
	lat = lat * normalizeFactor * -1 * scaleFactor

	//x = x / -42.291723
	long = long - minLong
	//y = y / 71.196104
	long = long * scaleFactor * normalizeFactor

	// Adjust these numbers to move the minimap around
	return lat + 1050, long + 1650
}

type coord struct {
	Lat  float64
	Long float64
}

func drawMap(coords []coord) image.Image {
	ggCtx := gg.NewContext(1920, 1080)
	ggCtx.SetRGBA(0, 0, 0, 0)
	ggCtx.Clear()
	//minimap color
	ggCtx.SetRGBA(0.1, 0.1, 0.2, 1)

	for _, coord := range coords {
		gx, gy := scaleGPS(coord.Lat, coord.Long)
		// fmt.Printf("drawing point at %f, %f\n", gx, gy)
		ggCtx.DrawCircle(gy, gx, 5)
		ggCtx.Fill()
	}

	return ggCtx.Image()
}

func dist(a1, b1, a2, b2 float64) float64 {
	return math.Sqrt(math.Pow(a2-a1, 2) + math.Pow(b2-b1, 2))
}
