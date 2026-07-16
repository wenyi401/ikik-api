package schema

import (
	"ikik-api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ShopBalanceLedger records wallet balance movement caused by store orders.
type ShopBalanceLedger struct {
	ent.Schema
}

func (ShopBalanceLedger) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "shop_balance_ledger"},
	}
}

func (ShopBalanceLedger) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ShopBalanceLedger) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("shop_order_id"),
		field.String("entry_type").
			MaxLen(30),
		field.Float("debit_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
			Default(0),
		field.Float("credit_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
			Default(0),
		field.Float("balance_before").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("balance_after").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int64("draw_cycle_id").
			Optional().
			Nillable(),
		field.Int("draw_cycle_index").
			Optional().
			Nillable(),
	}
}

func (ShopBalanceLedger) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("shop_balance_ledger").
			Field("user_id").
			Unique().
			Required(),
		edge.From("shop_order", ShopOrder.Type).
			Ref("balance_ledger").
			Field("shop_order_id").
			Unique().
			Required(),
		edge.From("draw_cycle", ShopDrawCycle.Type).
			Ref("balance_ledger").
			Field("draw_cycle_id").
			Unique(),
	}
}

func (ShopBalanceLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("shop_order_id", "entry_type").Unique(),
		index.Fields("draw_cycle_id"),
	}
}
