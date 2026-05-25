package contracts

import "context"

type AgentRegistry interface {
	Register(ctx context.Context, def AgentDefinition) error
	Validate(ctx context.Context, def AgentDefinition) error
	Resolve(ctx context.Context, name string) (AgentDefinition, error)
	List(ctx context.Context) ([]AgentDefinition, error)
}
