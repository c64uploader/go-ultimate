//go:build ignore

// Run: go run examples/raw.go
// https://www.youtube.com/watch?v=1HP1CWzU6yE

package main

import (
	"context"
	"math"
	"time"

	"github.com/c64uploader/go-ultimate"
)

type vec3 struct{ x, y, z float64 }

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()
	conn, _ := client.Raw.Dial(ctx)
	defer conn.Close()

	for _, r := range [][2]uint16{{0xD020, 0}, {0xD021, 0}, {0xD011, 0x3B}, {0xD016, 0x08}, {0xD018, 0x18}} {
		_ = conn.WriteMemory(ctx, r[0], []byte{byte(r[1])})
	}

	var stars [30]vec3
	for i := range stars {
		a, s := float64(i)*0.56, 0.8+0.4*math.Sin(float64(i)*0.5)
		stars[i] = vec3{math.Cos(a) * s, math.Sin(a) * s, float64(i)/30.0 + 0.05}
	}

	ramp := []byte{0, 6, 11, 4, 14, 12, 3, 15, 1}
	bayer := []float64{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}
	for i := range bayer {
		bayer[i] /= 16.0
	}

	var torus [28800]vec3
	for p := range 240 {
		cp, sp := math.Cos(float64(p)*math.Pi/120), math.Sin(float64(p)*math.Pi/120)
		for t := range 120 {
			ct, st := math.Cos(float64(t)*math.Pi/60), math.Sin(float64(t)*math.Pi/60)
			torus[p*120+t] = vec3{(0.65 + 0.32*ct) * cp, (0.65 + 0.32*ct) * sp, 0.32 * st}
		}
	}

	ax, ay := 0.0, 0.0
	for {
		var bBuf [8000]byte
		var cBuf [1000]byte
		var zBuf [64000]float64
		var dBuf [1000]float64

		for i := range cBuf {
			cBuf[i] = 0x10
		}
		for i := range zBuf {
			zBuf[i] = -100.0
		}

		for i := range stars {
			s := &stars[i]
			s.z -= 0.015
			if s.z <= 0.05 {
				s.z = 1.05
			}
			sx, sy := int(s.x/s.z*144.0+160.0), int(s.y/s.z*72.0+100.0)
			if sx >= 0 && sx < 320 && sy >= 0 && sy < 200 {
				bBuf[sy/8*320+sx/8*8+sy%8] |= 1 << (7 - sx%8)
				zBuf[sy*320+sx] = -10.0
			}
		}

		cx, sx := math.Cos(ax), math.Sin(ax)
		cy, sy := math.Cos(ay), math.Sin(ay)

		for _, p := range torus {
			y1, z1 := p.y*cx-p.z*sx, p.y*sx+p.z*cx
			rx, ry, rz := p.x*cy+z1*sy, y1, -p.x*sy+z1*cy
			px, py := int(rx*104.0+160.0), int(ry*60.0+100.0)

			if px >= 0 && px < 320 && py >= 0 && py < 200 {
				if idx := py*320 + px; rz > zBuf[idx] {
					zBuf[idx] = rz
					depth := (rz + 1.0) * (rz + 1.0) * 0.3375
					addr, cIdx := py/8*320+px/8*8+py%8, py/8*40+px/8

					if depth > bayer[py%4*4+px%4] {
						bBuf[addr] |= 1 << (7 - px%8)
					} else if zBuf[idx] != -10.0 {
						bBuf[addr] &= ^(1 << (7 - px%8))
					}

					if depth > dBuf[cIdx] {
						dBuf[cIdx] = depth
						cBuf[cIdx] = ramp[int(math.Min(depth*8.99, 8.0))] << 4
					}
				}
			}
		}

		_ = conn.WriteMemory(ctx, 0x2000, bBuf[:])
		_ = conn.WriteMemory(ctx, 0x0400, cBuf[:])

		ax, ay = ax+0.035, ay+0.061
		time.Sleep(33 * time.Millisecond)
	}
}
