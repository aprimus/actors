package actor

import (
	"lang"
	"reflect"
	"runtime"
)

// Somewhat longer than ideal for a function, but it is the
// main guts of what an actor does.
func (env *ActorEnv) mainLoop() {
	dying := false
	dead := false
	tombstone := false // Not accepting any more input
	recRunning := true // Flipped at end of activate()
	waitingOnKids := false
	burried := make(chan bool, 1)
	mqueue := lang.NewQueue()
	for !dead {
		dlog(env, "dying = ", dying, " dead = ", dead, " tstone = ",
			tombstone, " reR = ", recRunning, " wOK = ", waitingOnKids)
		select {
		// Control message
		case m := <-env.cbox:
			env.manageChildren(m, dying)
		case m := <-env.sbox:
			switch m.(type) {
			case sReceiveFinished:
				dlog(env, "Notified Received() completed")
				recRunning = false
			case sAssassin:
				dying = true
				dead = true
			case sHappyDeath:
				dlog(env, "received HappyDeath{}")
				if dying == false {
					go func() {
						dlog(env, "Sending self sTombstone{}")
						env.mbox <- Msg{sTombstone{}}
					}()
					if env.dhook != nil {
						env.dhook <- true
					}
					dying = true
				} else {
					dlog(env, "I've been told to die again!")
				}
			default:
				elog(env, "received an unhandled message",
					"of type", m)
				runtime.Goexit()
			}
		case m := <-env.mbox:
			switch m[0].(type) {
			case sTombstone:
				dlog(env, "Received sTombstone{}")
				tombstone = true
			default:
				mqueue.Push(m)
			}
		}
		if !recRunning && mqueue.Len() > 0 {
			recRunning = true
			zz := mqueue.Poll().(Msg)
			env.runMsg(zz)
		}
		if tombstone && mqueue.Len() == 0 && !waitingOnKids {
			dlog(env, "Calling killMyKids()")
			go env.This.killMyKids(burried)
			if len(env.This.children) > 0 {
				waitingOnKids = true
			} else {
				dead = true
			}
		}
		if waitingOnKids {
			if len(env.This.children) == 0 {
				dlog(env, "waitingOnKids = true. All dead.  Setting false")
				dead = true
			} else {
				dlog(env, "waitingOnKids = true. Remaining:", len(env.This.children))
			}
		}

		runtime.Gosched()
	}
	dlog(env, "Entering recRunning loop")
	for recRunning {
		msg := <-env.sbox
		switch msg.(type) {
		case sReceiveFinished:
			recRunning = false
		default:
			//
		}
	}
	dlog(env, "Waiting for burried")
	<-burried
	dlog(env, "Exiting")
	return
}

// This runs singly from mainLoop(), no race conditions
func (env *ActorEnv) manageChildren(msg interface{}, dying bool) {

	switch m := msg.(type) {
	case cAddChild:
		// Need to check that the name does not conflict.
		if _, ok := env.This.children[m.id]; ok || dying {
			// Actor already exists or instructed to make
			// no more actors
			m.resp <- false
		} else {
			env.This.children[m.id] = m.a
			m.resp <- true
		}
	case Obit:
		if env.msgObit {
			env.This.Send(Msg{m})
		} else if env.ohook != nil {
			go func() { env.ohook <- m }()
		} else {
			dlog(env, " received an obit, but has no handler")
		}
	case cRemoveChild:
		delete(env.This.children, m.id)
		m.ch <- true
	case cFindMember:
		child := env.This.children[m.fname]
		m.resp <- child
	case cAddWatcher:
		env.This.watchers[m.a] = tEmptyStruct{}
	case cDelWatcher:
		delete(env.This.watchers, m.a)
	default:
		elog(env, "received an unhandled message of type",
			reflect.TypeOf(m))
		runtime.Goexit()
	}
	return
}

func genDispatchFn(env *ActorEnv) func(msg Msg) {
	return func(msg Msg) {
		env.This.parent.Send(msg)
	}
}

func (env *ActorEnv) runMsg(msg Msg) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				dlog(env, "Child Died: ", r)
				env.Return(Msg{ChildDied{r, env.This, msg}})
			}
			dlog(env, "sReceiveFinished{} to")
			env.sbox <- sReceiveFinished{}
		}()
		switch b := env.behavior.(type) {
		case func(Msg, *ActorEnv):
			b(msg, env)
		case Receive:
			b(msg, env)
		case FarmReceive:
			b(msg, env, genDispatchFn(env))
		}
	}()
}
