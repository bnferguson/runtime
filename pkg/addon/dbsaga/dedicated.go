package dbsaga

import (
	"context"
	"fmt"

	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
	"miren.dev/runtime/pkg/saga"
)

// --- Shared Dedicated Saga Actions ---
//
// These actions are database-agnostic and shared by all on-cluster database
// addon providers. They depend on *AddonConfig (injected via saga.Using)
// for the addon name and port.

// WaitForDedicatedPool waits for the dedicated sandbox pool to have at
// least one ready instance.

type WaitForDedicatedPoolIn struct {
	PoolID entity.Id
}

type WaitForDedicatedPoolOut struct {
	Ready bool
}

func WaitForDedicatedPool(ctx context.Context, in WaitForDedicatedPoolIn) (WaitForDedicatedPoolOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	cfg := saga.Get[*AddonConfig](ctx)

	if err := fw.WaitForPool(ctx, in.PoolID, cfg.ReadyTimeout); err != nil {
		return WaitForDedicatedPoolOut{}, fmt.Errorf("waiting for pool: %w", err)
	}

	return WaitForDedicatedPoolOut{Ready: true}, nil
}

func UndoWaitForDedicatedPool(ctx context.Context, in WaitForDedicatedPoolIn, out WaitForDedicatedPoolOut) error {
	return nil
}

// CreateDedicatedService creates a network Service for the dedicated pool.

type CreateDedicatedServiceIn struct {
	ServiceName string
	AppName     string
	ServerName  string
}

type CreateDedicatedServiceOut struct {
	ServiceID entity.Id
}

func CreateDedicatedService(ctx context.Context, in CreateDedicatedServiceIn) (CreateDedicatedServiceOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	cfg := saga.Get[*AddonConfig](ctx)

	labels := types.LabelSet(
		"addon", cfg.AddonName,
		"app", in.AppName,
		"server", in.ServerName,
	)

	svcID, err := fw.CreateService(ctx, in.ServiceName, labels, cfg.Port)
	if err != nil {
		return CreateDedicatedServiceOut{}, fmt.Errorf("creating service: %w", err)
	}

	return CreateDedicatedServiceOut{ServiceID: svcID}, nil
}

func UndoCreateDedicatedService(ctx context.Context, in CreateDedicatedServiceIn, out CreateDedicatedServiceOut) error {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	return fw.DeleteService(ctx, out.ServiceID)
}

// WaitForDedicatedService waits for the service to receive an IP address.

type WaitForDedicatedServiceIn struct {
	ServiceID entity.Id
}

type WaitForDedicatedServiceOut struct {
	ServiceHost string
}

func WaitForDedicatedService(ctx context.Context, in WaitForDedicatedServiceIn) (WaitForDedicatedServiceOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	cfg := saga.Get[*AddonConfig](ctx)

	serviceHost, err := fw.WaitForServiceAddress(ctx, in.ServiceID, cfg.ReadyTimeout)
	if err != nil {
		return WaitForDedicatedServiceOut{}, fmt.Errorf("waiting for dedicated service address: %w", err)
	}

	return WaitForDedicatedServiceOut{ServiceHost: serviceHost}, nil
}

func UndoWaitForDedicatedService(ctx context.Context, in WaitForDedicatedServiceIn, out WaitForDedicatedServiceOut) error {
	return nil
}

// DeleteDedicatedService deletes the dedicated network Service.

type DeleteDedicatedServiceIn struct {
	DedicatedServiceID entity.Id
}

type DeleteDedicatedServiceOut struct {
	ServiceDeleted bool
}

func DeleteDedicatedService(ctx context.Context, in DeleteDedicatedServiceIn) (DeleteDedicatedServiceOut, error) {
	if in.DedicatedServiceID == "" {
		return DeleteDedicatedServiceOut{ServiceDeleted: false}, nil
	}

	fw := saga.Get[*addon.ProviderFramework](ctx)
	if err := fw.DeleteService(ctx, in.DedicatedServiceID); err != nil {
		return DeleteDedicatedServiceOut{}, fmt.Errorf("deleting service: %w", err)
	}

	return DeleteDedicatedServiceOut{ServiceDeleted: true}, nil
}

func UndoDeleteDedicatedService(ctx context.Context, in DeleteDedicatedServiceIn, out DeleteDedicatedServiceOut) error {
	return nil
}

// DeleteDedicatedPool deletes the dedicated sandbox pool.

type DeleteDedicatedPoolIn struct {
	DedicatedPoolID entity.Id
}

type DeleteDedicatedPoolOut struct {
	PoolDeleted bool `saga:"dedicated_pool_deleted"`
}

func DeleteDedicatedPool(ctx context.Context, in DeleteDedicatedPoolIn) (DeleteDedicatedPoolOut, error) {
	if in.DedicatedPoolID == "" {
		return DeleteDedicatedPoolOut{PoolDeleted: false}, nil
	}

	fw := saga.Get[*addon.ProviderFramework](ctx)
	if err := fw.DeleteSandboxPool(ctx, in.DedicatedPoolID); err != nil {
		return DeleteDedicatedPoolOut{}, fmt.Errorf("deleting sandbox pool: %w", err)
	}

	return DeleteDedicatedPoolOut{PoolDeleted: true}, nil
}

func UndoDeleteDedicatedPool(ctx context.Context, in DeleteDedicatedPoolIn, out DeleteDedicatedPoolOut) error {
	return nil
}
