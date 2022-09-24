package pub

import (
	"fmt"
	"reflect"
	"time"
)

// Publisher manages events, subscribers and emit events.
type Publisher struct {
	subscribers    map[string] *subQueue
	l  Logger
	// DisableLogs is false by default. Set DisableLogs to true to turn off logging.
	DisableLogs    bool
}

// NewPublisher creates a publisher for creating events, subscribing functions and emitting events.
func NewPublisher() *Publisher {
	return &Publisher{
		subscribers: make(map[string]*subQueue),
		l:      nil,
		DisableLogs: true,
	}
}

// SetLogger configures logger. Implement Logger interface.
func(d *Publisher) SetLogger(logger Logger){
	d.l = logger
}

// CreateEvent create events.
// Calling this method will remove all existing subscribers for event if it already exist.
func(d *Publisher) CreateEvent(events ...string){
	for _, ev := range events{
		if d.EventExist(ev){
			d.subscribers[ev].popAll()
		}
		d.subscribers[ev] = newQueue(nil)
	}
}

// Subscribe subscribes subscribers to event. Creates the event if event does not exist.
func(d *Publisher) Subscribe(event string, subscribers ...subFunc){
	for _, s := range subscribers {
		if !d.subscriberToEventAlreadyExists(s, event){
			d.subscribers[event].pushFunc(s)
			subscriberName := getFunctionName(s)

			d.logInf(fmt.Sprintf("Subscriber: %s subscribed to Event: %s", subscriberName, event))
		}
	}
}

// Unsubscribe unsubscribes subscriber from the event.
// Delete event if no subscriber is registered to the event.
func (d *Publisher) Unsubscribe(subscriber subFunc, event string) (bool, error) {
	if _, ok := d.subscribers[event]; !ok {
		return false, ErrEventDoesNotExist
	}

	eventSubs := d.subscribers[event]

	if !d.subscriberToEventAlreadyExists(subscriber, event) {
		return false, ErrSubscriberDoesNotExist
	}

	subscriberName := getFunctionName(subscriber)

	for idx, f := range eventSubs.subscribers {
		name := getFunctionName(f)

		if subscriberName == name {
			eventSubs.popFuncAt(idx)
		}
	}

	if d.SubscribersCount(event) == 0 {
		delete(d.subscribers, event)
	}

	d.logInf(fmt.Sprintf("Subscriber: %s unsubscribed from Event: %s", subscriberName, event))
	return true, nil
}

// Publish execute each subscriber registered to this event on separate goroutines.
// Returns ErrEventDoesNotExist if event does not exist.
func(d *Publisher) Publish(event string, data EventPayload) (bool, error){
	if _, ok := d.subscribers[event]; !ok {
		return false, ErrEventDoesNotExist
	}

	subscribers := d.subscribers[event].getAllSubs()

	if len(subscribers) == 0 {
		return false, ErrNoSubscribers
	}

	data.Header = Header{
		name:      event,
		eventTime: time.Now(),
	}

	for _, subscriber := range subscribers {
		 execSubscriber := d.subWrapper(subscriber, event).(subFunc)
		 go execSubscriber(data)
	}
	return true, nil
}

func(d *Publisher) subWrapper(f interface{}, event string) interface{} {
	fv := reflect.ValueOf(f)
	subscriberName := getFunctionName(f)
	wrapperFunc := reflect.MakeFunc(reflect.TypeOf(f), func(args []reflect.Value) (results []reflect.Value) {
		defer func(){
			if x := recover(); x != nil {
				d.logErr(fmt.Sprintf("Subscriber: %s processing failed for Event: %s", subscriberName, event))
			}
		}()
		d.logInf(fmt.Sprintf("Executing subscriber: %s. Event: %s", subscriberName, event))
		out := fv.Call(args)
		d.logInf(fmt.Sprintf("Done executing subscriber %s. Event: %s", subscriberName, event))
		return out
	})

	return wrapperFunc.Interface()
}

func(d *Publisher) logErr(msg string){
	if !d.DisableLogs && d.l != nil{
		d.l.LogErr(msg)
	}
}

func(d *Publisher) logInf(msg string){
	if !d.DisableLogs && d.l != nil{
		d.l.LogInfo(msg)
	}
}
