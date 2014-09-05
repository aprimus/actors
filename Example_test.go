package actor_test

import "fmt"
import "actor"

func ExampleActorGroup_SpawnFromFunc() {

	ag := actor.NewActorGroup("Example")

	ch := make(chan int, 5)

	ag.SpawnFromFunc(5,
		func(i int) actor.Receive {
			return func(msg actor.Msg, env *actor.ActorEnv) {
				retChan := msg[0].(chan int)
				retChan <- i
				env.Suicide()
			}
		})

	ag.SendAll(actor.Msg{ch})
	sum := 0
	for i := 0; i < 5; i++ {
		val := <-ch
		sum += val
	}
	fmt.Println(sum)
	// Output:
	// 10
}
