// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
	"github.com/paycrest/protocol/ent/predicate"
	"github.com/paycrest/protocol/ent/senderordertoken"
	"github.com/paycrest/protocol/ent/senderprofile"
	"github.com/paycrest/protocol/ent/token"
	"github.com/shopspring/decimal"
)

// SenderOrderTokenUpdate is the builder for updating SenderOrderToken entities.
type SenderOrderTokenUpdate struct {
	config
	hooks    []Hook
	mutation *SenderOrderTokenMutation
}

// Where appends a list predicates to the SenderOrderTokenUpdate builder.
func (sotu *SenderOrderTokenUpdate) Where(ps ...predicate.SenderOrderToken) *SenderOrderTokenUpdate {
	sotu.mutation.Where(ps...)
	return sotu
}

// SetUpdatedAt sets the "updated_at" field.
func (sotu *SenderOrderTokenUpdate) SetUpdatedAt(t time.Time) *SenderOrderTokenUpdate {
	sotu.mutation.SetUpdatedAt(t)
	return sotu
}

// SetFeePerTokenUnit sets the "fee_per_token_unit" field.
func (sotu *SenderOrderTokenUpdate) SetFeePerTokenUnit(d decimal.Decimal) *SenderOrderTokenUpdate {
	sotu.mutation.ResetFeePerTokenUnit()
	sotu.mutation.SetFeePerTokenUnit(d)
	return sotu
}

// AddFeePerTokenUnit adds d to the "fee_per_token_unit" field.
func (sotu *SenderOrderTokenUpdate) AddFeePerTokenUnit(d decimal.Decimal) *SenderOrderTokenUpdate {
	sotu.mutation.AddFeePerTokenUnit(d)
	return sotu
}

// SetFeeAddress sets the "fee_address" field.
func (sotu *SenderOrderTokenUpdate) SetFeeAddress(s string) *SenderOrderTokenUpdate {
	sotu.mutation.SetFeeAddress(s)
	return sotu
}

// SetRefundAddress sets the "refund_address" field.
func (sotu *SenderOrderTokenUpdate) SetRefundAddress(s string) *SenderOrderTokenUpdate {
	sotu.mutation.SetRefundAddress(s)
	return sotu
}

// SetSenderID sets the "sender" edge to the SenderProfile entity by ID.
func (sotu *SenderOrderTokenUpdate) SetSenderID(id uuid.UUID) *SenderOrderTokenUpdate {
	sotu.mutation.SetSenderID(id)
	return sotu
}

// SetSender sets the "sender" edge to the SenderProfile entity.
func (sotu *SenderOrderTokenUpdate) SetSender(s *SenderProfile) *SenderOrderTokenUpdate {
	return sotu.SetSenderID(s.ID)
}

// SetRegisteredTokenID sets the "registered_token" edge to the Token entity by ID.
func (sotu *SenderOrderTokenUpdate) SetRegisteredTokenID(id int) *SenderOrderTokenUpdate {
	sotu.mutation.SetRegisteredTokenID(id)
	return sotu
}

// SetRegisteredToken sets the "registered_token" edge to the Token entity.
func (sotu *SenderOrderTokenUpdate) SetRegisteredToken(t *Token) *SenderOrderTokenUpdate {
	return sotu.SetRegisteredTokenID(t.ID)
}

// Mutation returns the SenderOrderTokenMutation object of the builder.
func (sotu *SenderOrderTokenUpdate) Mutation() *SenderOrderTokenMutation {
	return sotu.mutation
}

// ClearSender clears the "sender" edge to the SenderProfile entity.
func (sotu *SenderOrderTokenUpdate) ClearSender() *SenderOrderTokenUpdate {
	sotu.mutation.ClearSender()
	return sotu
}

// ClearRegisteredToken clears the "registered_token" edge to the Token entity.
func (sotu *SenderOrderTokenUpdate) ClearRegisteredToken() *SenderOrderTokenUpdate {
	sotu.mutation.ClearRegisteredToken()
	return sotu
}

