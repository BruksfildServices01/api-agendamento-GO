package audit

import (
	"log"
	"sync"
)

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
	wg     sync.WaitGroup
}

func NewDispatcher(logger *Logger) *Dispatcher {
	d := &Dispatcher{
		logger: logger,
		queue:  make(chan Event, 512), // buffer maior — cobre picos de fechamento em massa
	}

	d.wg.Add(1)
	go d.worker()
	return d
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()
	for ev := range d.queue {
		if err := d.logger.Log(
			ev.BarbershopID,
			ev.UserID,
			ev.Action,
			ev.Entity,
			ev.EntityID,
			ev.Metadata,
		); err != nil {
			log.Println("[audit] log error:", err)
		}
	}
}

// Dispatch envia um evento para a fila. Nunca bloqueia nem quebra a request:
// se a fila estiver cheia, o evento é descartado com um log de aviso.
func (d *Dispatcher) Dispatch(ev Event) {
	select {
	case d.queue <- ev:
	default:
		log.Println("[audit] queue full — event dropped:", ev.Action)
	}
}

// Shutdown fecha a fila e aguarda o worker persistir todos os eventos pendentes.
// Deve ser chamado no graceful shutdown, antes de fechar a conexão com o DB.
func (d *Dispatcher) Shutdown() {
	close(d.queue)
	d.wg.Wait()
}
