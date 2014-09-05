package actor

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"
)

type EchoPacket struct {
	ch  chan string
	val string
}

type SelfIdPacket struct {
	ch chan int
}

type AddMsg struct {
	val int
}

func (m AddMsg) getVal() string {
	return strconv.Itoa(m.val)
}

type EchoInt struct {
	ch chan int
}

func (m EchoInt) getVal() string {
	return "Echo Int Packet"
}

type Spawn struct{}

type getValer interface {
	getVal() string
}

type Shutdown struct{}

func setupBasicAG(name string, gen func(int) Receive,
	num int) *ActorGroup {
	ag := NewActorGroup(name)
	ag.SpawnFromFunc(num, gen)
	return ag
}

func genParrotAndDieActor(val int) Receive {
	return func(msg Msg, env *ActorEnv) {
		ch := msg[0].(chan int)
		ch <- val
		env.Suicide()
	}
}

func genAccumulatorActor() Receive {
	v := 0
	return func(msg Msg, env *ActorEnv) {
		// By making a temporary copy and sleeping,
		// we can exacerbate any potential race cond's
		tv := v
		time.Sleep(10000 * time.Nanosecond)
		switch m := msg[0].(type) {
		case AddMsg:
			v = tv + m.val
		case EchoInt:
			m.ch <- v
		case Shutdown:
			env.Suicide()
		}
	}
}

func genSuicidalActor() Receive {
	div := 0
	return func(msg Msg, env *ActorEnv) {
		z := 5 / div
		fmt.Println(z)
	}
}

func genDieOnInputActor() Receive {
	return func(msg Msg, env *ActorEnv) {
		env.Suicide()
	}
}

// This is a generator-generator, which will return a
// generator for actors which notify a specified channel as
// soon as they receive their first message, and then wait
// the amount of time specified in the message before dying
func genDyingNoisyGen(notify chan bool) func() Receive {
	ch := notify
	return func() Receive {
		return func(msg Msg, env *ActorEnv) {
			ch <- true
			timeToWait := msg[0].(time.Duration)
			time.Sleep(timeToWait)
			env.Suicide()
		}
	}
}

// This checks to make sure there are no goroutines still
// running by looking at the size of the call stack
func verifyClean() bool {
	runtime.Gosched() // Give other things a chance to run
	<-time.After(50000 * time.Nanosecond)
	runtime.Gosched() // Give other things a chance to run
	buf := make([]byte, 5000)
	n := runtime.Stack(buf, true)
	if n > 1500 { // Probably have too much going on
		fmt.Println("Size of return is ", n)
		fmt.Println(string(buf))
		return false
	}
	return true
}

// Simple test to spin up a system, have the actors
// send a message, and then have a graceful shutdown.
func TestCreateAGandActors(t *testing.T) {
	testSize := 3
	ag := setupBasicAG("TOP", genEchoActor, testSize)
	if ag.Id != "TOP" {
		t.Errorf("Actor group not created")
	}
	ch := make(chan int, 5)
	sum := 0
	ag.SendAll(Msg{SelfIdPacket{ch}})
	for i := 0; i < testSize; i++ {
		sum += i
		n := <-ch
		sum -= n
	}
	if sum != 0 {
		t.Errorf("Actors did not supply correct sum")
	}
	ag.GracefulActiveShutdown()
	if !verifyClean() {
		t.Errorf("System did not fully shut down")
	}
	return
}

func genEchoActor(val int) Receive {
	v := val
	return func(msg Msg, env *ActorEnv) {
		switch m := msg[0].(type) {
		case Shutdown:
			env.Suicide()
		case EchoPacket:
			m.ch <- m.val
		case Spawn:
			env.NewActor(genEchoActor(0))
		case SelfIdPacket:
			m.ch <- v
		default:
			fmt.Println("Got a weird msg...",
				reflect.TypeOf(m))
		}
	}
}

func TestHeirarchicalActors(t *testing.T) {
	const numKids = 5
	ag := NewActorGroup("TOP")
	topActor := ag.NewNamedActor("topActor", genEchoActor(0))
	for i := 0; i < numKids; i++ {
		topActor.SendBlocking(Msg{Spawn{}})
	}
	<-time.After(100000 * time.Nanosecond)
	ag.GracefulActiveShutdown()
	<-time.After(100000 * time.Nanosecond)
	if !verifyClean() {
		t.Errorf("System did not fully shut down")

	}
	return
}

func TestSelfDestruct(t *testing.T) {
	numActs := 5
	ag := setupBasicAG("SelfDestruct", genEchoActor, numActs)
	shutdownMsg := Msg{Shutdown{}}
	ag.SendAll(shutdownMsg)

	time.Sleep(500000 * time.Nanosecond)
	if len(ag.guardian.children) > 0 {
		t.Errorf("Kids failed to self destruct (timeout?)")
	}
	members := ag.getMembers()
	// Guardian will still be alive
	if len(members) > 1 {
		t.Errorf("Group's list of members is not empty")
	}
	ag.GracefulActiveShutdown()
}

func TestFarm(t *testing.T) {
	testSize := 10
	ag := setupBasicAG("Farm", genEchoActor, 0)
	in := make(chan int, 10)
	for i := 0; i < testSize; i++ {
		ag.NewActor(genEchoActor(i))
	}
	m := Msg{SelfIdPacket{in}}
	ag.SendAll(m)
	sum := 0
	for i := 0; i < testSize; i++ {
		sum += i
		val := <-in
		sum -= val
	}
	if sum != 0 {
		t.Errorf("Values did not sum properly")
	}
	ag.SendAll(Msg{Shutdown{}})
	ag.GracefulPassiveShutdown()
}

