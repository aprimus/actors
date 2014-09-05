/*

The actor package is designed to provide a way to program
using actors in Go. In actor-centric programming, tasks are
performed by discrete actors, with no shared memory.
Communication is by message passing. A key advantage is that
programs written with the actor model, if designed properly,
are automatically concurrent.  Given Go's lightweight
threadding model, this means that the program will gain
advantages of multi-threading parallelism without use of
explicit Mutex's or Semaphores.

The package is desigend around three primary types --
actor.Actor, actor.ActorEnv, and actor.ActorGroup -- each of
which serves a different purpose

Type Actor

This is the world's view in to a specific actor.  It
provides methods for the world to interact with the actor.
Because an actor is largely self-determined, the world's
interface is limited.  An external party can send the actor
a message, or keep tabs on the lifecycle of the actor by
monitoring it.

Type ActorEnv

This is the struct from which an executing actor accesses
the world around it.  Essentially, any interaction the actor
wishes to have with the execution framework is via ActorEnv.
This includes actions such as spawning off new child actors
[New...()], sending a message to its parent [Return()],
altering its behavior [Become()/Revert()], or dying off
[Suicide()]

Type ActorGroup

Every actor exists within an ActorGroup.  As such, it is the
unifying struct which manages an entire system.  Every actor
exists either directly as a child of an ActorGroup or
another actor.  Any actor in the system can make a new top-
level actor using the ActorGroup.New...() methods.  The
ActorGroup also provides facilities for looking up actors by
name, and access to a shared name/value store. */
package actor
