// Package bapigrpc provides the gRPC transport layer for BAPI,
// using cramberry for deterministic binary serialization.
//
// No protobuf code generation is required. Domain types from
// bapi/types are serialized directly via cramberry struct tags.
package bapigrpc

import (
	"fmt"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"google.golang.org/grpc/encoding"
)

const codecName = "cramberry"

// CramberryCodec implements grpc/encoding.Codec using cramberry
// for deterministic binary serialization.
type CramberryCodec struct{}

func (CramberryCodec) Marshal(v any) ([]byte, error) {
	data, err := cramberry.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("cramberry marshal: %w", err)
	}
	return data, nil
}

func (CramberryCodec) Unmarshal(data []byte, v any) error {
	if err := cramberry.Unmarshal(data, v); err != nil {
		return fmt.Errorf("cramberry unmarshal: %w", err)
	}
	return nil
}

func (CramberryCodec) Name() string { return codecName }

func init() {
	encoding.RegisterCodec(CramberryCodec{})
}
