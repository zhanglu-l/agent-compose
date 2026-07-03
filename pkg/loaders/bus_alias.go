package loaders

import (
	"github.com/samber/do/v2"

	owner "agent-compose/pkg/bus"
)

type Bus = owner.Bus

func NewBus(di do.Injector) (*Bus, error) {
	return owner.NewBus(di)
}

func NewBusWithBuffer(size int) *Bus {
	return owner.NewBusWithBuffer(size)
}
