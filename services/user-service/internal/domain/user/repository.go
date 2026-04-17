package user

import "context"

// Repository is the port for User persistence.
type Repository interface {
	Save(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	ListByRole(ctx context.Context, role Role) ([]*User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
