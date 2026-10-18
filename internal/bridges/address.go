package bridges

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Address is what answers at the address of each bridge that hears,
// /bridges/<name>, which takes no sign-in. It hands what arrives, unread,
// to a new run of the bridge's in machine, by hear, which starts it and
// gives a channel that is sent the answer once the run has ended and what
// it heard is taken. The service is answered as the run says if it says
// within AnswerWithin, and otherwise is told its delivery was received;
// the run goes on, and is taken when it ends. Nothing that arrives is ever
// logged, and a delivery larger than MaxDelivery is refused whole.
type Address struct {
	Bridges *Bridges
	Hear    func(Start) (<-chan *Answer, error)
	// After gives a channel sent to once a time has passed; time.After
	// when it is not set.
	After func(time.Duration) <-chan time.Time
}

func (a *Address) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name, ok := strings.CutPrefix(r.URL.Path, "/bridges/")
	if !ok || name == "" || strings.Contains(name, "/") {
		http.Error(w, "there is no bridge here that hears", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxDelivery))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		http.Error(w, "the delivery is larger than Bulldog takes, so it was not taken", http.StatusRequestEntityTooLarge)
		return
	case err != nil:
		http.Error(w, "the delivery could not be read whole, so it was not taken", http.StatusBadRequest)
		return
	}
	d := Delivery{Method: r.Method, Headers: map[string]string{}, Body: string(body)}
	for name, values := range r.Header {
		d.Headers[name] = strings.Join(values, ", ")
	}
	start, err := a.Bridges.Hear(name, d)
	if errors.Is(err, ErrNotHearing) {
		http.Error(w, "there is no bridge here that hears", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "the office cannot take a delivery now", http.StatusServiceUnavailable)
		return
	}
	answered, err := a.Hear(start)
	if err != nil {
		http.Error(w, "the delivery could not be handed to the bridge", http.StatusServiceUnavailable)
		return
	}
	after := a.After
	if after == nil {
		after = time.After
	}
	select {
	case answer := <-answered:
		if answer == nil {
			received(w)
			return
		}
		for name, value := range answer.Headers {
			w.Header().Set(name, value)
		}
		w.WriteHeader(answer.Status)
		w.Write(answer.Body)
	case <-after(AnswerWithin):
		received(w)
	}
}

// received tells the service its delivery was received.
func received(w http.ResponseWriter) { w.WriteHeader(http.StatusOK) }
