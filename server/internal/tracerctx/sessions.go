// SPDX-License-Identifier: GPL-2.0-or-later
/*
 * Copyright (C) 2022 VMware, Inc. Enyinna Ochulor <eochulor@vmware.com>
 * Copyright (C) 2022 VMware, Inc. Tzvetomir Stoyanov (VMware) <tz.stoyanov@gmail.com>
 *
 * In-memory database with all currently configured tracing sessions.
 */
package tracerctx

import (
	"fmt"
	"math/rand"
	"strconv"

	"gitlab.eng.vmware.com/opensource/tracecruncher-api/internal/pods"
	"gitlab.eng.vmware.com/opensource/tracecruncher-api/internal/tracehook"
)

var (
	idGenRetries = 100
)

type sessionNew struct {
	Pod              string `json:"pod"`
	Container        string `json:"container"`
	TraceHook        string `json:"trace-hook"`
	TraceArguments   string `json:"trace-arguments"`
	TraceUserContext string `json:"trace-user-context"`
}

type sessionChange struct {
	Run bool `json:"run"`
}

type traceSessionInfo struct {
	Id          uint64
	Context     *string
	Containers  map[string][]*string
	TraceHook   *string
	TraceParams *string
	Running     bool
}

type traceSession struct {
	containers  []*pods.Container
	tHook       *tracehook.TraceHook
	tHookParam  *string
	userContext *string
	running     bool
	sessionPid  int
}

type sessionDb struct {
	all map[uint64]*traceSession
}

func newSessionDb() *sessionDb {
	return &sessionDb{
		all: make(map[uint64]*traceSession),
	}
}

func (s *sessionDb) newId() (uint64, error) {
	for i := 0; i <= idGenRetries; i++ {
		id := rand.Uint64()
		if _, ok := s.all[id]; !ok {
			return id, nil
		}
	}

	return 0, fmt.Errorf("Failed to generate session ID")
}

func (t *Tracer) getSessionInfo(id uint64) (*traceSessionInfo, error) {

	var s *traceSession
	var ok bool

	if s, ok = t.sessions.all[id]; !ok {
		return nil, fmt.Errorf("No session with ID %d", id)
	}
	res := traceSessionInfo{
		Running:     s.running,
		TraceHook:   &s.tHook.Name,
		TraceParams: s.tHookParam,
		Context:     s.userContext,
		Containers:  make(map[string][]*string),
		Id:          id,
	}
	for _, c := range s.containers {
		if _, ok := res.Containers[*c.Pod]; !ok {
			res.Containers[*c.Pod] = []*string{}
		}
		res.Containers[*c.Pod] = append(res.Containers[*c.Pod], c.Id)
	}

	return &res, nil
}

func (t *Tracer) newSession(s *sessionNew) (uint64, error) {
	var e error
	var id uint64
	ts := traceSession{
		running:     false,
		tHookParam:  &s.TraceArguments,
		userContext: &s.TraceUserContext,
	}

	if id, e = t.sessions.newId(); e != nil {
		return 0, e
	}
	ts.containers = t.pods.GetContainers(&s.Pod, &s.Container)
	if len(ts.containers) < 1 {
		return 0, fmt.Errorf("Cannot find any container")
	}

	if ts.tHook, e = t.hooks.GetHook(&s.TraceHook); e != nil {
		return 0, e
	}

	t.sessions.all[id] = &ts
	return id, nil
}

func (t *Tracer) startSession(id uint64) error {
	var s *traceSession
	var ok bool

	if s, ok = t.sessions.all[id]; !ok {
		return fmt.Errorf("No session with ID %d", id)
	}
	if s.running {
		return nil
	}

	s.running = true

	return nil
}

func (t *Tracer) stopSession(id uint64) error {
	var s *traceSession
	var ok bool

	if s, ok = t.sessions.all[id]; !ok {
		return fmt.Errorf("No session with ID %d", id)
	}
	if !s.running {
		return nil
	}

	s.running = false

	return nil
}

func (t *Tracer) changeSession(id *string, p *sessionChange) error {
	var err error
	var n uint64

	if n, err = strconv.ParseUint(*id, 10, 64); err == nil {
		if p.Run {
			err = t.startSession(n)
		} else {
			err = t.stopSession(n)
		}
	}

	return err
}

func (t *Tracer) destroySession(id *string) error {
	var n uint64
	var err error

	if n, err = strconv.ParseUint(*id, 10, 64); err != nil {
		return err
	}

	if err = t.stopSession(n); err != nil {
		return err
	}
	delete(t.sessions.all, n)
	return nil
}

func (t *Tracer) getSession(id *string, running bool) (*[]*traceSessionInfo, error) {
	res := []*traceSessionInfo{}

	if *id == "all" {
		for i, s := range t.sessions.all {
			if running && !s.running {
				continue
			}
			if info, err := t.getSessionInfo(i); err != nil {
				continue
			} else {
				res = append(res, info)
			}
		}
	} else {
		if n, err := strconv.ParseUint(*id, 10, 64); err != nil {
			return nil, err
		} else {
			if info, e := t.getSessionInfo(n); e != nil {
				return nil, e
			} else if !running || running == info.Running {
				res = append(res, info)
			}
		}
	}
	return &res, nil
}

func (t *Tracer) destroyAllSessions() {
	for i := range t.sessions.all {
		t.stopSession(i)
		delete(t.sessions.all, i)
	}
}
