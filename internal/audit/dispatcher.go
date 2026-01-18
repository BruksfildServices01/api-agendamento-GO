package audit

import "log"

type Event struct {
	BarbershopID uint
	UserID       *uint
	Action       string
	Entity       string
	EntityID     *uint
	Metadata     any
}

type Dispatcher struct {
	logger *Logger
	queue  chan Event
}

func NewDispatcher(logger *Logger) *Dispatcher {
	d := &Dispatcher{
		logger: logger,
		queue:  make(chan Event, 100), // buffer seguro
	}

	go d.worker()
	return d
}

func (d *Dispatcher) worker() {
	for ev := range d.queue {
		if err := d.logger.Log(
			ev.BarbershopID,
			ev.UserID,
			ev.Action,
			ev.Entity,
			ev.EntityID,
			ev.Metadata,
		); err != nil {
			log.Println("audit error:", err)
		}
	}
}

func (d *Dispatcher) Dispatch(ev Event) {
	select {
	case d.queue <- ev:
		// enviado
	default:
		// fila cheia → descartamos audit (nunca quebrar API)
		log.Println("audit queue full, dropping event")
	}
}
