package actor

import (
	"time"
)

type receiverClass struct {
	initiation time.Time
}

func rCSetup(arg interface{}) ActorClass {
	rC := &receiverClass{time.Now()}
	return rC
}

func (rC *receiverClass) Receive(msg Msg, env *ActorEnv) {
	msg[0].(chan time.Time) <- rC.initiation
}
