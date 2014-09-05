package actor

import (
	"github.com/aprimus/immutable/imHash"
	"reflect"
)

type dbInsert struct {
	dbReq
}

type dbGet struct {
	dbReq
}

type dbSet struct {
	dbReq
}

type dbMutate struct {
	dbReq
	f func(interface{}) (interface{}, bool)
}

type dbReq struct {
	key    string
	val    interface{}
	respCh chan DBResp
}

func (ag *ActorGroup) dbGet(key string) DBResp {
	key, val := ag.db.Find(key)
	return DBResp{key, val, 0}
}

func (ag *ActorGroup) dbGetSerialized(key string) DBResp {
	ch := make(chan DBResp, 1)
	query := &dbGet{dbReq{key, nil, ch}}
	ag.dbReq <- query
	resp := <-ch
	return resp
}

func (ag *ActorGroup) dbSet(key string, val interface{}) DBResp {
	ch := make(chan DBResp, 1)
	query := &dbSet{dbReq{key, val, ch}}
	ag.dbReq <- query
	resp := <-ch
	return resp
}

// Only if value currently empty
func (ag *ActorGroup) dbInsert(key string, val interface{}) DBResp {
	ch := make(chan DBResp, 1)
	query := &dbInsert{dbReq{key, val, ch}}
	ag.dbReq <- query
	resp := <-ch
	return resp
}

func (ag *ActorGroup) dbMutate(key string,
	f func(interface{}) (interface{}, bool)) DBResp {

	ch := make(chan DBResp, 1)
	query := &dbMutate{dbReq{key, nil, ch}, f}
	ag.dbReq <- query
	resp := <-ch
	return resp
}

func (ag *ActorGroup) startAGDB() {
	dlog(ag, "Entered")
	ag.db = imHash.NewStringHash()
	ag.dbReq = make(chan interface{}, 5)
	go ag.manageDB()
	dlog(ag, "Exited")
	return
}

func (ag *ActorGroup) manageDB() {
	dlog(ag, "Database Starting")
	dbCounter := 0
	more := true
	for more {
		q := <-ag.dbReq
		switch q := q.(type) {
		case *dbGet:
			dlog(ag, "*dbGet received, key = ", q.key)
			key, val := ag.db.Find(q.key)
			q.respCh <- DBResp{key, val, dbCounter}
		case *dbSet:
			dlog(ag, "*dbSet received, key = ", q.key)
			newDB := ag.db.Insert(q.key, q.val)
			dbCounter++ // tracks the number of updates
			ag.db = newDB
			q.respCh <- DBResp{q.key, q.val, dbCounter}
		case *dbInsert:
			dlog(ag, "*dbInsert received, key = ", q.key)
			if k, v := ag.db.Find(q.key); k != "" || v != nil {
				q.respCh <- DBResp{k, v, dbCounter}
			} else {
				newDB := ag.db.Insert(q.key, q.val)
				dbCounter++
				ag.db = newDB
				q.respCh <- DBResp{k, v, dbCounter}
			}
		case *dbMutate:
			dlog(ag, "*dbMutate received, key = ", q.key)
			k, v := ag.db.Find(q.key)
			newVal, doUpdate := q.f(v)
			if doUpdate {
				dlog(ag, "*dbMutate received, key = ", q.key, "Mutating")
				newDB := ag.db.Insert(k, newVal)
				dbCounter++
				ag.db = newDB
				q.respCh <- DBResp{k, newVal, dbCounter}
			} else {
				dlog(ag, "*dbMutate received, key = ", q.key, "No action")
				q.respCh <- DBResp{k, v, dbCounter}
			}
		case sHappyDeath:
			dlog(ag, "Instructed to exit")
			more = false
		default:
			elog(ag, "Unexpected msg type:", reflect.TypeOf(q))
		}
	}
	dlog(ag, "Exiting")
}
