// Subscribe to Bootic events over ZQM
//     socket, _  := booticzmq.NewZMQSubscriber(zmqAddress, topic)
//     daemon.SubscribeToType(NotifierChan, "pageview")
package booticzmq

import (
	"log"
	"regexp"
	"sync"

	zmq "github.com/alecthomas/gozmq"
	data "github.com/bootic/bootic_go_data"
)

type Daemon struct {
	socket        *zmq.Socket
	observers     map[string][]data.EventsChannel
	funcObservers []func(*data.Event)
	subscribeLock *sync.Mutex
}

func (d *Daemon) listen() {
	reg, _ := regexp.Compile(`^(?:[^ ]+)?\s+(.+)`)
	for {
		msg, _ := d.socket.Recv(0)

		r := reg.FindStringSubmatch(string(msg))

		if len(r) > 1 {
			payload := r[1]
			event, err := data.Decode([]byte(payload))
			if err != nil {
				log.Println("Invalid data", err)
			} else {
				d.Dispatch(event)
			}
		} else {
			log.Println("Irregular expression", string(msg))
		}
	}
}

func (self *Daemon) SubscribeToType(observer data.EventsChannel, typeStr string) {
	self.subscribeLock.Lock()
	self.observers[typeStr] = append(self.observers[typeStr], observer)
	self.subscribeLock.Unlock()
}

func (self *Daemon) Dispatch(event *data.Event) {
	self.subscribeLock.Lock()
	defer self.subscribeLock.Unlock()
	// dispatch to function observers (non-blocking)
	for _, fn := range self.funcObservers {
		go fn(event)
	}

	// Dispatch to global observers
	for _, observer := range self.observers["all"] {
		observer <- event
	}

	// Dispatch to type observers
	evtStr, _ := event.Get("type").String()
	for _, observer := range self.observers[evtStr] {
		observer <- event
	}
}

func (self *Daemon) SubscribeFunc(fn func(*data.Event)) {
	self.subscribeLock.Lock()
	self.funcObservers = append(self.funcObservers, fn)
	self.subscribeLock.Unlock()
}

func NewZMQSubscriber(host string, topics ...string) (daemon *Daemon, err error) {
	context, _ := zmq.NewContext()
	socket, err := context.NewSocket(zmq.SUB)

	for _, topic := range topics {
		socket.SetSockOptString(zmq.SUBSCRIBE, topic)
	}

	socket.Connect(host)

	var arr []func(*data.Event)

	daemon = &Daemon{
		socket:        socket,
		observers:     make(map[string][]data.EventsChannel),
		funcObservers: arr,
		subscribeLock: &sync.Mutex{},
	}

	go daemon.listen()

	return
}
