package fb

import (
	"testing"
)

func BenchmarkBlit16(b *testing.B) {
	d := NewTestDevice(800, 480, 16, false)
	img := NewTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}

func BenchmarkBlit16Rotate(b *testing.B) {
	d := NewTestDevice(800, 480, 16, true)
	img := NewTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}

func BenchmarkBlit32(b *testing.B) {
	d := NewTestDevice(800, 480, 32, false)
	img := NewTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}

func BenchmarkBlit32Rotate(b *testing.B) {
	d := NewTestDevice(800, 480, 32, true)
	img := NewTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}
