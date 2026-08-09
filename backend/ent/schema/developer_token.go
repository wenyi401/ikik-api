package schema

import (
	"ikik-api/ent/schema/mixins"
	"ikik-api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type DeveloperToken struct {
	ent.Schema
}

func (DeveloperToken) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "developer_tokens"}}
}

func (DeveloperToken) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (DeveloperToken) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("name").MaxLen(100).NotEmpty(),
		field.String("token_prefix").MaxLen(20).NotEmpty(),
		field.String("token_hash").MaxLen(64).NotEmpty().Unique(),
		field.JSON("scopes", []string{}).Default(func() []string { return []string{} }),
		field.String("status").MaxLen(20).Default(domain.StatusActive),
		field.Time("expires_at").Optional().Nillable(),
		field.Time("last_used_at").Optional().Nillable(),
		field.String("last_used_ip").MaxLen(64).Optional().Nillable(),
	}
}

func (DeveloperToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("developer_tokens").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (DeveloperToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status", "expires_at"),
		index.Fields("deleted_at"),
	}
}
