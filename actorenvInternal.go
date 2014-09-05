package actor

func newActorEnv(this *Actor, r interface{}) *ActorEnv {
	env := &ActorEnv{
		This:     this,
		behavior: r,
		mbox:     make(chan Msg, _MBOX_SIZE),
		cbox:     make(chan interface{}, 5),
		sbox:     make(chan interface{}, 5),
	}
	env.activate()
	return env
}

func (env *ActorEnv) activate() {
	env.This.Group.swg.Add(1)
	go func() {
		defer env.die()
		env.mainLoop()
	}()
	env.sbox <- sReceiveFinished{}
	dlog(env, "is active")
	return
}

func (env *ActorEnv) die() {
	// Non-Guardian actions only
	if env.This.parent != nil {
		ch := make(chan bool, 1)
		safeSend(env.This.parent.env.cbox,
			cRemoveChild{env.This.Id, ch},
			env, "ActorEnv.die()")
		<-ch
		env.This.Group.removeMember(env.This.fullName())
		env.This.Group.swg.Done()
	}
	if env.deathTimer != nil {
		env.deathTimer.Stop()
	}
	// Notify anyone watching that we're gone
	for a, _ := range env.This.watchers {
		a.obit(env.This, env.This.fullName())
	}
	close(env.mbox)
	close(env.cbox)
	close(env.sbox)
	dlog(env, "has terminated")
}

func (env *ActorEnv) newChildEnv(name string,
	actor *Actor) bool {

	resC := make(chan bool)
	safeSend(env.cbox, cAddChild{name, actor, resC},
		env.This, "actorEnv.newChild()")
	ok := <-resC
	if !ok {
		return false
	}
	ok = env.This.Group.addMember(actor.fullName(), actor)
	if !ok {
		// TODO: Should unwind here
		elog(env, "Failed to add", actor.fullName(), "to group")
	}
	return ok
}

func (env *ActorEnv) newChild(n string,
	receive interface{}) (*Actor, bool) {

	child := &Actor{
		Id:        n,
		Group:     env.This.Group,
		parent:    env.This,
		validator: nil,
	}
	child.children = make(map[string]*Actor)
	child.watchers = make(map[*Actor]tEmptyStruct)

	ok := env.newChildEnv(n, child)
	if !ok {
		// Could not add child, probably a dupe
		return nil, false
	}
	child.env = newActorEnv(child, receive)
	return child, true
}

func (env *ActorEnv) findChild(name string) *Actor {
	resC := make(chan *Actor)
	safeSend(env.cbox, cFindMember{name, resC},
		env.This, "actorEnv.findChild()")
	child := <-resC
	return child
}

func (env *ActorEnv) fullName() string {
	return env.This.fullName() + "(env)"
}
