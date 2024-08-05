package test

import (
	"context"

	"github.com/google/uuid"
	"github.com/paycrest/protocol/ent"
	"github.com/paycrest/protocol/types"
	"github.com/stretchr/testify/mock"
)

// Mock order service
type MockOrderService struct {
	mock.Mock
}

// CreateOrder mocks the CreateOrder method
func (m *MockOrderService) CreateOrder(ctx context.Context, client types.RPCClient, orderID uuid.UUID) error {
	return nil
}

// RefundOrder mocks the RefundOrder method
func (m *MockOrderService) RefundOrder(ctx context.Context, client types.RPCClient, orderID string) error {
	return nil
}

// RevertOrder mocks the RevertOrder method
func (m *MockOrderService) RevertOrder(ctx context.Context, client types.RPCClient, order *ent.PaymentOrder) error {
	return nil
}

// SettleOrder mocks the SettleOrder method
func (m *MockOrderService) SettleOrder(ctx context.Context, client types.RPCClient, orderID uuid.UUID) error {
	return nil
}
