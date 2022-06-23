package controllers

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func EnsureCommonFinalizer(
	ctx context.Context, obj client.Object,
	c client.Client, cacheFinalizer string,
) error {
	if !controllerutil.ContainsFinalizer(
		obj, cacheFinalizer) {
		controllerutil.AddFinalizer(obj, cacheFinalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("adding finalizer: %w", err)
		}
	}
	return nil
}

type dynamicWatchFreer interface {
	Free(obj client.Object) error
}

func HandleCommonDeletion(
	ctx context.Context, obj client.Object,
	c client.Client, dw dynamicWatchFreer,
	cacheFinalizer string,
) error {
	if controllerutil.ContainsFinalizer(obj, cacheFinalizer) {
		controllerutil.RemoveFinalizer(obj, cacheFinalizer)

		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("removing finalizer: %w", err)
		}
	}

	if err := dw.Free(obj); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}
	return nil
}
