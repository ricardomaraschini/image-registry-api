package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/ricardomaraschini/image-registry-api/registry"
	"k8s.io/klog"
)

type ev struct{}

func (e ev) NewTag(ctx context.Context, ns, image, tag string) error {
	klog.Infof("new tag %s/%s:%s", ns, image, tag)
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-ctx.Done()
		stop()
	}()

	authzer := &Authorizer{}
	reghandler := registry.New(
		authzer,
		registry.WithEventHandler(ev{}),
	)

	if err := reghandler.Start(ctx); err != nil {
		panic(err)
	}
}
