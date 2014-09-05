package actor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

func init() {
	debugLog = log.New(os.Stdout, "", 0)
	errorLog = log.New(os.Stderr, "", 0)
}

/* ---- UTILITY FUNCTIONS ---- */

func safeSend(dest chan interface{}, msg interface{},
	from tNamer, err string) {
	defer errLogWithText(from, err)
	dest <- msg
}

func sendIgnoreErr(dest chan interface{}, msg interface{}) {
	defer func() {
		if r := recover(); r != nil {
		}
	}()
	dest <- msg
}

// TODO: Remove Debug() from final version
func Debug(a bool) {
	_DODEBUG = a
}

func dlog(a tNamer, v ...interface{}) {
	if _DODEBUG {
		debugLog.Println("["+a.fullName()+"/"+caller(1)+"]",
			fmt.Sprint(v...))
	}
}

func elog(a tNamer, v ...interface{}) {
	errorLog.Println("["+a.fullName()+"/"+caller(1)+"]", v)
}

// In some cases, an actor may be more complicated than is
// easily expressed in a single Receive() function -- the use
// of implicit closures to store state versus the ability to
// have a class with methods can make some situations awkward.
func receiverFromClass(suf SetupFunc,
	ch chan interface{}) Receive {

	if ch != nil {
		param := <-ch
		return suf(param).Receive
	}
	return suf(nil).Receive
}

func genFarmReceiveAdaptor(distCh chan Msg,
	r FarmReceive) Receive {
	disp := func(msg Msg) {
		distCh <- msg
	}
	return func(msg Msg, env *ActorEnv) {
		r(msg, env, disp)
	}
}

func caller(steps int) string {
	name := "?"
	if pc, _, _, ok := runtime.Caller(steps + 1); ok {
		name = filepath.Base(runtime.FuncForPC(pc).Name())
	}
	return name
}
