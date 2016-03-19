package memory

import (
	"errors"
	"log"
	"time"

	"github.com/emersion/neutron/backend"
)

const EventListenTimeout = 5 * time.Minute

type EventsBackend struct {
	events map[string][]*event
}

type event struct {
	*backend.Event

	// Clients listen on the last event they have seen
	listeners []chan *event
}

// Merge two events.
func mergeEvents(dst, src *backend.Event) *backend.Event {
	if dst == nil {
		dst = &backend.Event{}
	}

	dst.ID = src.ID

	if src.Refresh != 0 {
		dst.Refresh = src.Refresh
	}
	if src.Reload != 0 {
		dst.Reload = src.Reload
	}

	dst.Notices = append(dst.Notices, src.Notices...)

	return dst
}

func (b *EventsBackend) InsertEvent(user string, e *backend.Event) error {
	e.ID = generateId()
	b.events[user] = append(b.events[user], &event{Event: e})
	return nil
}

// Listen for an event.
func (b *EventsBackend) listen(user string, e *event) {
	// This channel will receive new events to listen to as long as the client
	// reads newer events
	c := make(chan *event)

	next := e
	for {
		// Insert new listener to the list
		i := len(next.listeners)
		next.listeners = append(next.listeners, c)

		// Wait for a new event to listen to
		next = nil
		select {
		case next = <-c:
		case <-time.After(EventListenTimeout):
		}

		log.Println("next", next)

		// Is next event is null, stop listening
		if next == nil {
			e.listeners = append(e.listeners[:i], e.listeners[i+1:]...)
			close(c)
			return
		}
	}
}

func (b *EventsBackend) GetLastEvent(user string) (*backend.Event, error) {
	// No events for this user, create an empty one
	if len(b.events[user]) == 0 {
		err := b.InsertEvent(user, &backend.Event{})
		if err != nil {
			return nil, err
		}
	}

	// Get last event
	lastEvent := b.events[user][len(b.events[user])-1]
	// Listen to it
	go b.listen(user, lastEvent)

	return lastEvent.Event, nil
}

func (b *EventsBackend) GetEventsAfter(user, id string) (*backend.Event, error) {
	log.Println("events:", b.events[user])

	var merged *backend.Event
	var listener chan *event
	from := -1
	last := len(b.events[user]) - 1
	cleanupUntil := -1
	for i, e := range b.events[user] {
		if e.ID == id {
			// This is the event we're looking for

			if len(e.listeners) == 0 {
				// No listener on this event
				// That means that the listener has timed out
				// Create a new one
				return b.GetLastEvent(user)
			}

			// If there were no new events, there's no need to remove the listener
			if i < last {
				listener = e.listeners[0]
				e.listeners = e.listeners[1:]
			}

			from = i
			merged = &backend.Event{ID: e.ID}
		}

		// If there are no listeners on this event anymore, there's no need to keep
		// it in memory, we can destroy it
		if cleanupUntil == i-1 && len(e.listeners) == 0 {
			cleanupUntil = i
		}

		// If this is a new event, merge it
		if from != -1 && i > from {
			merged = mergeEvents(merged, e.Event)
		}
	}

	// Cleanup old events
	if cleanupUntil != -1 {
		b.events[user] = b.events[user][cleanupUntil+1:]
	}

	// Event ID not found
	if from == -1 {
		return nil, errors.New("No such event")
	}

	// If we removed the listener from an event, we have to add it to the last one
	if from < last {
		listener <- b.events[user][len(b.events[user])-1]
	}

	return merged, nil
}

func NewEventsBackend() backend.EventsBackend {
	return &EventsBackend{
		events: map[string][]*event{},
	}
}
