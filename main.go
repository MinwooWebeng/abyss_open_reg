package main

import (
	"abyss_open_reg/aurl"
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type HostData struct {
	connection_info []byte
	event_ch        chan *JoinRequestEvent
	waiter_cnt      atomic.Bool
	last_update     time.Time
}

type JoinRequestEvent struct {
	connection_info []byte
}

var (
	memory = make(map[string]*HostData)
	mu     sync.RWMutex
)

func main() {	
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/wait", eventWaiter)

	http.HandleFunc("/api/random", randomHandler)
	http.HandleFunc("/api/request", joinRequestHandler)

	static_fs := http.FileServer(http.Dir("./static/"))
	http.Handle("/", static_fs)

	go func() {
		for {
			time.Sleep(20 * time.Second)
			cleanup()
		}
	}()

    log.Println("Starting server on https://irublue.com")
	log.Fatal(http.ListenAndServeTLS(":443", "/etc/letsencrypt/live/yourdomain.com/fullchain.pem", "/etc/letsencrypt/live/yourdomain.com/privkey.pem", nil))
}

// removes host data that had no activity for 1 min.
func cleanup() {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now()
	for k, v := range memory {
		if now.Sub(v.last_update) > 1*time.Minute {
			fmt.Println("outdated: " + k)
			v.event_ch <- nil
			delete(memory, k)
		}
	}
}

// registerHandler handles POST requests to /register
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the entire request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request: Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Split into three parts
	parts := bytes.SplitN(bodyBytes, []byte("\n\n"), 3)
	if len(parts) != 3 {
		http.Error(w, "Bad Request: Expected three newline-separated values - "+fmt.Sprint(len(parts)), http.StatusBadRequest)
		return
	}

	//parse the first string (AURL)
	abyss_url, err := aurl.TryParse(string(parts[0]))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Store in the map (with lock)
	mu.Lock()
	data, ok := memory[abyss_url.Hash]
	if ok {
		data.event_ch <- nil
		delete(memory, abyss_url.Hash)
	}

	memory[abyss_url.Hash] = &HostData{
		connection_info: bodyBytes,
		event_ch:        make(chan *JoinRequestEvent, 8),
		last_update:     time.Now(),
	}
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}

// pending eventWaiter
func eventWaiter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "URL parameter 'id' missing", http.StatusBadRequest)
		return
	}

	mu.Lock()
	host_data, ok := memory[id]
	if ok {
		host_data.last_update = time.Now()
	}
	mu.Unlock()

	if !ok {
		http.Error(w, "not registered", http.StatusNotFound)
		return
	}

	if !host_data.waiter_cnt.CompareAndSwap(false, true) {
		http.Error(w, "Already waiting", http.StatusConflict)
		return
	}

	select {
	case join_req := <-host_data.event_ch: //waits
		if join_req == nil {
			http.Error(w, "host info outdated", http.StatusGone)
		} else {
			w.Write(join_req.connection_info)
		}
	case <-time.After(5 * time.Second):
		w.Write([]byte(".")) //OK
	}

	host_data.waiter_cnt.Store(false) //returns occupation
}

// randomHandler handles GET requests to /random
func randomHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "URL parameter 'id' missing", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// If there are no entries, return an error
	if len(memory) == 0 {
		http.Error(w, "No peers available", http.StatusNotFound)
		return
	}

	// Pick a random entry
	var keys []string
	for k := range memory {
		if k == id {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		http.Error(w, "No peers available", http.StatusNotFound)
		return
	}
	randomKey := keys[rand.Intn(len(keys))]

	w.Write([]byte(randomKey))
}

func joinRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "URL parameter 'id' missing", http.StatusBadRequest)
		return
	}

	targ := r.URL.Query().Get("targ")
	if targ == "" {
		http.Error(w, "URL parameter 'targ' missing", http.StatusBadRequest)
		return
	}

	mu.Lock()
	host_data, id_ok := memory[id]
	targ_data, targ_ok := memory[targ]
	mu.Unlock()

	if !id_ok {
		http.Error(w, "host not registered", http.StatusNotFound)
		return
	}
	if !targ_ok {
		http.Error(w, "target not registered", http.StatusNotFound)
		return
	}

	//raise join request
	targ_data.event_ch <- &JoinRequestEvent{
		connection_info: host_data.connection_info,
	}

	w.Write(targ_data.connection_info)
}
