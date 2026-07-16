package schema

import (
	"ikik-api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// APIKeyGroupRoute stores the priority routes an API key may use for scheduling.
type APIKeyGroupRoute struct {
	ent.Schema
}

func (APIKeyGroupRoute) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "api_key_group_routes"},
	}
}

func (APIKeyGroupRoute) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (APIKeyGroupRoute) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("api_key_id"),
		field.Int64("group_id"),
		field.Int("priority").
			Default(100),
		field.Int("weight").
			Default(1),
		field.Bool("enabled").
			Default(true),
		field.Int("cooldown_seconds").
			Default(30),
	}
}

func (APIKeyGroupRoute) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("api_key", APIKey.Type).
			Ref("group_routes").
			Field("api_key_id").
			Unique().
			Required(),
		edge.From("group", Group.Type).
			Ref("api_key_group_routes").
			Field("group_id").
			Unique().
			Required(),
	}
}

func (APIKeyGroupRoute) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("api_key_id", "group_id").
			Unique(),
		index.Fields("api_key_id", "enabled", "priority"),
		index.Fields("group_id"),
	}
}
