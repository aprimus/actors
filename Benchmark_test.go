package actor_test

import (
	"actor"
	"bufio"
	"os"
	"runtime"
	"testing"
	"time"
)

func genParrotAndDieActor() actor.Receive {
	return func(msg actor.Msg, env *actor.ActorEnv) {
		ch := msg[0].(chan int)
		n := msg[1].(int)
		ch <- n
		env.Suicide()
	}
}

func BenchmarkCreation2CPU(b *testing.B) {
	bmTrivialActorCreation(2, b)
}

func bmTrivialActorCreation(cpus int, b *testing.B) {
	ag := actor.NewActorGroup("Bench")
	runtime.GOMAXPROCS(cpus)
	runtime.GC()
	b.ResetTimer()
	ch := make(chan int, 100)
	go func() {
		for i := 0; i < b.N; i++ {
			go func() {
				a := ag.NewActor(genParrotAndDieActor())
				a.Send(actor.Msg{ch, i})
			}()
		}
	}()
	sum := 0
	for i := 0; i < b.N; i++ {
		n := <-ch
		sum += n
	}
	ag.GracefulPassiveShutdown()
}

type cmd struct {
	ch  chan int
	val int
}

func responder(ch chan cmd) {
	m := <-ch
	m.ch <- m.val
}

func BenchmarkTrivialNativeGo(b *testing.B) {
	runtime.GOMAXPROCS(2)
	runtime.GC()
	b.ResetTimer()

	sumCh := make(chan int, 100)
	go func() {
		for i := 0; i < b.N; i++ {
			go func() {
				num := i
				och := make(chan cmd, 1)
				go responder(och)
				och <- cmd{sumCh, num}
			}()
		}
	}()
	sum := 0
	for i := 0; i < b.N; i++ {
		n := <-sumCh
		sum += n
	}
}

func genReadFromZeroAndDieActor() actor.Receive {
	return func(msg actor.Msg, env *actor.ActorEnv) {
		ch := msg[0].(chan int)
		n := msg[1].(int)
		fi, err := os.Open("/dev/zero")
		if err != nil {
			panic(err)
		}
		defer func() {
			if err := fi.Close(); err != nil {
				panic(err)
			}
		}()
		r := bufio.NewReader(fi)
		buf := make([]byte, n)
		_, err = r.Read(buf)
		if err != nil {
			println("FAIL")
		}
		ch <- n
		env.Suicide()
	}
}

// This will fail without a large number of open files
// allowed.
func BenchmarkDevZeroActor2CPU(b *testing.B) {
	bmReadFromZeroActor(2, b)
}

func bmReadFromZeroActor(cpus int, b *testing.B) {
	ag := actor.NewActorGroup("Bench")
	runtime.GOMAXPROCS(cpus)
	runtime.GC()
	time.Sleep(500000 * time.Nanosecond)

	b.ResetTimer()
	ch := make(chan int, 100)
	go func() {
		for i := 0; i < b.N; i++ {
			go func() {
				a := ag.NewActor(genReadFromZeroAndDieActor())
				a.Send(actor.Msg{ch, i})
			}()
		}
	}()
	sum := 0
	for i := 0; i < b.N; i++ {
		n := <-ch
		sum += n
		if i%10000 == 0 {
			println("i = ", i)
		}

	}
	ag.GracefulPassiveShutdown()
}

func DevZeroResponder(ch chan cmd) {
	m := <-ch
	fi, err := os.Open("/dev/zero")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := fi.Close(); err != nil {
			panic(err)
		}
	}()
	r := bufio.NewReader(fi)
	buf := make([]byte, m.val)
	_, err = r.Read(buf)
	if err != nil {
		println("FAIL")
	}
	m.ch <- m.val
}

// This will fail without a large number of open files
// allowed.
func BenchmarkDevZeroNativeGo(b *testing.B) {
	runtime.GOMAXPROCS(2)
	runtime.GC()
	time.Sleep(500000 * time.Nanosecond)
	b.ResetTimer()

	sumCh := make(chan int, 100)
	go func() {
		for i := 0; i < b.N; i++ {
			go func() {
				num := i
				och := make(chan cmd, 1)
				go DevZeroResponder(och)
				och <- cmd{sumCh, num}
			}()
		}
	}()
	sum := 0
	for i := 0; i < b.N; i++ {
		n := <-sumCh
		sum += n
		if i%10000 == 0 {
			println("i = ", i)
		}
	}
}
