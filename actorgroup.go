package actor

import (
	"github.com/aprimus/actor/stringgenerator"
	"github.com/aprimus/immutable/imHash"
	"reflect"
	"sync"
	"time"
)

type ActorGroup struct {
	Id             string
	swg            sync.WaitGroup // Actors != guardian
	ewg            sync.WaitGroup // system things
	memberCh       chan interface{}
	guardian       *Actor
	uniqueStringCh chan string
	stringControl  chan bool //Shuts down string generator
	db             *imHash.StringHash
	dbReq          chan interface{}
}

func NewActorGroup(name string) *ActorGroup {
	ag := &ActorGroup{
		Id: name,
	}
	ag.memberCh = make(chan interface{}, 20)
	go func() {
		defer ag.ewg.Done()
		ag.ewg.Add(1)
		ag.manageMembers(ag.memberCh)
	}()
	ag.guardian = newGuardian(ag)
	ag.uniqueStringCh, ag.stringControl =
		stringgenerator.NewGenerator(name, ag.ewg)
	ag.startAGDB()
	return ag
}

func (ag *ActorGroup) GetOrCreateActor(n string,
	f ReceiveGenerator) *Actor {

	a, ok := ag.GetNamedActor(n)
	if ok == false {
		a = ag.NewNamedActor(n, f())
	}
	return a
}

func (ag *ActorGroup) GetOrCreateActorObject(n string,
	suf SetupFunc, ch chan interface{}) *Actor {

	a, ok := ag.GetNamedActor(n)
	if ok == false {
		a = ag.NewNamedActor(n, receiverFromClass(suf, ch))
	}
	return a
}

// NewOptionedActor allows creation of an actor with
// additional configuration options.
//
// See ActorGroup.NewNamedOptionedActor for details
func (ag *ActorGroup) NewOptionedActor(aO *ActorOptions) *Actor {
	return ag.guardian.env.NewOptionedActor(aO)
}

// NewNamedOptionedActor allows creation of an actor with
// additional configuration options.
//
// See ActorGroup.NewNamedOptionedActor for details
func (ag *ActorGroup) NewNamedOptionedActor(n string,
	aO *ActorOptions) *Actor {

	return ag.guardian.env.NewNamedOptionedActor(n, aO)
}

// GetNamedActor() queries the ActorGroup to see if an appropriately
// named actor exists, returning the actor if possible.
func (ag *ActorGroup) GetNamedActor(n string) (*Actor, bool) {
	a, ok := ag.guardian.children[n]
	if ok {
		return a, true
	} else {
		return nil, false
	}
}

// SendByName is syntactic sugar for looking up an actor by
// name and then sending a message.  The value returned does
// not indicate the ultimate success of the delivery attempt,
// but only whether such a named actor exists
func (ag *ActorGroup) SendByName(name string, msg Msg) bool {
	ok := ag.guardian.SendByName(name, msg)
	return ok
}

// SendByFullName is identical to SendByName, except that it takes
// a full actor's name instead of a relative one.
func (ag *ActorGroup) SendByFullName(name string, msg Msg) bool {
	ch := make(chan *Actor)
	ag.memberCh <- cFindMember{name, ch}
	resp := <-ch
	if ch == nil {
		return false
	} else {
		resp.Send(msg)
	}
	return true
}

// SendOrCreateByName ensures that the message is sent to
// an actor. If the actor already exists, it will pass along
// the message.  However, if no such actor exists, it will
// first create the actor and thenpass it the message.
// SendOrCreateByName only works for top level actors.
func (ag *ActorGroup) SendOrCreateByName(name string,
	msg Msg, r Receive) {

	exists := ag.SendByName(name, msg)
	if exists {
		return
	}
	a := ag.NewNamedActor(name, r)
	a.Send(msg)
	return
}

// NewActor() returns an actor.  It is the same as NewNamedActor,
// except that the system provides a guaranteed unique name
func (ag *ActorGroup) NewActor(r Receive) *Actor {
	a := ag.NewNamedActor(ag.getGUID(), r)
	return a
}

// NewNamedActor() creates a new top-level actor in a group.
func (ag *ActorGroup) NewNamedActor(n string, r Receive) *Actor {
	// TODO: Put in a delay on channel to not return until
	// actor is fully spun up
	a, ok := ag.guardian.env.newChild(n, r)
	if ok == false {
		return nil
	}
	return a
}

// NewActorFarm is a pass-thru function to actor.NewActorFarm,
// allowing a farm to be built at the top level in a group
func (ag *ActorGroup) NewActorFarm(farm FarmClass) *Actor {
	return ag.guardian.env.NewNamedActorFarm(ag.getGUID(), farm)
}

// NewNamedActorFarm is a pass-thru function to
// actor.NewNamedActorFarm, allowing a farm to be built at
// the top level in a group
func (ag *ActorGroup) NewNamedActorFarm(n string,
	farm FarmClass) *Actor {

	return ag.guardian.env.NewNamedActorFarm(n, farm)
}

// SpawnFromFunc allows programmatic spawning of multiple
// new actors. Each agent is created with the receiver
// returned by calling generator(i)
func (ag *ActorGroup) SpawnFromFunc(num int,
	generator func(int) Receive) {

	ag.guardian.env.SpawnFromFunc(num, generator)
}

