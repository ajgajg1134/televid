package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
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

var minLat, maxLat, minLong, maxLong = 90., -90., 90., -90.

func main() {
	activities, err := tcx.ReadFile("Harvard_Road_Race.tcx") // Put your TCX filename here
	if err != nil {
		fmt.Println(err)
		return
	}

	coords := []coord{}

	for _, activity := range activities.Acts.Act {
		for _, lap := range activity.Laps {
			for _, trackpoint := range lap.Trk.Pt {
				minLat = math.Min(minLat, trackpoint.Lat)
				minLong = math.Min(minLat, trackpoint.Long)

				maxLat = math.Max(maxLat, trackpoint.Lat)
				maxLong = math.Max(maxLong, trackpoint.Long)
				coords = append(coords, coord{
					Lat:  trackpoint.Lat,
					Long: trackpoint.Long,
				})
			}
		}
	}

	fmt.Printf("%10.10f lat %10f %10f long %10f\n", minLat, maxLat, minLong, maxLong)

	miniMap := drawMap(coords)

	fts := []frameTelemetry{}

	for _, activity := range activities.Acts.Act {
		for _, lap := range activity.Laps {
			limit := 20
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

				// Interpolate data between each trackpoint to make video smoother
				fts = append(fts, frameTelemetry{
					Power: pt.Power,
					Speed: pt.Speed,
					HR:    pt.HR,
					Lat:   pt.Lat,
					Long:  pt.Long,
				})
				fts = append(fts, frameTelemetry{
					Power: infill(pt.Power, ptNext.Power, 0.8),
					Speed: infill(pt.Speed, ptNext.Speed, 0.8),
					HR:    infill(pt.HR, ptNext.HR, 0.8),
					Lat:   infill(pt.Lat, ptNext.Lat, 0.8),
					Long:  infill(pt.Long, ptNext.Long, 0.8),
				})
				fts = append(fts, frameTelemetry{
					Power: infill(pt.Power, ptNext.Power, 0.6),
					Speed: infill(pt.Speed, ptNext.Speed, 0.6),
					HR:    infill(pt.HR, ptNext.HR, 0.6),
					Lat:   infill(pt.Lat, ptNext.Lat, 0.6),
					Long:  infill(pt.Long, ptNext.Long, 0.6),
				})
				fts = append(fts, frameTelemetry{
					Power: infill(pt.Power, ptNext.Power, 0.4),
					Speed: infill(pt.Speed, ptNext.Speed, 0.4),
					HR:    infill(pt.HR, ptNext.HR, 0.4),
					Lat:   infill(pt.Lat, ptNext.Lat, 0.4),
					Long:  infill(pt.Long, ptNext.Long, 0.4),
				})
				fts = append(fts, frameTelemetry{
					Power: infill(pt.Power, ptNext.Power, 0.2),
					Speed: infill(pt.Speed, ptNext.Speed, 0.2),
					HR:    infill(pt.HR, ptNext.HR, 0.2),
					Lat:   infill(pt.Lat, ptNext.Lat, 0.2),
					Long:  infill(pt.Long, ptNext.Long, 0.2),
				})
			}
		}
	}

	work := make(chan *frameWork)
	wg := sync.WaitGroup{}
	numWorkers := runtime.GOMAXPROCS(0)
	fmt.Printf("Starting %d workers\n", numWorkers)
	for range numWorkers {
		wg.Go(func() {
			font, err := truetype.Parse(gomonobold.TTF)
			if err != nil {
				log.Fatal(err)
			}
			bigFace := truetype.NewFace(font, &truetype.Options{Size: 75})
			smallFace := truetype.NewFace(font, &truetype.Options{Size: 55})
			for ft := range work {
				generateImage(ft.ft, ft.i, miniMap, bigFace, smallFace)
			}
		})
	}

	for i, ft := range fts {
		if i%10 == 0 {
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
}

func generateImage(frameTelemetry *frameTelemetry, i int, staticImg image.Image, bigFace, smallFace font.Face) {
	ggCtx := gg.NewContext(1920, 1080)
	ggCtx.SetRGBA(0, 0, 0, 0)
	ggCtx.Clear()
	ggCtx.SetRGBA(0, 0, 0, 1)
	ggCtx.DrawImage(staticImg, 0, 0) // TODO: use contexts smarter to make this faster?
	//Big Fonts
	ggCtx.SetFontFace(bigFace)
	drawStringDropShadow(ggCtx, fmt.Sprintf("%3.0f mph", frameTelemetry.Speed*MPS_CONVERT), 20, 95)

	ggCtx.SetFontFace(smallFace)
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

func drawWattBox(ggCtx *gg.Context, watts float64) {
	height, width, x, y := 50., 270., 55., 950.
	grad := gg.NewLinearGradient(x, y, x+width, y+height)

	// https://uigradients.com/#KingYna
	grad.AddColorStop(0, color.RGBA{R: 26, G: 42, B: 252, A: 255})
	grad.AddColorStop(0.5, color.RGBA{R: 178, G: 31, B: 0x1f, A: 255})
	grad.AddColorStop(1, color.RGBA{R: 0xfd, G: 0xbb, B: 0x2d, A: 255})

	ggCtx.SetLineWidth(4)
	ggCtx.SetLineCapRound()
	ggCtx.SetRGB(0.1, 0.1, 0.15)
	ggCtx.DrawRectangle(x, y, width, height)
	ggCtx.Stroke()

	//Scale width. We want 30 -> 300
	calcWidth := watts / 450. * 250.
	calcWidth = math.Min(width, calcWidth+30)

	ggCtx.SetFillStyle(grad)
	ggCtx.DrawRectangle(x, y, calcWidth, height)

	ggCtx.Fill()
}

func drawHRBox(ggCtx *gg.Context, bpm float64) {
	height, width, x, y := 50., 270., 395., 950.
	grad := gg.NewLinearGradient(x, y, x+width, y+height)

	// https://uigradients.com/#Kyoto
	grad.AddColorStop(0, color.RGBA{R: 0xc2, G: 0x15, B: 0x0, A: 255})
	grad.AddColorStop(1, color.RGBA{R: 0xff, G: 0xc5, B: 0x0, A: 255})

	ggCtx.SetLineWidth(4)
	ggCtx.SetLineCapRound()
	ggCtx.SetRGB(0.15, 0.08, 0.08)
	ggCtx.DrawRectangle(x, y, width, height)
	ggCtx.Stroke()

	//Scale width. We want 30 -> 300
	calcWidth := bpm / 200. * 180.
	calcWidth = math.Min(width, calcWidth+30)

	ggCtx.SetFillStyle(grad)
	ggCtx.DrawRectangle(x, y, calcWidth, height)

	ggCtx.Fill()
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
	scaleFactor := 275. // How tall in pixels
	lat = lat - minLat
	lat = lat * normalizeFactor * -1 * scaleFactor

	//x = x / -42.291723
	long = long - minLong
	//y = y / 71.196104
	long = long * scaleFactor * normalizeFactor

	return lat + 1050, long + 1625
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

// Want 42.291441 to be 0
// want 42.291723 to be 1
