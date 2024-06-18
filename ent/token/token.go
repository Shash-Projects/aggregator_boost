// Code generated by ent, DO NOT EDIT.

package token

import (
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
)

const (
	// Label holds the string label denoting the token type in the database.
	Label = "token"
	// FieldID holds the string denoting the id field in the database.
	FieldID = "id"
	// FieldCreatedAt holds the string denoting the created_at field in the database.
	FieldCreatedAt = "created_at"
	// FieldUpdatedAt holds the string denoting the updated_at field in the database.
	FieldUpdatedAt = "updated_at"
	// FieldSymbol holds the string denoting the symbol field in the database.
	FieldSymbol = "symbol"
	// FieldContractAddress holds the string denoting the contract_address field in the database.
	FieldContractAddress = "contract_address"
	// FieldDecimals holds the string denoting the decimals field in the database.
	FieldDecimals = "decimals"
	// FieldIsEnabled holds the string denoting the is_enabled field in the database.
	FieldIsEnabled = "is_enabled"
	// EdgeNetwork holds the string denoting the network edge name in mutations.
	EdgeNetwork = "network"
	// EdgePaymentOrders holds the string denoting the payment_orders edge name in mutations.
	EdgePaymentOrders = "payment_orders"
	// EdgeLockPaymentOrders holds the string denoting the lock_payment_orders edge name in mutations.
	EdgeLockPaymentOrders = "lock_payment_orders"
	// EdgeSenderOrders holds the string denoting the sender_orders edge name in mutations.
	EdgeSenderOrders = "sender_orders"
	// Table holds the table name of the token in the database.
	Table = "tokens"
	// NetworkTable is the table that holds the network relation/edge.
	NetworkTable = "tokens"
	// NetworkInverseTable is the table name for the Network entity.
	// It exists in this package in order to avoid circular dependency with the "network" package.
	NetworkInverseTable = "networks"
	// NetworkColumn is the table column denoting the network relation/edge.
	NetworkColumn = "network_tokens"
	// PaymentOrdersTable is the table that holds the payment_orders relation/edge.
	PaymentOrdersTable = "payment_orders"
	// PaymentOrdersInverseTable is the table name for the PaymentOrder entity.
	// It exists in this package in order to avoid circular dependency with the "paymentorder" package.
	PaymentOrdersInverseTable = "payment_orders"
	// PaymentOrdersColumn is the table column denoting the payment_orders relation/edge.
	PaymentOrdersColumn = "token_payment_orders"
	// LockPaymentOrdersTable is the table that holds the lock_payment_orders relation/edge.
	LockPaymentOrdersTable = "lock_payment_orders"
	// LockPaymentOrdersInverseTable is the table name for the LockPaymentOrder entity.
	// It exists in this package in order to avoid circular dependency with the "lockpaymentorder" package.
	LockPaymentOrdersInverseTable = "lock_payment_orders"
	// LockPaymentOrdersColumn is the table column denoting the lock_payment_orders relation/edge.
	LockPaymentOrdersColumn = "token_lock_payment_orders"
	// SenderOrdersTable is the table that holds the sender_orders relation/edge.
	SenderOrdersTable = "sender_order_tokens"
	// SenderOrdersInverseTable is the table name for the SenderOrderToken entity.
	// It exists in this package in order to avoid circular dependency with the "senderordertoken" package.
	SenderOrdersInverseTable = "sender_order_tokens"
	// SenderOrdersColumn is the table column denoting the sender_orders relation/edge.
	SenderOrdersColumn = "token_sender_orders"
)

// Columns holds all SQL columns for token fields.
var Columns = []string{
	FieldID,
	FieldCreatedAt,
	FieldUpdatedAt,
	FieldSymbol,
	FieldContractAddress,
	FieldDecimals,
	FieldIsEnabled,
}

// ForeignKeys holds the SQL foreign-keys that are owned by the "tokens"
// table and are not defined as standalone fields in the schema.
var ForeignKeys = []string{
	"network_tokens",
}