// Save executes the query and returns the number of nodes affected by the update operation.
func (sotu *SenderOrderTokenUpdate) Save(ctx context.Context) (int, error) {
	sotu.defaults()
	return withHooks(ctx, sotu.sqlSave, sotu.mutation, sotu.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (sotu *SenderOrderTokenUpdate) SaveX(ctx context.Context) int {
	affected, err := sotu.Save(ctx)
	if err != nil {
		panic(err)
	}
	return affected
}

// Exec executes the query.
func (sotu *SenderOrderTokenUpdate) Exec(ctx context.Context) error {
	_, err := sotu.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (sotu *SenderOrderTokenUpdate) ExecX(ctx context.Context) {
	if err := sotu.Exec(ctx); err != nil {
		panic(err)
	}
}

// defaults sets the default values of the builder before save.
func (sotu *SenderOrderTokenUpdate) defaults() {
	if _, ok := sotu.mutation.UpdatedAt(); !ok {
		v := senderordertoken.UpdateDefaultUpdatedAt()
		sotu.mutation.SetUpdatedAt(v)
	}
}

// check runs all checks and user-defined validators on the builder.
func (sotu *SenderOrderTokenUpdate) check() error {
	if v, ok := sotu.mutation.FeeAddress(); ok {
		if err := senderordertoken.FeeAddressValidator(v); err != nil {
			return &ValidationError{Name: "fee_address", err: fmt.Errorf(`ent: validator failed for field "SenderOrderToken.fee_address": %w`, err)}
		}
	}
	if v, ok := sotu.mutation.RefundAddress(); ok {
		if err := senderordertoken.RefundAddressValidator(v); err != nil {
			return &ValidationError{Name: "refund_address", err: fmt.Errorf(`ent: validator failed for field "SenderOrderToken.refund_address": %w`, err)}
		}
	}
	if _, ok := sotu.mutation.SenderID(); sotu.mutation.SenderCleared() && !ok {
		return errors.New(`ent: clearing a required unique edge "SenderOrderToken.sender"`)
	}
	if _, ok := sotu.mutation.RegisteredTokenID(); sotu.mutation.RegisteredTokenCleared() && !ok {
		return errors.New(`ent: clearing a required unique edge "SenderOrderToken.registered_token"`)
	}
	return nil
}

func (sotu *SenderOrderTokenUpdate) sqlSave(ctx context.Context) (n int, err error) {
	if err := sotu.check(); err != nil {
		return n, err
	}
	_spec := sqlgraph.NewUpdateSpec(senderordertoken.Table, senderordertoken.Columns, sqlgraph.NewFieldSpec(senderordertoken.FieldID, field.TypeInt))
	if ps := sotu.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := sotu.mutation.UpdatedAt(); ok {
		_spec.SetField(senderordertoken.FieldUpdatedAt, field.TypeTime, value)
	}
	if value, ok := sotu.mutation.FeePerTokenUnit(); ok {
		_spec.SetField(senderordertoken.FieldFeePerTokenUnit, field.TypeFloat64, value)
	}
	if value, ok := sotu.mutation.AddedFeePerTokenUnit(); ok {
		_spec.AddField(senderordertoken.FieldFeePerTokenUnit, field.TypeFloat64, value)
	}
	if value, ok := sotu.mutation.FeeAddress(); ok {
		_spec.SetField(senderordertoken.FieldFeeAddress, field.TypeString, value)
	}
	if value, ok := sotu.mutation.RefundAddress(); ok {
		_spec.SetField(senderordertoken.FieldRefundAddress, field.TypeString, value)
	}
	if sotu.mutation.SenderCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   senderordertoken.SenderTable,
			Columns: []string{senderordertoken.SenderColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(senderprofile.FieldID, field.TypeUUID),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := sotu.mutation.SenderIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   senderordertoken.SenderTable,
			Columns: []string{senderordertoken.SenderColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(senderprofile.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if sotu.mutation.RegisteredTokenCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   senderordertoken.RegisteredTokenTable,
			Columns: []string{senderordertoken.RegisteredTokenColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(token.FieldID, field.TypeInt),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := sotu.mutation.RegisteredTokenIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   senderordertoken.RegisteredTokenTable,
			Columns: []string{senderordertoken.RegisteredTokenColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(token.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if n, err = sqlgraph.UpdateNodes(ctx, sotu.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{senderordertoken.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return 0, err
	}
	sotu.mutation.done = true
	return n, nil
}

// SenderOrderTokenUpdateOne is the builder for updating a single SenderOrderToken entity.
type SenderOrderTokenUpdateOne struct {
	config
	fields   []string
	hooks    []Hook
	mutation *SenderOrderTokenMutation
}

// SetUpdatedAt sets the "updated_at" field.
func (sotuo *SenderOrderTokenUpdateOne) SetUpdatedAt(t time.Time) *SenderOrderTokenUpdateOne {
	sotuo.mutation.SetUpdatedAt(t)
	return sotuo
}

// SetFeePerTokenUnit sets the "fee_per_token_unit" field.
func (sotuo *SenderOrderTokenUpdateOne) SetFeePerTokenUnit(d decimal.Decimal) *SenderOrderTokenUpdateOne {
	sotuo.mutation.ResetFeePerTokenUnit()
	sotuo.mutation.SetFeePerTokenUnit(d)
	return sotuo
}

// AddFeePerTokenUnit adds d to the "fee_per_token_unit" field.
func (sotuo *SenderOrderTokenUpdateOne) AddFeePerTokenUnit(d decimal.Decimal) *SenderOrderTokenUpdateOne {
	sotuo.mutation.AddFeePerTokenUnit(d)
	return sotuo
}

// SetFeeAddress sets the "fee_address" field.
func (sotuo *SenderOrderTokenUpdateOne) SetFeeAddress(s string) *SenderOrderTokenUpdateOne {
	sotuo.mutation.SetFeeAddress(s)
	return sotuo
}

// SetRefundAddress sets the "refund_address" field.
func (sotuo *SenderOrderTokenUpdateOne) SetRefundAddress(s string) *SenderOrderTokenUpdateOne {
	sotuo.mutation.SetRefundAddress(s)
	return sotuo
}

// SetSenderID sets the "sender" edge to the SenderProfile entity by ID.
func (sotuo *SenderOrderTokenUpdateOne) SetSenderID(id uuid.UUID) *SenderOrderTokenUpdateOne {
	sotuo.mutation.SetSenderID(id)
	return sotuo
}

// SetSender sets the "sender" edge to the SenderProfile entity.
func (sotuo *SenderOrderTokenUpdateOne) SetSender(s *SenderProfile) *SenderOrderTokenUpdateOne {
	return sotuo.SetSenderID(s.ID)
}

// SetRegisteredTokenID sets the "registered_token" edge to the Token entity by ID.
func (sotuo *SenderOrderTokenUpdateOne) SetRegisteredTokenID(id int) *SenderOrderTokenUpdateOne {
	sotuo.mutation.SetRegisteredTokenID(id)
	return sotuo
}

// SetRegisteredToken sets the "registered_token" edge to the Token entity.
func (sotuo *SenderOrderTokenUpdateOne) SetRegisteredToken(t *Token) *SenderOrderTokenUpdateOne {
	return sotuo.SetRegisteredTokenID(t.ID)
}

// Mutation returns the SenderOrderTokenMutation object of the builder.
func (sotuo *SenderOrderTokenUpdateOne) Mutation() *SenderOrderTokenMutation {
	return sotuo.mutation
}

// ClearSender clears the "sender" edge to the SenderProfile entity.
func (sotuo *SenderOrderTokenUpdateOne) ClearSender() *SenderOrderTokenUpdateOne {
	sotuo.mutation.ClearSender()
	return sotuo
}

// ClearRegisteredToken clears the "registered_token" edge to the Token entity.
func (sotuo *SenderOrderTokenUpdateOne) ClearRegisteredToken() *SenderOrderTokenUpdateOne {
	sotuo.mutation.ClearRegisteredToken()
	return sotuo
}

// Where appends a list predicates to the SenderOrderTokenUpdate builder.
func (sotuo *SenderOrderTokenUpdateOne) Where(ps ...predicate.SenderOrderToken) *SenderOrderTokenUpdateOne {
	sotuo.mutation.Where(ps...)
	return sotuo
}

// Select allows selecting one or more fields (columns) of the returned entity.
// The default is selecting all fields defined in the entity schema.
func (sotuo *SenderOrderTokenUpdateOne) Select(field string, fields ...string) *SenderOrderTokenUpdateOne {
	sotuo.fields = append([]string{field}, fields...)
	return sotuo
}

// Save executes the query and returns the updated SenderOrderToken entity.
func (sotuo *SenderOrderTokenUpdateOne) Save(ctx context.Context) (*SenderOrderToken, error) {
	sotuo.defaults()
	return withHooks(ctx, sotuo.sqlSave, sotuo.mutation, sotuo.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (sotuo *SenderOrderTokenUpdateOne) SaveX(ctx context.Context) *SenderOrderToken {
	node, err := sotuo.Save(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// Exec executes the query on the entity.
func (sotuo *SenderOrderTokenUpdateOne) Exec(ctx context.Context) error {
	_, err := sotuo.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (sotuo *SenderOrderTokenUpdateOne) ExecX(ctx context.Context) {
	if err := sotuo.Exec(ctx); err != nil {
		panic(err)
	}
}

// defaults sets the default values of the builder before save.
func (sotuo *SenderOrderTokenUpdateOne) defaults() {
	if _, ok := sotuo.mutation.UpdatedAt(); !ok {
		v := senderordertoken.UpdateDefaultUpdatedAt()
		sotuo.mutation.SetUpdatedAt(v)
	}
}

// check runs all checks and user-defined validators on the builder.
func (sotuo *SenderOrderTokenUpdateOne) check() error {
	if v, ok := sotuo.mutation.FeeAddress(); ok {
		if err := senderordertoken.FeeAddressValidator(v); err != nil {
			return &ValidationError{Name: "fee_address", err: fmt.Errorf(`ent: validator failed for field "SenderOrderToken.fee_address": %w`, err)}
		}
	}
	if v, ok := sotuo.mutation.RefundAddress(); ok {
		if err := senderordertoken.RefundAddressValidator(v); err != nil {
			return &ValidationError{Name: "refund_address", err: fmt.Errorf(`ent: validator failed for field "SenderOrderToken.refund_address": %w`, err)}
		}
	}
	if _, ok := sotuo.mutation.SenderID(); sotuo.mutation.SenderCleared() && !ok {
		return errors.New(`ent: clearing a required unique edge "SenderOrderToken.sender"`)
	}
	if _, ok := sotuo.mutation.RegisteredTokenID(); sotuo.mutation.RegisteredTokenCleared() && !ok {
		return errors.New(`ent: clearing a required unique edge "SenderOrderToken.registered_token"`)
	}
	return nil
}

func (sotuo *SenderOrderTokenUpdateOne) sqlSave(ctx context.Context) (_node *SenderOrderToken, err error) {
	if err := sotuo.check(); err != nil {
		return _node, err
	}
	_spec := sqlgraph.NewUpdateSpec(senderordertoken.Table, senderordertoken.Columns, sqlgraph.NewFieldSpec(senderordertoken.FieldID, field.TypeInt))
	id, ok := sotuo.mutation.ID()
	if !ok {
		return nil, &ValidationError{Name: "id", err: errors.New(`ent: missing "SenderOrderToken.id" for update`)}
	}
	_spec.Node.ID.Value = id
	if fields := sotuo.fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, senderordertoken.FieldID)
		for _, f := range fields {
			if !senderordertoken.ValidColumn(f) {
				return nil, &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
			}
			if f != senderordertoken.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, f)
			}
		}
	}
	if ps := sotuo.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := sotuo.mutation.UpdatedAt(); ok {
		_spec.SetField(senderordertoken.FieldUpdatedAt, field.TypeTime, value)
	}
	if value, ok := sotuo.mutation.FeePerTokenUnit(); ok {
		_spec.SetField(senderordertoken.FieldFeePerTokenUnit, field.TypeFloat64, value)
	}
	if value, ok := sotuo.mutation.AddedFeePerTokenUnit(); ok {
		_spec.AddField(senderordertoken.FieldFeePerTokenUnit, field.TypeFloat64, value)
	}
	if value, ok := sotuo.mutation.FeeAddress(); ok {
		_spec.SetField(senderordertoken.FieldFeeAddress, field.TypeString, value)
	}
	if value, ok := sotuo.mutation.RefundAddress(); ok {
		_spec.SetField(senderordertoken.FieldRefundAddress, field.TypeString, value)
	}
	if sotuo.mutation.SenderCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   senderordertoken.SenderTable,
			Columns: []string{senderordertoken.SenderColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(senderprofile.FieldID, field.TypeUUID),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := sotuo.mutation.SenderIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   senderordertoken.SenderTable,
			Columns: []string{senderordertoken.SenderColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(senderprofile.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	if sotuo.mutation.RegisteredTokenCleared() {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   senderordertoken.RegisteredTokenTable,
			Columns: []string{senderordertoken.RegisteredTokenColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(token.FieldID, field.TypeInt),
			},
		}
		_spec.Edges.Clear = append(_spec.Edges.Clear, edge)
	}
	if nodes := sotuo.mutation.RegisteredTokenIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   senderordertoken.RegisteredTokenTable,
			Columns: []string{senderordertoken.RegisteredTokenColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(token.FieldID, field.TypeInt),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges.Add = append(_spec.Edges.Add, edge)
	}
	_node = &SenderOrderToken{config: sotuo.config}
	_spec.Assign = _node.assignValues
	_spec.ScanValues = _node.scanValues
	if err = sqlgraph.UpdateNode(ctx, sotuo.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{senderordertoken.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	sotuo.mutation.done = true
	return _node, nil
}
