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

// ShopCardKey holds the schema definition for card-key inventory.
type ShopCardKey struct {
	ent.Schema
}

func (ShopCardKey) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "shop_card_keys",
			Checks: map[string]string{
				"shop_card_keys_sold_requires_order": "NOT (status = 'sold' AND (order_id IS NULL OR sold_at IS NULL))",
			},
		},
	}
}

func (ShopCardKey) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ShopCardKey) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("product_id"),
		field.String("content").
			NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("status").
			MaxLen(20).
			Default("available"),
		field.Int64("order_id").
			Optional().
			Nillable(),
		field.Time("locked_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("locked_until").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("sold_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ShopCardKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("product", ShopProduct.Type).
			Ref("card_keys").
			Field("product_id").
			Unique().
			Required(),
		edge.From("order", ShopOrder.Type).
			Ref("card_keys").
			Field("order_id").
			Unique(),
	}
}

func (ShopCardKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("product_id", "status"),
		index.Fields("order_id"),
		index.Fields("status"),
		index.Fields("locked_until"),
	}
}
