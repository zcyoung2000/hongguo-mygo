package uiapp

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	shimmerPeriod = 1600 * time.Millisecond

	shimmerSweep        = 1100 * time.Millisecond
	shimmerBand         = float32(260)
	shimmerFrameRate    = float32(30)
	shimmerSampleOffset = 350 * time.Millisecond
)

type shimmer struct {
	base  ui.Color
	sheen ui.Color
	span  float32
	still bool
}

func pageShimmer(c *ui.Context) shimmer {
	return shimmer{
		base:  c.Theme().SurfaceHover,
		sheen: c.Theme().SurfacePressed,
	}.in(c)
}

func (s shimmer) in(c *ui.Context) shimmer {
	s.span, _ = c.Size()
	s.still = c.Preferences().ReduceMotion
	return s
}

func (s shimmer) paint(p *ui.Painter, r ui.Rect, radius float32) {
	if s.still || r.W <= 0 {
		p.Fill(r, s.base, radius)
		return
	}
	at := time.Duration(p.Now().UnixNano() % int64(shimmerPeriod))
	if at >= shimmerSweep {
		p.Fill(r, s.base, radius)
		p.After(shimmerPeriod - at)
		return
	}
	p.After(shimmerPeriod - at)
	p.AnimationFrameRate(shimmerFrameRate)
	t := float32(at) / float32(shimmerSweep)
	x := -shimmerBand/2 + t*(s.span+shimmerBand)
	if x+shimmerBand/2 < r.X || x-shimmerBand/2 > r.X+r.W {
		p.Fill(r, s.base, radius)
		return
	}
	along := func(position float32) float32 { return (position - r.X) / r.W }
	p.FillGradient(r, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
		{Color: s.base, At: along(x - shimmerBand/2)},
		{Color: s.sheen, At: along(x)},
		{Color: s.base, At: along(x + shimmerBand/2)},
	}}, radius)
}

func (s shimmer) bone(c *ui.Context, radius float32) ui.Element {
	return ui.Box(c).Shrink(0).Draw(func(p *ui.Painter, rect ui.Rect) { s.paint(p, rect, radius) })
}

func (s shimmer) text(c *ui.Context, size, lineHeight float32) ui.Element {
	return ui.Box(c).Shrink(0).Height(size * lineHeight).Draw(func(p *ui.Painter, rect ui.Rect) {
		height := size * 0.78
		s.paint(p, ui.Rect{
			X: rect.X,
			Y: rect.Y + (rect.H-height)/2,
			W: rect.W,
			H: height,
		}, height*0.32)
	})
}

var boneWidths = [...]float32{62, 44, 78, 53, 70, 38, 66, 49, 82, 57, 41, 73}

func boneWidth(index int) float32 { return boneWidths[index%len(boneWidths)] }
