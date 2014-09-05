package actor_test

import (
	"actor"
	"fmt"
	"reflect"
)

type testAInfo struct {
	allowed []interface{}
}

func GenValidating(data []interface{}) *testAInfo {
	tAI := &testAInfo{data}
	return tAI
}

func (tS *testAInfo) Receive(msg actor.Msg,
	env *actor.ActorEnv) {

	fmt.Println(msg[0])
}

// Messages will only be passed to the actor if either
// (a) msg[0] has a type matching a type in allowed[], or
// (b) msg[0] is an integer between 0 and 10
func (tS *testAInfo) Validate(msg actor.Msg) bool {
	switch m := msg[0].(type) {
	case int:
		if m > 0 && m < 10 {
			return true
		}
	default:
		for _, datum := range tS.allowed {
			if reflect.TypeOf(msg[0]) ==
				reflect.TypeOf(datum) {
				return true
			}
		}
	}
	// Does not match a valid type
	return false
}

func ExampleActorGroup_NewOptionedActor() {
	ag := actor.NewActorGroup("ValidatorGrp")
	okMessages := make([]interface{}, 0)
	okMessages = append(okMessages, "Hello")
	tAI := GenValidating(okMessages)
	a := ag.NewOptionedActor(&actor.ActorOptions{
		Receive:   tAI.Receive,
		Validator: tAI.Validate,
	})
	messages := []interface{}{"Hi", 3.3, 6, 17}
	for _, m := range messages {
		a.SendBlocking(actor.Msg{m})
	}
	ag.GracefulActiveShutdown()
	// OUTPUT:
	// Hi
	// 6
}
