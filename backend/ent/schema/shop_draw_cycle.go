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

// ShopDrawCycle stores a bounded reward pool for one user and one draw product.
type ShopDrawCycle struct {
	ent.Schema
}

func (ShopDrawCycle) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "shop_draw_cycles"},
	}
}

func (ShopDrawCycle) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ShopDrawCycle) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("product_id"),
		field.Int("cycle_no"),
		field.Int("guarantee_count"),
		field.Float("target_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.JSON("remaining_amounts", []float64{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int("drawn_count").
			Default(0),
		field.Float("drawn_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
			Default(0),
		field.Bool("completed").
			Default(false),
	}
}

func (ShopDrawCycle) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("shop_draw_cycles").
			Field("user_id").
			Unique().
			Required(),
		edge.From("product", ShopProduct.Type).
			Ref("draw_cycles").
			Field("product_id").
			Unique().
			Required(),
		edge.To("orders", ShopOrder.Type),
		edge.To("balance_ledger", ShopBalanceLedger.Type),
	}
}

func (ShopDrawCycle) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "product_id", "completed"),
		index.Fields("user_id", "product_id", "cycle_no").Unique(),
	}
}
