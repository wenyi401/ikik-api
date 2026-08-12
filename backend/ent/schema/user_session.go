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

type UserSession struct {
	ent.Schema
}

func (UserSession) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "user_sessions"}}
}

func (UserSession) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (UserSession) Fields() []ent.Field {
	return []ent.Field{
		field.String("sid").MaxLen(36).Unique(),
		field.Int64("user_id"),
		field.Int64("version").Default(1),
		field.Int64("user_auth_version"),
		field.Enum("status").Values("active", "revoking", "revoked").Default("active"),
		field.String("refresh_hash").MaxLen(64),
		field.String("previous_refresh_hash").MaxLen(64).Default(""),
		field.Time("previous_valid_until").Optional().Nillable(),
		field.String("login_method").MaxLen(32).Default("unknown"),
		field.String("ip").MaxLen(64).Default(""),
		field.String("user_agent").MaxLen(512).Default(""),
		field.Time("last_active_at"),
		field.Time("expires_at"),
		field.Time("revoked_at").Optional().Nillable(),
		field.String("revoked_reason").MaxLen(64).Default(""),
	}
}

func (UserSession) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("login_sessions").Field("user_id").Unique().Required(),
	}
}

func (UserSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "status", "last_active_at"),
		index.Fields("user_id", "created_at"),
		index.Fields("expires_at"),
	}
}
