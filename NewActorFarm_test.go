package actor_test

import (
	"actor"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

type GoogleCounter struct {
	accumulator    int
	re             *regexp.Regexp
	distChan       chan actor.Msg
	actor.FarmInfo // Emeds for two free methods
}

func SetupGoogleFarm(dc chan actor.Msg) actor.FarmClass {
	g := &GoogleCounter{}
	g.accumulator = 0
	g.re = regexp.MustCompile("(?i)unix")
	g.FarmInfo = actor.FarmInfo{MaxWorkers: 1, DistChan: dc}
	return g
}

func (g *GoogleCounter) GenWorker() interface{} {
	return func(msg actor.Msg, env *actor.ActorEnv) {
		defer func() { env.Suicide() }()
		term := msg[0].(string)
		url := strings.Join([]string{"http://www.google.com" +
			"/search?q=", term, "#q=", term}, "")
		res, _ := http.Get(url)
		srcFile, _ := ioutil.ReadAll(res.Body)
		res.Body.Close()
		l := len(g.re.FindAllStringSubmatch(string(srcFile), -1))
		env.Return(actor.Msg{l})
	}
}

func (g *GoogleCounter) GenFarmer() actor.FarmReceive {
	onLine := true
	return func(msg actor.Msg, env *actor.ActorEnv,
		dispatch actor.DispatchFn) {
		switch m := msg[0].(type) {
		case int:
			g.accumulator += m
		case string:
			dispatch(actor.Msg{m})
		case actor.EndSentinel:
			dispatch(actor.Msg{m})
		case actor.ChildDied:
			// If not online, we'll get an error and just
			// say happy things
			onLine = false
		case actor.WorkComplete:
			if g.accumulator < 30 && g.accumulator > 17 ||
				!onLine {
				fmt.Println("20-ish mentions found")
			}
			env.Suicide()
		}
	}
}

// This will only be meaningful if on the internet
func ExampleActorGroup_NewActorFarm() {
	ag := actor.NewActorGroup("Agriculture")
	distChan := make(chan actor.Msg, 5)
	terms := []string{"Dennis+Ritchie",
		"Ken Thompson", "HP-UX"}
	f := ag.NewActorFarm(SetupGoogleFarm(distChan))
	for _, term := range terms {
		f.SendBlocking(actor.Msg{term})
	}
	f.Send(actor.Msg{actor.EndSentinel{}})
	ag.GracefulPassiveShutdown()
	// OUTPUT:
	// 20-ish mentions found
}
