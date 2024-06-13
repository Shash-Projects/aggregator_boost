// Code generated by ent, DO NOT EDIT.

package network

import (
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
)

const (
	// Label holds the string label denoting the network type in the database.
	Label = "network"
	// FieldID holds the string denoting the id field in the database.
	FieldID = "id"
	// FieldCreatedAt holds the string denoting the created_at field in the database.
	FieldCreatedAt = "created_at"
	// FieldUpdatedAt holds the string denoting the updated_at field in the database.
	FieldUpdatedAt = "updated_at"
	// FieldChainID holds the string denoting the chain_id field in the database.
	FieldChainID = "chain_id"
	// FieldChainIDHex holds the string denoting the chain_id_hex field in the database.
	FieldChainIDHex = "chain_id_hex"
	// FieldIdentifier holds the string denoting the identifier field in the database.
	FieldIdentifier = "identifier"
	// FieldRPCEndpoint holds the string denoting the rpc_endpoint field in the database.
	FieldRPCEndpoint = "rpc_endpoint"
	// FieldGatewayContractAddress holds the string denoting the gateway_contract_address field in the database.
	FieldGatewayContractAddress = "gateway_contract_address"
	// FieldIsTestnet holds the string denoting the is_testnet field in the database.
	FieldIsTestnet = "is_testnet"
	// FieldFee holds the string denoting the fee field in the database.
	FieldFee = "fee"
	// EdgeTokens holds the string denoting the tokens edge name in mutations.
	EdgeTokens = "tokens"
	// Table holds the table name of the network in the database.
	Table = "networks"
	// TokensTable is the table that holds the tokens relation/edge.
	TokensTable = "tokens"
	// TokensInverseTable is the table name for the Token entity.
	// It exists in this package in order to avoid circular dependency with the "token" package.
	TokensInverseTable = "tokens"
	// TokensColumn is the table column denoting the tokens relation/edge.
	TokensColumn = "network_tokens"
)

// Columns holds all SQL columns for network fields.
var Columns = []string{
	FieldID,
	FieldCreatedAt,
	FieldUpdatedAt,
	FieldChainID,
	FieldChainIDHex,
	FieldIdentifier,
	FieldRPCEndpoint,
	FieldGatewayContractAddress,
	FieldIsTestnet,
	FieldFee,
}

// ValidColumn reports if the column name is valid (part of the table columns).
func ValidColumn(column string) bool {
	for i := range Columns {
		if column == Columns[i] {
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
	// DefaultGatewayContractAddress holds the default value on creation for the "gateway_contract_address" field.
	DefaultGatewayContractAddress string
)

// OrderOption defines the ordering options for the Network queries.
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

// ByChainID orders the results by the chain_id field.
func ByChainID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldChainID, opts...).ToFunc()
}

// ByChainIDHex orders the results by the chain_id_hex field.
func ByChainIDHex(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldChainIDHex, opts...).ToFunc()
}

// ByIdentifier orders the results by the identifier field.
func ByIdentifier(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldIdentifier, opts...).ToFunc()
}

// ByRPCEndpoint orders the results by the rpc_endpoint field.
func ByRPCEndpoint(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldRPCEndpoint, opts...).ToFunc()
}

// ByGatewayContractAddress orders the results by the gateway_contract_address field.
func ByGatewayContractAddress(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldGatewayContractAddress, opts...).ToFunc()
}

// ByIsTestnet orders the results by the is_testnet field.
func ByIsTestnet(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldIsTestnet, opts...).ToFunc()
}

// ByFee orders the results by the fee field.
func ByFee(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldFee, opts...).ToFunc()
}

// ByTokensCount orders the results by tokens count.
func ByTokensCount(opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborsCount(s, newTokensStep(), opts...)
	}
}

// ByTokens orders the results by tokens terms.
func ByTokens(term sql.OrderTerm, terms ...sql.OrderTerm) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newTokensStep(), append([]sql.OrderTerm{term}, terms...)...)
	}
}
func newTokensStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(TokensInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.O2M, false, TokensTable, TokensColumn),
	)
}
