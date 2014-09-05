package actor_test

import (
	"actor"
	"fmt"
)

func FirstActor(msg actor.Msg, env *actor.ActorEnv) {
	// On first message, respond and change behavior
	switch m := msg[0].(type) {
	case chan string:
		m <- "Init()"
	}
	env.Become(SecondActor)
}

func SecondActor(msg actor.Msg, env *actor.ActorEnv) {
	switch m := msg[0].(type) {
	case chan string:
		if msg[1] == "Revert" {
			env.Revert()
			m <- "Reverting"
		} else {
			m <- msg[1].(string)
		}
	}
}

func ExampleActorEnv_Become() {

	// InitActor has two modes of operation.  When first
	// created, it will respond to any request (containing
	// chan string) by (a) sending "Init()" and (b) altering
	// behavior.  In the altered state, the actor will echo
	// any string sent, until it receives the string "Revert",
	// in which case it will respond with "Reverting" and
	// then go back to the initial behavior.

	ch := make(chan string, 5)
	ping := actor.Msg{ch, "Ping!"}
	revertMsg := actor.Msg{ch, "Revert"}
	ag := actor.NewActorGroup("BecomeAsInit")
	a := ag.NewActor(FirstActor)
	msgs := []actor.Msg{ping, ping, ping, revertMsg, ping}
	for _, m := range msgs {
		a.Send(m)
		resp := <-ch
		fmt.Println(resp)

	}
	ag.GracefulActiveShutdown()
	// OUTPUT:
	// Init()
	// Ping!
	// Ping!
	// Reverting
	// Init()
}
