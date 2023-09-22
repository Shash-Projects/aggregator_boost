// Code generated by ent, DO NOT EDIT.

package lockorderfulfillment

import (
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
)

const (
	// Label holds the string label denoting the lockorderfulfillment type in the database.
	Label = "lock_order_fulfillment"
	// FieldID holds the string denoting the id field in the database.
	FieldID = "id"
	// FieldCreatedAt holds the string denoting the created_at field in the database.
	FieldCreatedAt = "created_at"
	// FieldUpdatedAt holds the string denoting the updated_at field in the database.
	FieldUpdatedAt = "updated_at"
	// FieldTxID holds the string denoting the tx_id field in the database.
	FieldTxID = "tx_id"
	// FieldTxReceiptImage holds the string denoting the tx_receipt_image field in the database.
	FieldTxReceiptImage = "tx_receipt_image"
	// FieldConfirmations holds the string denoting the confirmations field in the database.
	FieldConfirmations = "confirmations"
	// EdgeOrder holds the string denoting the order edge name in mutations.
	EdgeOrder = "order"
	// Table holds the table name of the lockorderfulfillment in the database.
	Table = "lock_order_fulfillments"
	// OrderTable is the table that holds the order relation/edge.
	OrderTable = "lock_order_fulfillments"
	// OrderInverseTable is the table name for the LockPaymentOrder entity.
	// It exists in this package in order to avoid circular dependency with the "lockpaymentorder" package.
	OrderInverseTable = "lock_payment_orders"
	// OrderColumn is the table column denoting the order relation/edge.
	OrderColumn = "lock_payment_order_fulfillment"
)

// Columns holds all SQL columns for lockorderfulfillment fields.
var Columns = []string{
	FieldID,
	FieldCreatedAt,
	FieldUpdatedAt,
	FieldTxID,
	FieldTxReceiptImage,
	FieldConfirmations,
}

// ForeignKeys holds the SQL foreign-keys that are owned by the "lock_order_fulfillments"
// table and are not defined as standalone fields in the schema.
var ForeignKeys = []string{
	"lock_payment_order_fulfillment",
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
	// DefaultConfirmations holds the default value on creation for the "confirmations" field.
	DefaultConfirmations int
)

// OrderOption defines the ordering options for the LockOrderFulfillment queries.
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

// ByTxID orders the results by the tx_id field.
func ByTxID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldTxID, opts...).ToFunc()
}

// ByTxReceiptImage orders the results by the tx_receipt_image field.
func ByTxReceiptImage(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldTxReceiptImage, opts...).ToFunc()
}

// ByConfirmations orders the results by the confirmations field.
func ByConfirmations(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldConfirmations, opts...).ToFunc()
}

// ByOrderField orders the results by order field.
func ByOrderField(field string, opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newOrderStep(), sql.OrderByField(field, opts...))
	}
}
func newOrderStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(OrderInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.O2O, true, OrderTable, OrderColumn),
	)
}
