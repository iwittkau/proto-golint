package test

import (
	"fmt"

	pb "github.com/iwittkau/proto-golint/internal/proto"
)

var t pb.Test

func init() {
	t.D = 1.0
	somefunc(&t)
}

func somefunc(t *pb.Test) interface{} {
	func(...interface{}) {}(t.GetB(), t.D)

	_, t.T = true, true
	_, t.T, _ = true, true, false
	_, _, t.T = true, true, false
	t.T, _ = true, true
	t.D = 2
	fmt.Println(
		t.B, t.D,
		t.F,
		t.I32,
		t.I64,
		t.S,
		t.T,
		t.U32,
		t.U64,
		t.Embedded,
	)
	return t.B
}
