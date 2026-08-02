package adapter

import (
	"context"
	"fmt"

	"clawbot-gateway/internal/database"
)

// ── Echo 调试适配器 ──

type EchoAdapter struct {
	id   string
	name string
}

func NewEchoAdapter(id, name string) *EchoAdapter {
	return &EchoAdapter{id: id, name: name}
}

func (e *EchoAdapter) ID() string                           { return e.id }
func (e *EchoAdapter) Name() string                         { return e.name }
func (e *EchoAdapter) Type() string                         { return "echo" }
func (e *EchoAdapter) HealthCheck(ctx context.Context) bool { return true }

func (e *EchoAdapter) Handle(ctx context.Context, req *BackendRequest) (*BackendResponse, error) {
	reply := fmt.Sprintf("[Echo:%s] %s", e.id, req.Message)
	return &BackendResponse{Msg: reply, Backend: e.id}, nil
}

func (e *EchoAdapter) HandleStream(ctx context.Context, req *BackendRequest, ch chan<- string) error {
	defer close(ch)
	reply := fmt.Sprintf("[Echo:%s] %s", e.id, req.Message)
	ch <- reply
	return nil
}

// 自注册
func init() {
	RegisterAdapter("echo", func(b database.Backend) BackendAdapter {
		return NewEchoAdapter(b.ID, b.Name)
	})
}
