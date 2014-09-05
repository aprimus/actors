package actor

import (
	"github.com/hishboy/gocommons/lang"
	"reflect"
)

func manageFarmer(env *ActorEnv, generator interface{},
	distChan chan Msg, limit int) {

	g := generator
	actorsLeft := limit
	dC := distChan
	deaths := make(chan Obit, 10)
	killMe := make(chan bool, 1)
	moreComing := true
	moreToSend := true
	msgQ := lang.NewQueue()
	env.AddObitHook(deaths)
	env.AddDieHook(killMe)
CLEANUP:
	for moreToSend {
		select {
		case <-killMe:
			moreToSend = false
			dlog(env, "received a killMe")
			break CLEANUP
		case m := <-dC:
			if m == nil {
				moreComing = false
				dlog(env, "Received nil, closing.")
				dC = make(chan Msg, 0)
				defer func() { close(dC) }()
			} else {
				switch m[0].(type) {
				case EndSentinel:
					moreComing = false
				default:
					msgQ.Push(m)
				}
			}
		case <-deaths:
			actorsLeft++
		}
		// Task dispatch
		if actorsLeft > 0 && msgQ.Len() > 0 {
			dlog(env, "dispatching")
			actorsLeft--
			var a *Actor
			embryo := g.(func() interface{})
			fetus := embryo()
			switch f := fetus.(type) {
			case func(Msg, *ActorEnv):
				a = env.NewActor(f)
			case FarmClass:
				a = env.NewActorFarm(f)
			case *ActorOptions:
				a = env.NewOptionedActor(f)
			default:
				dlog(env, "generator is of unsupported type",
					reflect.TypeOf(a))
			}
			if a == nil {
				elog(env, "Failed to create actor despite",
					"recognized type of", reflect.TypeOf(fetus))
			}
			a.Monitor(env.This)
			a.Send(msgQ.Poll().(Msg))
		}
		if !moreComing && msgQ.Len() == 0 {
			moreToSend = false
		}
	}
	for actorsLeft < limit {
		select {
		case <-deaths:
			actorsLeft++
		case <-killMe:
			dlog(env, "Farmer just told to die off when",
				"already in CLEANUP")
		}
	}
	env.This.Send(Msg{WorkComplete{}})
	dlog(env, "Exiting")
}
