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

// ShopCategory holds the schema definition for store categories.
type ShopCategory struct {
	ent.Schema
}

func (ShopCategory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "shop_categories"},
	}
}

func (ShopCategory) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ShopCategory) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("icon").
			Optional().
			Nillable().
			MaxLen(255),
		field.Int("sort_order").
			Default(0),
		field.Bool("enabled").
			Default(true),
		field.String("description").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (ShopCategory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("products", ShopProduct.Type),
	}
}

func (ShopCategory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
		index.Fields("sort_order"),
	}
}