// SendAll sends a message to all members of a group
func (ag *ActorGroup) SendAll(msg Msg) {
	members := ag.getMembers()
	for _, aRec := range members {
		aRec.a.Send(msg)
	}
}

func newGuardian(ag *ActorGroup) *Actor {
	a := &Actor{Id: "GUARDIAN", Group: ag, validator: nil}
	a.env = newActorEnv(a, func(msg Msg, env *ActorEnv) {
		/* */
	})
	// guardian is NOT part of the sync group
	ag.swg.Done()
	a.children = make(map[string]*Actor)
	return a
}

// GracefulPassiveShutdown takes no actions of itself,
// it simply waits until all actors have terminated,
// and then returns
func (ag *ActorGroup) GracefulPassiveShutdown() {
	dlog(ag, "GracefulPassiveShutdown() -- "+
		"Waiting for all actors to terminate")
	dlog(ag, "Calling ag.swg.Wait()")
	if _DODEBUG {
		ag.AnnounceMembers(1 * time.Second)
	}
	ag.swg.Wait()
	dlog(ag, "Returned from ag.swg.Wait()")
	ag.stringControl <- true
	<-ag.uniqueStringCh // Need to trigger reading control
	ag.memberCh <- sHappyDeath{}
	ag.dbReq <- sHappyDeath{}
	dlog(ag, "Calling ag.ewg.Wait()")
	ag.ewg.Wait()
	dlog(ag, "Exiting")
}

// GracefulActiveShutdown notifies all actors to die,
// and only returns after they have expired.
func (ag *ActorGroup) GracefulActiveShutdown() {
	ag.guardian.Die()
	ag.GracefulPassiveShutdown()
}

// GetAllChildren returns a list of all the names of all
// top level actors in the group.
func (ag *ActorGroup) GetAllChildren() []string {
	return ag.guardian.env.GetChildrensNames()
}

func (ag *ActorGroup) fullName() string {
	return ag.Id + "(g)"
}

func (ag *ActorGroup) getGUID() string {
	g := <-ag.uniqueStringCh
	return g
}

/*

==== Membership Management ====

*/

func (ag *ActorGroup) removeMember(fname string) bool {
	ch := make(chan bool)
	ag.memberCh <- cRemoveMember{fname, ch}
	resp := <-ch
	return resp
}

func (ag *ActorGroup) addMember(fname string,
	actor *Actor) bool {

	ch := make(chan bool)
	ag.memberCh <- cAddMember{fname, actor, ch}
	resp := <-ch
	return resp
}

func (ag *ActorGroup) findMember(fname string) *Actor {
	ch := make(chan *Actor)
	ag.memberCh <- cFindMember{fname, ch}
	resp := <-ch
	return resp
}

func (ag *ActorGroup) getMembers() []*tActorRec {
	ch := make(chan []*tActorRec)
	ag.memberCh <- cGetAllMembers{ch}
	resp := <-ch
	return resp
}

func (ag *ActorGroup) manageMembers(ch chan interface{}) {
	members := make(map[string]*Actor)
	alive := true
	for alive {
		m := <-ch
		switch m := m.(type) {
		case cAddMember:
			_, ok := members[m.fname]
			if ok {
				dlog(ag, "Tried to add ", m.fname,
					" to group but already existed")
				m.resp <- false
			} else {
				dlog(ag, "Adding "+m.fname+" to group")
				members[m.fname] = m.a
				m.resp <- true
			}
		case cRemoveMember:
			_, ok := members[m.fname]
			if !ok {
				dlog(ag, "Tried to delete", m.fname,
					"from group but not found")
				m.resp <- false
			} else {
				dlog(ag, "Deleting "+m.fname+" from group")
				delete(members, m.fname)
				m.resp <- true
			}
		case cFindMember:
			actor, ok := members[m.fname]
			if !ok {
				m.resp <- nil
			} else {
				m.resp <- actor
			}
		case cGetAllMembers:
			list := make([]*tActorRec, 0)
			i := 0
			for n, a := range members {
				list = append(list, &tActorRec{n, a})
				i++
			}
			m.resp <- list
		case sHappyDeath:
			alive = false
		default:
			elog(ag, "manageMembers() received an unknown",
				"message of type", reflect.TypeOf(m))
		}
	}
}

// This is a utility method, which will periodically output
// a list of all members within the ActorGroup.  Useful for
// debugging, or seeing the state of a system.
func (ag *ActorGroup) AnnounceMembers(d time.Duration) chan bool {
	ctl := make(chan bool)
	go func() {
		i := 0
		for {
			select {
			case <-time.After(d):
				i++
				time.Sleep(d)
				print("Living Actors:")
				for _, s := range ag.getMembers() {
					print(s.fname, " ")
				}
				print("\n")
			case <-ctl:
				return
			}
		}
	}()
	return ctl
}

func (ag *ActorGroup) AnnounceGroupSize(d time.Duration) chan bool {
	ctl := make(chan bool)
	go func() {
		i := 0
		for {
			select {
			case <-time.After(d):
				i++
				time.Sleep(d)
				println("Total Living Actors:", len(ag.getMembers()))
			case <-ctl:
				return
			}
		}

	}()
	return ctl
}
