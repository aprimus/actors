package actor

import (
	"time"
)

// ActorEnv is passed to receive() along with a given message,
// and provides access to environmental resources the actor
// may need.  ActorEnv provides some utility methods for altering
// an actor's behavior.  With the exception of Suicide(), these
// are designed for implementing monitoring systems within the
// framework, and are unlikely to be useful to general application
// programmers.
type ActorEnv struct {
	This        *Actor
	behaviors   []interface{} // Two types
	behavior    interface{}   // Two types
	mbox        chan Msg
	cbox        chan interface{}
	sbox        chan interface{}
	farmRec     *tFarmData
	ohook       chan Obit
	msgObit     bool
	dhook       chan bool
	deathTimer  *time.Timer
	lastMessage Msg
}

/* These functions are usable by an agent to change it's
behavior */

// Because these functions can only be called from within the
// actor, which runs serially, there are no race
// conditions.  Updates can be done directly.

// AddObitHook informs the framework that the actor wishes to be
// informed of death notices.  If set to a non-nil value, the death
// of any monitored actor will be sent along to the specified channel
func (env *ActorEnv) AddObitHook(ch chan Obit) {
	env.ohook = ch
}

// Stops the framework from sending the actor any future death notices.
func (env *ActorEnv) RemoveObitHook() {
	env.ohook = nil
}

// SetObitForward(true) will cause any death notices received from
// monitored actors to be sent to the actor using the normal message
// system.  If this is set to true, the ObitHook will be ignored.
func (env *ActorEnv) SetObitForward(state bool) {
	env.msgObit = true
}

// AddDieHook specifies a channel on which the actor framework can
// notify an actor that it should terminate.  This can be used to
// interrupt long-running processes or for other lower level actions
func (env *ActorEnv) AddDieHook(ch chan bool) {
	env.dhook = ch
}

// Suicide() will cause an actor to gracefully die.
// This process shuts down the actor as follows:
// 1. The actor accepts no new incoming message
// 2. The actor finishes processing any messages already queued
// 3. The actor sends Die() messages to all living children
// 4. The actor waits until the children have died
// 5. Obit notices (if applicable) are sent
// 6. Death
//
func (env *ActorEnv) Suicide() {
	safeSend(env.sbox, sHappyDeath{}, env, "ActorEnv.Suicide()")
}

// Become allows an actor to change it's behavior.  A normal use of
// Become is to have an actor perform one set of actions when first
// invoked, and a different set of actions on further messages.
func (env *ActorEnv) Become(rec Receive) {
	env.behaviors = append(env.behaviors, env.behavior)
	env.behavior = rec
}

// Revert modifies an actors behavior, undoing the effect of the
// prior Become() call.  Behaviors are kept in a stack, reverted
// to in LIFO order.
func (env *ActorEnv) Revert() bool {
	if len(env.behaviors) == 0 {
		return false
	}
	last := len(env.behaviors) - 1
	env.behavior = env.behaviors[last]
	env.behaviors = env.behaviors[0:last]
	return true
}

// Return allows an actor to send a message to its parent.
func (env *ActorEnv) Return(msg Msg) {
	env.This.parent.Send(msg)
}

// GetChildrensNames() returns a slice with pointers
// to the names of all children.
func (env *ActorEnv) GetChildrensNames() []string {
	r := make([]string, 1)
	for _, k := range env.This.children {
		r = append(r, k.Id)
	}
	return r
}

// GetChildren() returns a slice with pointers to all
// children.
func (env *ActorEnv) GetChildren() []*Actor {
	alist := make([]*Actor, 0)
	for _, a := range env.This.children {
		alist = append(alist, a)
	}
	return alist
}

// NewActorFarm takes a generator and a channel.  For each message
// received from src, it will spawn off a new actor and pass the
// message along.  The farm will have a maximum of ''limit'' workers
// running at any time.
func (env *ActorEnv) NewActorFarm(farm FarmClass) *Actor {
	farmer := env.NewNamedActorFarm(env.This.Group.getGUID(), farm)
	return farmer
}

