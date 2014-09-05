package actor

import (
	"fmt"
	"log"
	"reflect"
	"time"
)

var _DODEBUG = false

const _MBOX_SIZE = 35

var debugLog, errorLog, panicLog *log.Logger

/*

   PUBLIC TYPES

*/

// Go does not support untyped channes, so to provide
// maximum flexibility, a message is nothing more than
// an arbitrary length slice of anythings
type Msg []interface{}

// Helper function, simply prints the type of the first
// element in a message.
func (m Msg) GoString() string {
	return fmt.Sprintf("Msg-- Type %s", reflect.TypeOf(m[0]))
}

// WorkComplete is a message sent by the system to
// a farmer actor to indicate that all workers have
// terminated, and no more workers will be created (usually
// because the channel indicating new works has been closed.)
type WorkComplete struct{}

type EndSentinel struct{}

// ChildDied is sent to the parent of any actor which
// experiences a panic.  The message includes the
// panic thrown, the actor which suffered the panic,
// and the Msg on which the actor was working.
type ChildDied struct {
	Err     interface{}
	A       *Actor
	Message Msg
}

// "Receive" is the external interface to an actor,
// named after the Erlang BIF.
type Receive func(msg Msg, env *ActorEnv)

type FarmReceive func(msg Msg, env *ActorEnv, disp DispatchFn)

type ReceiveGenerator func() Receive

type DispatchFn func(msg Msg)

type FarmClass interface {
	GenFarmer() FarmReceive
	GenWorker() interface{} // Lots of different ways
	GetDistChan() chan Msg
	GetMaxWorkers() int
}

// FarmInfo is a type which can be embedded into a FarmClass.
// Setting the two values provides the GetDistChan() and
// GetMaxWorkers() functions for free
type FarmInfo struct {
	MaxWorkers int
	DistChan   chan Msg
}

func (f *FarmInfo) GoString() {
	fmt.Sprintf("FarmInfo{Max: %v, Dist: %v}", f.MaxWorkers,
		f.DistChan)
}

type SetupFunc func(interface{}) ActorClass

// ActorOptions allows creation of an actor with more
// control than just specifying the Receive() method.  More
// information can be found in the examples for
// ActorGroup.NewOptionedActor()

type ActorOptions struct {
	Receive      func(msg Msg, env *ActorEnv)
	FirstMessage Msg            //Sent immediately upon birth
	LastMessage  Msg            //Sent immediately prior to death
	Validator    func(Msg) bool //Applied to incoming messages
	AgeOut       time.Duration  //System generates Suicide()
	Farm         FarmClass      //Only use this or Receive
}

type ActorClass interface {
	Receive(msg Msg, env *ActorEnv)
}

// GetDistChan() is a helper method when creating factories.
// See type FarmInfo.
func (f *FarmInfo) GetDistChan() chan Msg {
	return f.DistChan
}

// GetMaxWorkers() is a helper method when creating factories.
// See type FarmInfo.
func (f *FarmInfo) GetMaxWorkers() int {
	return f.MaxWorkers
}

// Obit is the type sent as the result of a monitored's
// actor dying.
type Obit struct {
	A     *Actor
	Fname string
}

func (o Obit) GoString() string {
	return fmt.Sprintf("Obit{%s}", o.Fname)
}

// DBesp is returned by all ActorGroup.DB* functions.
type DBResp struct {
	key  string
	val  interface{}
	tick int
}

func (d DBResp) GoString() string {
	return fmt.Sprintf("DBResp{key: %s, val: %#v (%s)}",
		d.key, d.val, reflect.TypeOf(d.val))
}

/*

  PRIVATE TYPES

*/

// Data types beginning with a "c" are control messages

type cRemoveChild struct {
	id string
	ch chan bool
}

type cAddChild struct {
	id   string
	a    *Actor
	resp chan bool
}

type cAddWatcher struct {
	a *Actor
}

type cDelWatcher struct {
	a *Actor
}

type cSetObitHook struct {
	ch chan Obit
}

type cSetDieHook struct {
	ch chan bool
}

type cFindMember struct {
	fname string
	resp  chan *Actor
}

type cSetMSGObit struct {
	val bool
}

type cAddMember struct {
	fname string
	a     *Actor
	resp  chan bool
}

type cRemoveMember struct {
	fname string
	resp  chan bool
}

type cGetAllMembers struct {
	resp chan []*tActorRec
}

// Data types beginning with s indicate a state change
// These messages are unacknowledged

type sAssassin struct{}

type sHappyDeath struct{}

type sReceiveFinished struct{}

type sTombstone struct{}

// Data types beginning with t are regular types

type tEmptyStruct struct{}

type tActorRec struct {
	fname string
	a     *Actor
}

type tFarmData struct {
	distCh        chan *Msg
	idleWorkers   int
	actorReceiver Receive
}

type tNamer interface {
	fullName() string
}
