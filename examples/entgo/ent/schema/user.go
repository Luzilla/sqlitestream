package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// User is a minimal Ent schema for the example.
type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
	}
}
