package stringgenerator_test

// This example shows a simple use of the stringgenerator

import (
	"fmt"
	"github.com/aprimus/actor/stringgenerator"
)

func ExampleNewGenerator() {
	gen, _ := stringgenerator.NewGenerator("MyPrefix")
	for i := 0; i < 5; i++ {
		s := <-gen
		fmt.Println(s)
	}
	// OUTPUT:
	// MyPrefix-0
	// MyPrefix-1
	// MyPrefix-2
	// MyPrefix-3
	// MyPrefix-4
}