// ag.SendByName() uses a.SendByName, so this tests both
func TestSendByName(t *testing.T) {
	ag := setupBasicAG("TestSendByName", genEchoActor, 2)
	names := ag.GetAllChildren()
	for _, n := range names {
		ag.SendByName(n, Msg{Shutdown{}})
	}
	checkNeg := ag.SendByName("IntentionalInvalidName",
		Msg{Shutdown{}})
	if checkNeg {
		t.Errorf("Improper response from ag.SendByName()" +
			" to a non-existant actor")
	}
	ag.GracefulPassiveShutdown()
}

func TestSelfDestruction(t *testing.T) {
	testSize := 5
	ag := setupBasicAG("TestSelfDescruction",
		genParrotAndDieActor, testSize)
	in := make(chan int, 1)
	ag.SendAll(Msg{in})
	for i := 0; i < testSize; i++ {
		<-in
	}
	ag.GracefulPassiveShutdown()
}

// Make sure that a given actor is not running more than
// one receive() simultaneously
func TestSerialActorBehavior(t *testing.T) {
	testSize := 1000
	ag := setupBasicAG("TestSerialActorBehavior",
		genParrotAndDieActor, 0)
	r := genAccumulatorActor()
	a := ag.NewActor(r)
	sum := 0
	for i := 0; i < testSize; i++ {
		sum += i
		// Because we're testing serial behavior, we
		// want to be sure all these messages delivered
		a.SendBlocking(Msg{AddMsg{i}})
	}
	ch := make(chan int)
	a.Send(Msg{EchoInt{ch}})
	ret := <-ch
	if ret != sum {
		t.Errorf("Returned value incorrect, received " +
			strconv.Itoa(ret) + " instead of " +
			strconv.Itoa(sum))
	}
	ag.GracefulActiveShutdown()
}

func TestReceiverFromClass(t *testing.T) {
	testSize := 5
	startAt := 10

	ch := make(chan interface{}, 2)
	go func() {
		for i := startAt; i < startAt+testSize; i++ {
			ch <- strconv.Itoa(i)
		}
	}()

	ag := NewActorGroup("ReceiverFromClass")

	for i := 0; i < testSize; i++ {
		ag.NewNamedActor(strconv.Itoa(i),
			receiverFromClass(rCSetup, ch))
	}
	current := time.Now()
	resp := make(chan time.Time, 1)
	ag.SendAll(Msg{resp})
	for i := 0; i < testSize; i++ {
		inst := <-resp
		if current.Before(inst) {
			t.Errorf("Invalid time returned")
		}
	}

	ag.GracefulActiveShutdown()
}

type optionsAgent struct {
	m map[string]interface{}
}

func EmptyReceive(msg Msg, env *ActorEnv) {
	// Empty
}

func TestOptionedActor(t *testing.T) {
	ag := NewActorGroup("AgeTest")

	ao := &ActorOptions{
		Receive: EmptyReceive,
		AgeOut:  time.Duration(1000 * time.Nanosecond),
	}

	ag.NewOptionedActor(ao)
	ag.GracefulPassiveShutdown()
}

func DBActor(msg Msg, env *ActorEnv) {
	cmd := msg[0].(string)
	if cmd == "set" {
		env.DBSet(msg[1].(string), msg[2])
	} else if cmd == "get" {
		resp := env.DBGet(msg[1].(string))
		switch v := resp.val.(type) {
		case string:
			msg[2].(chan string) <- v
		default:
			msg[2].(chan string) <- "Empty"
		}
	} else if cmd == "mutate" {
		resp := env.DBMutate(msg[1].(string),
			msg[2].(func(interface{}) (interface{}, bool)))
		msg[3].(chan string) <- resp.val.(string)
	} else if cmd == "die" {
		msg[1].(chan string) <- "dead"
		env.Suicide()
	}
}

func m(v interface{}) (interface{}, bool) {
	switch v := v.(type) {
	case string:
		if v == "doctor" {
			return "Doctor Who", true
		}
		return "", false
	default:
		println("Oops, type received by mutate is ",
			reflect.TypeOf(v))
	}
	return "", false
}

func TestAGDB(t *testing.T) {
	ag := NewActorGroup("DBTest")
	a1 := ag.NewActor(DBActor)
	rchan := make(chan string, 5)
	for _, m := range []Msg{Msg{"get", "donut", rchan},
		Msg{"set", "apple", "doctor"},
		Msg{"set", "actor", "model"},
		Msg{"die", rchan}} {
		a1.SendBlocking(m)
	}
	for i, expected := range []string{"Empty", "dead"} {
		val := <-rchan
		if val != expected {
			t.Errorf("A1-I = %v, Expected %#v "+
				"but received %#v", i, expected, val)
		}
	}
	a2 := ag.NewActor(DBActor)
	a3 := ag.NewActor(DBActor)
	for _, m := range []Msg{Msg{"get", "apple", rchan},
		Msg{"get", "actor", rchan},
		Msg{"mutate", "apple", m, rchan},
		Msg{"mutate", "actor", m, rchan},
		Msg{"die", rchan}} {
		a2.SendBlocking(m)
	}
	for i, expected := range []string{"doctor", "model",
		"Doctor Who", "model", "dead"} {
		val := <-rchan
		if val != expected {
			t.Errorf("A2-I = %v, Expected %#v "+
				"but received %#v", i, expected, val)
		}
	}
	for _, m := range []Msg{Msg{"get", "apple", rchan},
		Msg{"get", "actor", rchan},
		Msg{"die", rchan}} {
		a3.SendBlocking(m)
	}
	for i, expected := range []string{"Doctor Who",
		"model", "dead"} {
		val := <-rchan
		if val != expected {
			t.Errorf("A3-I = %v, Expected %#v but "+
				"received %#v", i, expected, val)
		}
	}
	ag.GracefulPassiveShutdown()
}