func (env *ActorEnv) NewNamedActorFarm(n string, f FarmClass) *Actor {
	farmerActor := env.NewNamedActor(n,
		genFarmReceiveAdaptor(f.GetDistChan(), f.GenFarmer()))
	go manageFarmer(farmerActor.env, f.GenWorker,
		f.GetDistChan(), f.GetMaxWorkers())
	return farmerActor
}

// NewNamedActor will create a new actor as a child of
// the caller.
func (env *ActorEnv) NewActor(receive Receive) *Actor {
	child := env.NewNamedActor(env.This.Group.getGUID(), receive)
	return child
}

// NewNamedActor will create a new actor as a child of
// the caller.
func (env *ActorEnv) NewNamedActor(n string,
	receive interface{}) *Actor {

	newAct, ok := env.newChild(n, receive)
	if ok == false {
		return nil
	}
	return newAct
}

func (env *ActorEnv) NewOptionedActor(aO *ActorOptions) *Actor {
	return env.NewNamedOptionedActor(env.This.Group.getGUID(), aO)
}

func (env *ActorEnv) NewNamedOptionedActor(n string,
	aO *ActorOptions) *Actor {

	var newActor *Actor
	switch {
	case aO.Receive != nil:
		var ok bool
		newActor, ok = env.newChild(n, aO.Receive)
		if !ok {
			elog(env, "Failed to create actor using "+
				"specified Receive", n)
			return nil
		}
	case aO.Farm != nil:
		newActor = env.NewNamedActorFarm(n, aO.Farm)
		if newActor == nil {
			elog(env, "Failed to create farm", n)
			return nil
		}
	default:
		elog(env, "ActorOptions does not have a valid "+
			"constructor (Receive or Farm)")
		return nil
	}
	newActor.validator = aO.Validator

	if aO.AgeOut != 0 {
		timer := time.AfterFunc(aO.AgeOut, func() {
			defer recover()
			newActor.env.Suicide()
		})
		newActor.env.deathTimer = timer
	}

	if aO.FirstMessage != nil {
		newActor.SendBlocking(aO.FirstMessage)
	}

	if aO.LastMessage != nil {
		newActor.env.lastMessage = aO.LastMessage
	}
	return newActor
}

// SpawnFromFunc allows programmatic spawning of multiple new
// child agents. Each agent is created with the receiver
// returned by calling generator(i).  More complex auto-spawning
// can be done using NewActorFarm()
//
// See (*ActorGroup) SpawnFromFunc() for an example of usage
func (env *ActorEnv) SpawnFromFunc(num int,
	generator func(int) Receive) {

	for i := 0; i < num; i++ {
		env.NewActor(generator(i))
	}
}

func (env *ActorEnv) DBSet(key string, val interface{}) DBResp {
	return env.This.Group.dbSet(key, val)
}

func (env *ActorEnv) DBGet(key string) DBResp {
	return env.This.Group.dbGet(key)
}

func (env *ActorEnv) DBInsert(key string, val interface{}) DBResp {
	return env.This.Group.dbInsert(key, val)
}

// DBMutate allows an atomic update on a value within the
// group's key/value data store.  The current value of key
// is fed into mutator, and, if mutator's boolean return value
// is true, the new value is put into the db
//
// WARNING -- Mutators should be entirely non-blocking and not
// request any external information.  If a mutator has any blocking
// action, it is possible it will deadlock the group's DB.
func (env *ActorEnv) DBMutate(key string,
	mutator func(interface{}) (interface{}, bool)) DBResp {
	dlog(env, "Entered")

	return env.This.Group.dbMutate(key, mutator)
}

/* These are internal-only functions */

func (env *ActorEnv) GoString() string {
	return env.This.Id + "(env)"
}
