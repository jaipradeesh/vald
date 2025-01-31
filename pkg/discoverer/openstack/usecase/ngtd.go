package usecase

import (
	"context"

	"github.com/vdaas/vald/pkg/discoverer/openstack/config"
	"github.com/vdaas/vald/pkg/discoverer/openstack/service"
)

type Runner interface {
	Start(ctx context.Context) chan []error
}

type run struct {
	cfg    config.Data
	server service.Server
}

func New(cfg config.Data) (Runner, error) {
	return &run{
		cfg:    cfg,
		server: service.NewServer(nil),
	}, nil
}

func (t *run) Start(ctx context.Context) chan error {
	return t.server.ListenAndServe(ctx)
}
