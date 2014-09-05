package actor

type Actor struct {
	Id        string
	Group     *ActorGroup
	env       *ActorEnv
	parent    *Actor
	watchers  map[*Actor]tEmptyStruct
	children  map[string]*Actor
	options   map[string]interface{}
	validator func(Msg) bool
}

func (a *Actor) validateMsg(msg Msg) bool {
	if a.validator == nil {
		return true
	}
	ok := a.validator(msg)
	if !ok {
		dlog(a, "Actor.validateMsg() intercepted message for")
		return false
	}
	return true
}

// Send propogates the message to the actor upon which the
// function is called.  The function is guaranteed to return
// immediately.
func (a *Actor) Send(msg Msg) {
	if a.validateMsg(msg) {
		go func() {
			defer errLogWithText(a, "Failure in Send()")
			// TODO: Check for failure with a defer
			a.env.mbox <- msg
		}()
	}
}

func (a *Actor) SendByName(name string, msg Msg) bool {
	target := a.env.findChild(name)
	if target == nil {
		return false
	}
	target.Send(msg)
	return true
}

// SendBlocking() adds the msg to the specified actor's inbox.
// However, unlike Send(), this operation will block until the
// actor has accepted the message, and can indicate a failure.
// Because the return status indicates success/failure, no
// logging output is generated.  (Be aware, this method returns
// once the actor has receieved the message.  That does not mean
// that the actor is actively processing it, only that is on
// the actor's internal queue.)
//
// NOTE: Blocking operations should generally NOT be used with
// the actor paradigm.  Using this could lead to deadlock.
func (a *Actor) SendBlocking(msg Msg) bool {
	valid := a.validateMsg(msg)
	if !valid {
		return false
	}
	retCode := true
	defer func() {
		if r := recover(); r != nil {
			retCode = false
		}
	}()
	a.env.mbox <- msg

	return retCode
}

// Die() will cause an actor to gracefully die.  For details on
// the death process, see ActorEnv.Suicide().
//
// The operation is non-blocking, and will return immediately.
func (a *Actor) Die() {
	a.env.Suicide()
}

// Monitor will cause the watcher to be notified when a dies
func (a *Actor) Monitor(watcher *Actor) {
	safeSend(a.env.cbox, cAddWatcher{watcher}, a, "actor.AddWatcher()")
}

// Unmonitor removes a previous monitor entry
func (a *Actor) Unmonitor(watcher *Actor) {
	safeSend(a.env.cbox, cDelWatcher{watcher}, a, "actor.DelWatcher()")
}

// a.obit() is called to notify a that deceased has terminated
func (a *Actor) obit(deceased *Actor, fname string) {
	// We don't care if this one fails
	sendIgnoreErr(a.env.cbox, Obit{deceased, fname})
	//safeSend(a.env.cbox, Obit{deceased, fname}, a, "actor.Obit()")
}

// This is called exclusively from the env's loop
func (a *Actor) killMyKids(resp chan bool) {
	dlog(a, "Entered")
	defer errLog(a)
	for _, k := range a.children {
		k.Die()
	}
	dlog(a, "Sending true")
	resp <- true
	dlog(a, "Exting")
}

func (a *Actor) GoString() string {
	if a.parent == nil {
		return a.Group.Id + "(guard)"
	}
	return a.Id
}

func (a *Actor) getName() string {
	if a.parent == nil {
		return a.Group.Id
	}
	return a.Id
}

func (a *Actor) fullName() string {
	name := a.getName()
	if a.parent != nil {
		name = a.parent.fullName() + ":" + name
	}
	return name
}

func errLogWithText(a tNamer, s string) {
	if r := recover(); r != nil {
		elog(a, s, r)
	}
}

func errLog(a tNamer) {
	if r := recover(); r != nil {
		elog(a, r)
	}
}