// ValidColumn reports if the column name is valid (part of the table columns).
func ValidColumn(column string) bool {
	for i := range Columns {
		if column == Columns[i] {
			return true
		}
	}
	for i := range ForeignKeys {
		if column == ForeignKeys[i] {
			return true
		}
	}
	return false
}

var (
	// DefaultCreatedAt holds the default value on creation for the "created_at" field.
	DefaultCreatedAt func() time.Time
	// DefaultUpdatedAt holds the default value on creation for the "updated_at" field.
	DefaultUpdatedAt func() time.Time
	// UpdateDefaultUpdatedAt holds the default value on update for the "updated_at" field.
	UpdateDefaultUpdatedAt func() time.Time
	// SymbolValidator is a validator for the "symbol" field. It is called by the builders before save.
	SymbolValidator func(string) error
	// ContractAddressValidator is a validator for the "contract_address" field. It is called by the builders before save.
	ContractAddressValidator func(string) error
	// DefaultIsEnabled holds the default value on creation for the "is_enabled" field.
	DefaultIsEnabled bool
)

// OrderOption defines the ordering options for the Token queries.
type OrderOption func(*sql.Selector)

// ByID orders the results by the id field.
func ByID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldID, opts...).ToFunc()
}

// ByCreatedAt orders the results by the created_at field.
func ByCreatedAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldCreatedAt, opts...).ToFunc()
}

// ByUpdatedAt orders the results by the updated_at field.
func ByUpdatedAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldUpdatedAt, opts...).ToFunc()
}

// BySymbol orders the results by the symbol field.
func BySymbol(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldSymbol, opts...).ToFunc()
}

// ByContractAddress orders the results by the contract_address field.
func ByContractAddress(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldContractAddress, opts...).ToFunc()
}

// ByDecimals orders the results by the decimals field.
func ByDecimals(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldDecimals, opts...).ToFunc()
}

// ByIsEnabled orders the results by the is_enabled field.
func ByIsEnabled(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldIsEnabled, opts...).ToFunc()
}

// ByNetworkField orders the results by network field.
func ByNetworkField(field string, opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newNetworkStep(), sql.OrderByField(field, opts...))
	}
}

// ByPaymentOrdersCount orders the results by payment_orders count.
func ByPaymentOrdersCount(opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborsCount(s, newPaymentOrdersStep(), opts...)
	}
}

// ByPaymentOrders orders the results by payment_orders terms.
func ByPaymentOrders(term sql.OrderTerm, terms ...sql.OrderTerm) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newPaymentOrdersStep(), append([]sql.OrderTerm{term}, terms...)...)
	}
}

// ByLockPaymentOrdersCount orders the results by lock_payment_orders count.
func ByLockPaymentOrdersCount(opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborsCount(s, newLockPaymentOrdersStep(), opts...)
	}
}

// ByLockPaymentOrders orders the results by lock_payment_orders terms.
func ByLockPaymentOrders(term sql.OrderTerm, terms ...sql.OrderTerm) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newLockPaymentOrdersStep(), append([]sql.OrderTerm{term}, terms...)...)
	}
}

// BySenderOrdersCount orders the results by sender_orders count.
func BySenderOrdersCount(opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborsCount(s, newSenderOrdersStep(), opts...)
	}
}

// BySenderOrders orders the results by sender_orders terms.
func BySenderOrders(term sql.OrderTerm, terms ...sql.OrderTerm) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newSenderOrdersStep(), append([]sql.OrderTerm{term}, terms...)...)
	}
}
func newNetworkStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(NetworkInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.M2O, true, NetworkTable, NetworkColumn),
	)
}
func newPaymentOrdersStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(PaymentOrdersInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.O2M, false, PaymentOrdersTable, PaymentOrdersColumn),
	)
}
func newLockPaymentOrdersStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(LockPaymentOrdersInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.O2M, false, LockPaymentOrdersTable, LockPaymentOrdersColumn),
	)
}
func newSenderOrdersStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(SenderOrdersInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.O2M, false, SenderOrdersTable, SenderOrdersColumn),
	)
}
