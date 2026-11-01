package main

import "testing"

// O joystick manda ±128 ao empurrar e 0 ao soltar: cada "desce" precisa de um "sobe".
func TestEmitAxisSoltaATecla(t *testing.T) {
	var ax int32
	ch := make(chan Event, 64)
	for _, v := range []int32{128, 0, 128, 0, -128, 126, 0, -128, 0} {
		emitAxis(ch, &ax, v, KeyLeft, KeyRight)
	}
	close(ch)
	want := []Event{
		{KeyRight, true}, {KeyRight, false},
		{KeyRight, true}, {KeyRight, false},
		{KeyLeft, true}, {KeyLeft, false}, {KeyRight, true}, {KeyRight, false},
		{KeyLeft, true}, {KeyLeft, false},
	}
	i := 0
	for e := range ch {
		if i >= len(want) || e != want[i] {
			t.Fatalf("evento %d = %+v, esperado %+v", i, e, want[min(i, len(want)-1)])
		}
		i++
	}
	if i != len(want) {
		t.Fatalf("%d eventos, esperado %d", i, len(want))
	}
}
