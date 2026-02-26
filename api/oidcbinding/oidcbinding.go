package oidcbinding

//go:generate mkdir -p oidcbinding_v1alpha
//go:generate go run ../../pkg/rpc/cmd/rpcgen -pkg oidcbinding_v1alpha -input rpc.yml -output oidcbinding_v1alpha/rpc.gen.go
